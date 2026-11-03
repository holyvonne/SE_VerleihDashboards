package domain

import (
	"errors"
	"testing"
	"time"
)

// Feste Zeitzone für deterministische Kalendertage in Tests.
var testLoc = time.FixedZone("Testzeit", 3600)

var t0 = time.Date(2026, 11, 10, 10, 0, 0, 0, testLoc)

var testPricing = DailyRate{PerDay: Euro(20), Location: testLoc}

func period(start time.Time, d time.Duration) Period { return Period{Start: start, End: start.Add(d)} }

// 3.3: Buchung übernimmt Zeitraum und Preis.
func TestNewRental(t *testing.T) {
	p := period(t0, 50*time.Hour)
	r := NewRental(1, 2, p, t0.Add(-time.Hour), Euro(60))
	if r.SkiID != 1 || r.PersonID != 2 || r.Planned() != p || r.Price != Euro(60) || r.IsReturned() {
		t.Errorf("Buchung falsch: %+v", r)
	}
}

// 3.5: Zustände werden aus der Uhrzeit abgeleitet.
func TestRentalState(t *testing.T) {
	r := NewRental(1, 2, period(t0, 24*time.Hour), t0.Add(-48*time.Hour), Euro(40))
	cases := []struct {
		name    string
		now     time.Time
		want    RentalState
		overdue bool
	}{
		{"vor Beginn", t0.Add(-time.Minute), RentalReserved, false},
		{"genau Beginn", t0, RentalActive, false},
		{"mittendrin", t0.Add(12 * time.Hour), RentalActive, false},
		{"genau geplante Rückgabe", t0.Add(24 * time.Hour), RentalActive, true},
		{"später", t0.Add(30 * time.Hour), RentalActive, true},
	}
	for _, c := range cases {
		if got := r.State(c.now); got != c.want {
			t.Errorf("%s: State = %q, want %q", c.name, got, c.want)
		}
		if got := r.IsOverdue(c.now); got != c.overdue {
			t.Errorf("%s: IsOverdue = %v, want %v", c.name, got, c.overdue)
		}
	}
	_ = r.Return(t0.Add(30*time.Hour), testPricing)
	if r.State(t0.Add(31*time.Hour)) != RentalClosed || r.IsOverdue(t0.Add(31*time.Hour)) {
		t.Error("nach Rückgabe: abgeschlossen und nicht überfällig erwartet")
	}
}

// F-4.4, AK-10, AK-17: Rückgabe, Endpreis, keine zweite Rückgabe.
func TestRentalReturn(t *testing.T) {
	booked := period(t0, 50*time.Hour) // 10.11. 10:00 – 12.11. 12:00 → 3 Kalendertage
	tests := []struct {
		name      string
		at        time.Time
		wantFinal Money
	}{
		{"vorzeitig: keine Erstattung", t0.Add(2 * time.Hour), Euro(60)},
		{"pünktlich", booked.End, Euro(60)},
		{"einen Kalendertag zu spät", booked.End.Add(24 * time.Hour), Euro(80)},
		{"wenige Stunden zu spät, gleicher Tag", booked.End.Add(3 * time.Hour), Euro(60)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRental(1, 2, booked, t0, Euro(60))
			if err := r.Return(tt.at, testPricing); err != nil {
				t.Fatal(err)
			}
			if !r.ReturnedAt.Equal(tt.at) || r.FinalPrice != tt.wantFinal || r.ChargedPrice() != tt.wantFinal {
				t.Errorf("ReturnedAt = %v, FinalPrice = %v, want %v", r.ReturnedAt, r.FinalPrice, tt.wantFinal)
			}
			if err := r.Return(tt.at.Add(time.Hour), testPricing); !errors.Is(err, ErrAlreadyReturned) {
				t.Errorf("zweite Rückgabe: err = %v, want ErrAlreadyReturned", err)
			}
			if !r.ReturnedAt.Equal(tt.at) {
				t.Error("zweite Rückgabe hat den Zeitpunkt verändert")
			}
		})
	}
}

// AK-18: Reservierung kann nicht zurückgegeben, nur storniert werden.
func TestRentalReturnAndCancel_Reservation(t *testing.T) {
	r := NewRental(1, 2, period(t0, 24*time.Hour), t0.Add(-48*time.Hour), Euro(40))
	if err := r.Return(t0.Add(-time.Hour), testPricing); !errors.Is(err, ErrNotStarted) {
		t.Errorf("Return vor Beginn: err = %v, want ErrNotStarted", err)
	}
	if r.IsReturned() {
		t.Error("Reservierung wurde trotzdem zurückgegeben")
	}
	cancellable := func(now time.Time) error { return Evaluate(r, []Rental{r}, now).CheckCancellable() }
	if err := cancellable(t0.Add(-time.Minute)); err != nil {
		t.Errorf("Stornieren vor Beginn: %v", err)
	}
	if err := cancellable(t0); !errors.Is(err, ErrNotCancellable) {
		t.Errorf("Stornieren ab Beginn: err = %v, want ErrNotCancellable", err)
	}
	_ = r.Return(t0.Add(time.Hour), testPricing)
	if err := cancellable(t0.Add(-time.Hour)); !errors.Is(err, ErrNotCancellable) {
		t.Errorf("Stornieren nach Rückgabe: err = %v", err)
	}
}

