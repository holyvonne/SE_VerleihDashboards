package domain

import (
	"fmt"
	"testing"
	"time"
)

// hourly ist eine einfache Preisfunktion für die Tests: 1 € pro angefangene Stunde.
func hourly(p Period) Price {
	h := int(p.Duration() / time.Hour)
	if p.Duration()%time.Hour != 0 || h == 0 {
		h++
	}
	return Price{Amount: Euro(int64(h)), Explanation: fmt.Sprintf("%d h", h)}
}

func booking(id int, start, end time.Time) Booking {
	b := NewBooking(1, 1, Period{start, end}, start, hourly(Period{start, end}))
	b.ID = id
	return b
}

// Spezifikation 4.4: Zustände aus der Uhrzeit.
func TestEvaluateStates(t *testing.T) {
	b := booking(1, at(10, 0), at(12, 0))
	cases := []struct {
		now     time.Time
		state   BookingState
		overdue bool
	}{
		{at(9, 59), StateReserved, false},
		{at(10, 0), StateActive, false},
		{at(11, 59), StateActive, false},
		{at(12, 0), StateActive, true},
	}
	for _, c := range cases {
		sit := Evaluate(b, []Booking{b}, c.now)
		if sit.State != c.state || sit.Overdue != c.overdue || sit.Conflict {
			t.Errorf("%s: %+v, want %s overdue=%v", c.now.Format("15:04"), sit, c.state, c.overdue)
		}
	}
	returned := b
	if err := returned.Return(at(11, 0), hourly); err != nil {
		t.Fatal(err)
	}
	if sit := Evaluate(returned, nil, at(11, 30)); sit.State != StateClosed {
		t.Errorf("zurückgegeben: %+v", sit)
	}
}

// AK-24 auf Domain-Ebene: verspätete Rückgabe trifft die nachfolgende Reservierung.
func TestEvaluateConflictAndWaiting(t *testing.T) {
	a := booking(1, at(10, 0), at(12, 0))
	b := booking(2, at(13, 0), at(15, 0))
	all := []Booking{a, b}

	if sit := Evaluate(b, all, at(11, 0)); sit.State != StateReserved || sit.Conflict {
		t.Errorf("11:00: A läuft planmäßig, B ohne Konflikt erwartet: %+v", sit)
	}
	if sit := Evaluate(b, all, at(12, 30)); sit.State != StateReserved || !sit.Conflict {
		t.Errorf("12:30: A überfällig, B reserviert mit Konflikt erwartet: %+v", sit)
	}
	if sit := Evaluate(b, all, at(13, 30)); sit.State != StateWaiting || !sit.Conflict {
		t.Errorf("13:30: B wartend erwartet: %+v", sit)
	}
	if sit := Evaluate(a, all, at(13, 30)); sit.State != StateActive || !sit.Overdue {
		t.Errorf("13:30: A aktiv und überfällig erwartet: %+v", sit)
	}

	if err := a.Return(at(13, 40), hourly); err != nil {
		t.Fatal(err)
	}
	all = []Booking{a, b}
	if sit := Evaluate(b, all, at(13, 45)); sit.State != StateActive || sit.Conflict {
		t.Errorf("nach Rückgabe von A: B aktiv ohne Konflikt erwartet: %+v", sit)
	}
}

// 4.5a: Eine wartende Buchung bleibt wartend, auch wenn ihr geplantes Ende vorbei ist,
// und eine weitere Folgebuchung ist ebenfalls im Konflikt (Kette überfälliger Ausleihen).
func TestWaitingChain(t *testing.T) {
	a := booking(1, at(10, 0), at(11, 0))
	b := booking(2, at(12, 0), at(13, 0))
	c := booking(3, at(14, 0), at(15, 0))
	all := []Booking{a, b, c}
	now := at(14, 30)
	if sit := Evaluate(a, all, now); sit.State != StateActive || !sit.Overdue {
		t.Errorf("A: %+v", sit)
	}
	for _, x := range []Booking{b, c} {
		if sit := Evaluate(x, all, now); sit.State != StateWaiting || !sit.Conflict {
			t.Errorf("Buchung %d: %+v", x.ID, sit)
		}
	}
	active := 0
	for _, x := range all {
		if Evaluate(x, all, now).State == StateActive {
			active++
		}
	}
	if active != 1 {
		t.Errorf("I-2a: %d aktive Buchungen, want 1", active)
	}
}