// 3.6a, AK-22: Überfällige Ausleihe und Folgebuchungen desselben Ski.
func TestEvaluateConflictAndWaiting(t *testing.T) {
	a := NewRental(1, 1, period(t0, 2*time.Hour), t0, Euro(20))
	a.ID = 1
	b := NewRental(1, 2, period(t0.Add(3*time.Hour), 2*time.Hour), t0, Euro(20))
	b.ID = 2
	c := NewRental(1, 3, period(t0.Add(6*time.Hour), time.Hour), t0, Euro(20))
	c.ID = 3
	other := NewRental(2, 3, period(t0.Add(3*time.Hour), time.Hour), t0, Euro(20)) // anderer Ski
	other.ID = 4
	all := []Rental{a, b, c, other}

	if sit := Evaluate(b, all, t0.Add(time.Hour)); sit.State != RentalReserved || sit.Conflict {
		t.Errorf("A läuft planmäßig: B = %+v", sit)
	}
	if sit := Evaluate(b, all, t0.Add(150*time.Minute)); sit.State != RentalReserved || !sit.Conflict {
		t.Errorf("A überfällig: B = %+v, want reserviert mit Konflikt", sit)
	}
	now := t0.Add(6*time.Hour + 30*time.Minute) // A immer noch nicht zurück
	if sit := Evaluate(a, all, now); sit.State != RentalActive || !sit.Overdue {
		t.Errorf("A = %+v", sit)
	}
	for _, x := range []Rental{b, c} {
		if sit := Evaluate(x, all, now); sit.State != RentalWaiting || !sit.Conflict {
			t.Errorf("Buchung %d = %+v, want wartend", x.ID, sit)
		}
	}
	if sit := Evaluate(other, all, now); sit.State != RentalActive || sit.Conflict {
		t.Errorf("Buchung eines anderen Ski darf nicht betroffen sein: %+v", sit)
	}
	if err := Evaluate(b, all, now).CheckCancellable(); err != nil {
		t.Errorf("wartende Buchung muss stornierbar sein: %v", err)
	}

	_ = a.Return(now, testPricing)
	all[0] = a
	if sit := Evaluate(b, all, now); sit.State != RentalActive || sit.Conflict {
		t.Errorf("nach Rückgabe von A: B = %+v, want aktiv", sit)
	}
}

// 3.6: Belegungszeitraum je Zustand.
func TestRentalBlocks(t *testing.T) {
	booked := period(t0, 48*time.Hour)

	r := NewRental(1, 2, booked, t0, Euro(60))
	if got := r.Blocks(t0.Add(-time.Hour)); got != booked {
		t.Errorf("reserviert: %v, want %v", got, booked)
	}
	if got := r.Blocks(t0.Add(time.Hour)); got != booked {
		t.Errorf("aktiv: %v, want %v", got, booked)
	}
	if got := r.Blocks(booked.End.Add(time.Hour)); !got.IsOpen() || !got.Start.Equal(t0) {
		t.Errorf("überfällig: %v, want offen ab Beginn", got)
	}

	early := t0.Add(5 * time.Hour)
	_ = r.Return(early, testPricing)
	if got := r.Blocks(t0.Add(10 * time.Hour)); !got.End.Equal(early) {
		t.Errorf("vorzeitig zurück: Ende %v, want %v", got.End, early)
	}
}

// 3.6, AK-16, AK-17: Konfliktsuche über Belegungen.
func TestFindConflict(t *testing.T) {
	existing := NewRental(1, 2, period(t0, 48*time.Hour), t0, Euro(60))
	rentals := []Rental{existing}
	now := t0.Add(-24 * time.Hour)

	if _, found := FindConflict(period(t0.Add(24*time.Hour), 48*time.Hour), rentals, now); !found {
		t.Error("teilweise Überschneidung nicht gefunden")
	}
	if _, found := FindConflict(period(t0.Add(48*time.Hour), 48*time.Hour), rentals, now); found {
		t.Error("angrenzender Zeitraum fälschlich als Konflikt")
	}

	// Überfällig: blockiert auch spätere Zeiträume, bis der Ski zurück ist.
	late := t0.Add(72 * time.Hour)
	if _, found := FindConflict(period(t0.Add(96*time.Hour), time.Hour), rentals, late); !found {
		t.Error("überfällige Ausleihe blockiert nicht")
	}
	// Vorzeitige Rückgabe gibt die Restzeit frei.
	_ = rentals[0].Return(t0.Add(2*time.Hour), testPricing)
	if _, found := FindConflict(period(t0.Add(3*time.Hour), time.Hour), rentals, t0.Add(2*time.Hour)); found {
		t.Error("Restzeit nach vorzeitiger Rückgabe nicht frei")
	}
}

// F-5.1: geplante Dauer, nach Rückgabe die tatsächliche.
func TestRentalDuration(t *testing.T) {
	r := NewRental(1, 2, period(t0, 51*time.Hour), t0, Euro(60))
	if got := r.Duration(); got != 51*time.Hour {
		t.Errorf("geplant: %v", got)
	}
	_ = r.Return(t0.Add(5*time.Hour), testPricing)
	if got := r.Duration(); got != 5*time.Hour {
		t.Errorf("tatsächlich: %v", got)
	}
}

func TestRentalStateLabel(t *testing.T) {
	if RentalClosed.Label() != "Zurückgegeben" || RentalReserved.Label() != "Reserviert" {
		t.Error("unerwartetes Label")
	}
}