// Spezifikation 4.5: Belegung.
func TestBlocks(t *testing.T) {
	b := booking(1, at(10, 0), at(12, 0))
	if got := b.Blocks(at(11, 0)); got != b.Planned() {
		t.Errorf("planmäßig: %v", got)
	}
	if got := b.Blocks(at(12, 0)); !got.IsOpen() || !got.Start.Equal(at(10, 0)) {
		t.Errorf("überfällig muss offen sein: %v", got)
	}
	_ = b.Return(at(10, 30), hourly)
	if got := b.Blocks(at(11, 0)); !got.End.Equal(at(10, 30)) {
		t.Errorf("vorzeitige Rückgabe gibt Restzeit frei: %v", got)
	}
}

// AK-13 (Domain): Endpreis für [Beginn, max(geplant, tatsächlich)).
func TestReturnFinalPrice(t *testing.T) {
	early := booking(1, at(9, 0), at(12, 0))
	if err := early.Return(at(10, 15), hourly); err != nil {
		t.Fatal(err)
	}
	if early.FinalPrice.Amount != Euro(3) || !early.ReturnedAt.Equal(at(10, 15)) {
		t.Errorf("vorzeitig: %v %v", early.FinalPrice, early.ReturnedAt)
	}
	if early.Duration() != 75*time.Minute {
		t.Errorf("tatsächliche Dauer: %v", early.Duration())
	}

	late := booking(2, at(9, 0), at(12, 0))
	_ = late.Return(at(12, 20), hourly)
	if late.FinalPrice.Amount != Euro(4) {
		t.Errorf("verspätet: %v", late.FinalPrice)
	}
	if err := late.Return(at(13, 0), hourly); err != ErrAlreadyReturned {
		t.Errorf("doppelt: %v", err)
	}

	future := booking(3, at(14, 0), at(15, 0))
	if err := future.Return(at(13, 0), hourly); err != ErrNotStarted {
		t.Errorf("vor Beginn: %v", err)
	}
}

func TestFindConflict(t *testing.T) {
	bs := []Booking{booking(1, at(10, 0), at(12, 0)), booking(2, at(14, 0), at(16, 0))}
	if c, ok := FindConflict(Period{at(15, 0), at(17, 0)}, bs, at(9, 0), 0); !ok || c.ID != 2 {
		t.Errorf("Konflikt mit 2 erwartet: %v %v", c.ID, ok)
	}
	if _, ok := FindConflict(Period{at(15, 0), at(17, 0)}, bs, at(9, 0), 2); ok {
		t.Errorf("eigene Buchung muss übersprungen werden")
	}
	if _, ok := FindConflict(Period{at(12, 0), at(14, 0)}, bs, at(9, 0), 0); ok {
		t.Errorf("Lücke zwischen den Buchungen muss frei sein")
	}
}

func TestItemStatusAt(t *testing.T) {
	it := Item{ID: 1, Status: StatusAvailable}
	bs := []Booking{booking(1, at(10, 0), at(12, 0))}
	if got := it.StatusAt(bs, at(9, 0)); got != StatusAvailable {
		t.Errorf("vor Beginn: %s", got)
	}
	if got := it.StatusAt(bs, at(10, 0)); got != StatusRented {
		t.Errorf("ab Beginn: %s", got)
	}
	it.Status = StatusMaintenance
	if got := it.StatusAt(bs, at(10, 0)); got != StatusMaintenance {
		t.Errorf("Wartung hat Vorrang: %s", got)
	}
}

func TestCopiesAreIndependent(t *testing.T) {
	it := Item{Attributes: map[string]string{"a": "1"}}
	c := it.Copy()
	c.Attributes["a"] = "2"
	if it.Attributes["a"] != "1" {
		t.Error("Item.Copy teilt die Merkmals-Map")
	}
	b := booking(1, at(10, 0), at(12, 0))
	_ = b.Return(at(11, 0), hourly)
	bc := b.Copy()
	*bc.ReturnedAt = at(23, 0)
	if !b.ReturnedAt.Equal(at(11, 0)) {
		t.Error("Booking.Copy teilt ReturnedAt")
	}
}
