package service_test

import (
	"sync"
	"testing"
	"time"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/service"
	"fahrradverleih/plugins/fahrrad"
)

// AK-8: Die Vorschau speichert nichts und liefert Preis und Erklärung.
func TestQuoteDoesNotStore(t *testing.T) {
	s, _ := newService(t)
	bike, person := addBike(t, s, fahrrad.Mountainbike), addPerson(t, s)
	q, err := s.Quote(in(bike, person, at(9, 0), at(11, 30)))
	if err != nil {
		t.Fatal(err)
	}
	if q.Price.Amount != domain.Euro(24) || q.Price.Explanation != "3 angefangene Stunden × 8,00 €" || !q.IsReservation {
		t.Errorf("Vorschau: %+v", q)
	}
	if q.Duration() != 150*time.Minute || q.Item.Name != "Rad Test" {
		t.Errorf("Dauer/Name: %v %q", q.Duration(), q.Item.Name)
	}
	if n := len(s.Bookings(service.FilterAll)); n != 0 {
		t.Errorf("Vorschau hat %d Buchungen angelegt", n)
	}
}

// AK-10, AK-15: Ausleihe ab sofort und Reservierung, die zur Startzeit aktiv wird.
func TestBookNowAndReservation(t *testing.T) {
	s, c := newService(t)
	a, b, person := addBike(t, s, fahrrad.Citybike), addBike(t, s, fahrrad.Citybike), addPerson(t, s)
	c.set(at(8, 0).Add(37 * time.Second))

	now := mustBook(t, s, in(a, person, at(8, 0), at(10, 0))) // „Sofort“ = aktuelle Minute
	if v, _ := s.Item(a); v.Status != domain.StatusRented {
		t.Errorf("sofort gebucht: Status %s", v.Status)
	}
	if bv, _ := s.Booking(now.ID); bv.State != domain.StateActive {
		t.Errorf("sofort gebucht: Zustand %s", bv.State)
	}

	res := mustBook(t, s, in(b, person, at(12, 0), at(14, 0)))
	v, _ := s.Item(b)
	if v.Status != domain.StatusAvailable || v.NextReservation == nil || v.NextReservation.ID != res.ID {
		t.Errorf("Reservierung: Status %s, nächste %v", v.Status, v.NextReservation)
	}
	st, _ := s.Stats(fahrrad.Key)
	if st.Rented != 1 || st.Reserved != 1 || st.Available != 1 {
		t.Errorf("Dashboard: %+v", st)
	}

	c.set(at(12, 0))
	if v, _ := s.Item(b); v.Status != domain.StatusRented {
		t.Errorf("zur Startzeit muss das Rad ausgeliehen sein: %s", v.Status)
	}
	checkInvariants(t, s)
}

// AK-11: Überschneidungen mit [10:00, 12:00).
func TestBookRejectsOverlaps(t *testing.T) {
	s, _ := newService(t)
	bike, other, person := addBike(t, s, fahrrad.Citybike), addBike(t, s, fahrrad.Citybike), addPerson(t, s)
	mustBook(t, s, in(bike, person, at(10, 0), at(12, 0)))

	for name, p := range map[string][2]time.Time{
		"teilweise":    {at(11, 0), at(13, 0)},
		"umschließend": {at(9, 0), at(14, 0)},
		"enthalten":    {at(10, 30), at(11, 30)},
		"identisch":    {at(10, 0), at(12, 0)},
	} {
		_, err := s.Book(in(bike, person, p[0], p[1]))
		if err != domain.ErrItemBooked {
			t.Errorf("%s: %v, want ErrItemBooked", name, err)
		}
	}
	mustBook(t, s, in(bike, person, at(12, 0), at(13, 0)))
	mustBook(t, s, in(bike, person, at(9, 0), at(10, 0)))
	mustBook(t, s, in(other, person, at(10, 0), at(12, 0)))
	checkInvariants(t, s)
}

// AK-15: Eingaberegeln mit den Grenzen des Plugins.
func TestBookValidation(t *testing.T) {
	s, _ := newService(t)
	bike, person := addBike(t, s, fahrrad.Citybike), addPerson(t, s)
	cases := []struct {
		name       string
		start, end time.Time
		field, msg string
	}{
		{"Vergangenheit", at(7, 59), at(10, 0), "start", domain.MsgStartPast},
		{"Ende vor Beginn", at(10, 0), at(9, 0), "end", domain.MsgEndBefore},
		{"fehlt", time.Time{}, at(9, 0), "start", domain.MsgMissingTime},
		{"59 Minuten", at(10, 0), at(10, 59), "end", "Verleihdauer muss mindestens 1 Stunde betragen"},
		{"über 30 Tage", at(10, 0), at(10, 1).AddDate(0, 0, 30), "end", "Verleihdauer darf höchstens 30 Tage betragen"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := s.Book(in(bike, person, c.start, c.end))
			assertField(t, err, c.field, c.msg)
		})
	}
	mustBook(t, s, in(bike, person, at(10, 0), at(11, 0)))                   // genau 1 Stunde
	mustBook(t, s, in(bike, person, at(11, 0), at(11, 0).AddDate(0, 0, 30))) // genau 30 Tage

	_, err := s.Book(in(999, 999, at(10, 0), at(11, 0)))
	assertField(t, err, "item_id", "Bitte eine gültige Auswahl treffen")
	assertField(t, err, "person_id", "Bitte eine Person wählen")
}

// AK-16
func TestBookMaintenance(t *testing.T) {
	s, _ := newService(t)
	bike, person := addBike(t, s, fahrrad.Citybike), addPerson(t, s)
	if _, err := s.UpdateItem(bike, service.ItemInput{Category: fahrrad.Citybike, Attributes: bikeAttrs("Rad"), Status: domain.StatusMaintenance}); err != nil {
		t.Fatal(err)
	}
	_, err := s.Book(in(bike, person, at(10, 0), at(11, 0)))
	assertErr(t, err, domain.ErrItemInMaintenance)
	if offers, _ := s.Available(fahrrad.Key, "", domain.Period{Start: at(10, 0), End: at(11, 0)}); len(offers) != 0 {
		t.Errorf("Rad in Wartung darf nicht verfügbar sein")
	}
}

// AK-12: viele parallele Buchungen für überlappende Zeiträume – genau eine gewinnt.
func TestBookConcurrent(t *testing.T) {
	s, _ := newService(t)
	bike := addBike(t, s, fahrrad.EBike)
	persons := make([]int, 30)
	for i := range persons {
		persons[i] = addPerson(t, s)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	start := make(chan struct{})
	for i, p := range persons {
		wg.Add(1)
		go func(i, p int) {
			defer wg.Done()
			<-start
			offset := time.Duration(i%3) * 15 * time.Minute
			if _, err := s.Book(in(bike, p, at(10, 0).Add(offset), at(12, 0).Add(offset))); err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}(i, p)
	}
	close(start)
	wg.Wait()
	if ok != 1 {
		t.Fatalf("%d erfolgreiche Buchungen, want 1", ok)
	}
	checkInvariants(t, s)
}

// AK-13: Rückgabe, Endpreis, doppelte Rückgabe, überfällige Ausleihe blockiert.
func TestReturn(t *testing.T) {
	s, c := newService(t)
	bike, person := addBike(t, s, fahrrad.EBike), addPerson(t, s)

	early := mustBook(t, s, in(bike, person, at(9, 0), at(12, 0)))
	if early.Price.Amount != domain.Euro(36) {
		t.Fatalf("Preis: %v", early.Price)
	}
	_, err := s.Return(early.ID)
	assertErr(t, err, domain.ErrNotStarted)

	c.set(at(10, 15))
	got, err := s.Return(early.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.ReturnedAt.Equal(at(10, 15)) || got.FinalPrice.Amount != domain.Euro(36) {
		t.Errorf("vorzeitig: %v %v", got.ReturnedAt, got.FinalPrice)
	}
	_, err = s.Return(early.ID)
	assertErr(t, err, domain.ErrAlreadyReturned)
	second := mustBook(t, s, in(bike, person, at(10, 15), at(11, 15))) // Restzeit wieder frei
	c.set(at(11, 15))
	if _, err := s.Return(second.ID); err != nil {
		t.Fatal(err)
	}

	late := mustBook(t, s, in(bike, person, at(13, 0), at(16, 0)))
	c.set(at(16, 20))
	if _, err := s.Book(in(bike, person, at(17, 0), at(18, 0))); err != domain.ErrItemBooked {
		t.Errorf("überfällige Ausleihe muss blockieren: %v", err)
	}
	got, _ = s.Return(late.ID)
	if got.FinalPrice.Amount != domain.Euro(48) || got.FinalPrice.Explanation != "4 angefangene Stunden × 12,00 €" {
		t.Errorf("verspätet: %v", got.FinalPrice)
	}
	mustBook(t, s, in(bike, person, at(17, 0), at(18, 0)))
	checkInvariants(t, s)
}

// AK-14
func TestCancel(t *testing.T) {
	s, c := newService(t)
	bike, person := addBike(t, s, fahrrad.Citybike), addPerson(t, s)
	res := mustBook(t, s, in(bike, person, at(10, 0), at(12, 0)))
	if err := s.Cancel(res.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Booking(res.ID); err != domain.ErrNotFound {
		t.Errorf("stornierte Buchung muss gelöscht sein: %v", err)
	}
	active := mustBook(t, s, in(bike, person, at(10, 0), at(12, 0)))
	c.set(at(10, 30))
	assertErr(t, s.Cancel(active.ID), domain.ErrNotCancellable)
	assertErr(t, s.Cancel(999), domain.ErrNotFound)
}

// AK-24: Verspätete Rückgabe trifft eine nachfolgende Reservierung.
func TestConflictAndWaiting(t *testing.T) {
	s, c := newService(t)
	bike, p1, p2 := addBike(t, s, fahrrad.Citybike), addPerson(t, s), addPerson(t, s)
	a := mustBook(t, s, in(bike, p1, at(10, 0), at(12, 0)))
	b := mustBook(t, s, in(bike, p2, at(13, 0), at(15, 0)))

	c.set(at(12, 30))
	bv, _ := s.Booking(b.ID)
	if bv.State != domain.StateReserved || !bv.Conflict {
		t.Errorf("12:30: %+v", bv.Situation)
	}
	st, _ := s.Stats(fahrrad.Key)
	if len(st.Conflicts) != 1 || st.Overdue != 1 {
		t.Errorf("Dashboard: %d Konflikte, %d überfällig", len(st.Conflicts), st.Overdue)
	}

	c.set(at(13, 30))
	bv, _ = s.Booking(b.ID)
	if bv.State != domain.StateWaiting || bv.CanReturn() || !bv.CanCancel() {
		t.Errorf("13:30: %+v", bv.Situation)
	}
	_, err := s.Return(b.ID)
	assertErr(t, err, domain.ErrNotStarted)
	checkInvariants(t, s)

	c.set(at(13, 40))
	ra, err := s.Return(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ra.FinalPrice.Amount != domain.Euro(20) {
		t.Errorf("Endpreis A (4 Std × 5 €): %v", ra.FinalPrice)
	}
	bv, _ = s.Booking(b.ID)
	if bv.State != domain.StateActive || bv.Conflict {
		t.Errorf("nach Rückgabe von A: %+v", bv.Situation)
	}
	if _, err := s.Return(b.ID); err != nil {
		t.Errorf("B muss zurückgegeben werden können: %v", err)
	}
	checkInvariants(t, s)
}

// AK-25: Umbuchen auf ein freies Rad, Ablehnung bei belegtem Ziel, Wartung oder aktiver Buchung.
func TestReassign(t *testing.T) {
	s, c := newService(t)
	bike, free, busy, ebike := addBike(t, s, fahrrad.Citybike), addBike(t, s, fahrrad.Citybike), addBike(t, s, fahrrad.Citybike), addBike(t, s, fahrrad.EBike)
	p1, p2 := addPerson(t, s), addPerson(t, s)
	a := mustBook(t, s, in(bike, p1, at(10, 0), at(12, 0)))
	b := mustBook(t, s, in(bike, p2, at(13, 0), at(15, 0)))
	mustBook(t, s, in(busy, p1, at(14, 0), at(16, 0)))

	c.set(at(13, 30)) // A überfällig, B wartet
	view, offers, err := s.Alternatives(b.ID)
	if err != nil || view.State != domain.StateWaiting {
		t.Fatalf("Alternativen: %v %v", view.State, err)
	}
	ids := map[int]bool{}
	for _, o := range offers {
		ids[o.View.Item.ID] = true
	}
	if !ids[free] || !ids[ebike] || ids[busy] || ids[bike] {
		t.Errorf("Alternativen: %v", ids)
	}
	if offers[0].View.Item.Category != fahrrad.Citybike {
		t.Errorf("gleicher Typ muss vorn stehen: %v", offers[0].View.Item.Category)
	}

	_, err = s.Reassign(b.ID, busy)
	assertErr(t, err, domain.ErrItemBooked)
	_, err = s.Reassign(a.ID, free)
	assertErr(t, err, domain.ErrNotReassignable)

	moved, err := s.Reassign(b.ID, ebike)
	if err != nil {
		t.Fatal(err)
	}
	if moved.ItemID != ebike || moved.Price.Amount != domain.Euro(24) {
		t.Errorf("umgebucht auf E-Bike: Rad %d, Preis %v (2 Std × 12 €)", moved.ItemID, moved.Price)
	}
	bv, _ := s.Booking(b.ID)
	if bv.State != domain.StateActive || bv.Conflict {
		t.Errorf("nach Umbuchen: %+v", bv.Situation)
	}
	checkInvariants(t, s)

	if _, err := s.UpdateItem(free, service.ItemInput{Category: fahrrad.Citybike, Attributes: bikeAttrs("Rad"), Status: domain.StatusMaintenance}); err != nil {
		t.Fatal(err)
	}
	res := mustBook(t, s, in(ebike, p2, at(20, 0), at(21, 0)))
	_, err = s.Reassign(res.ID, free)
	assertErr(t, err, domain.ErrItemInMaintenance)
}

// AK-6: Verfügbarkeitssuche.
func TestAvailable(t *testing.T) {
	s, _ := newService(t)
	a, b, cBike, e := addBike(t, s, fahrrad.Citybike), addBike(t, s, fahrrad.Citybike), addBike(t, s, fahrrad.Citybike), addBike(t, s, fahrrad.EBike)
	person := addPerson(t, s)
	mustBook(t, s, in(a, person, at(10, 0), at(12, 0)))
	if _, err := s.UpdateItem(cBike, service.ItemInput{Category: fahrrad.Citybike, Attributes: bikeAttrs("Rad"), Status: domain.StatusMaintenance}); err != nil {
		t.Fatal(err)
	}
	ids := func(cat string, start, end time.Time) map[int]domain.Money {
		offers, err := s.Available(fahrrad.Key, cat, domain.Period{Start: start, End: end})
		if err != nil {
			t.Fatal(err)
		}
		m := map[int]domain.Money{}
		for _, o := range offers {
			m[o.View.Item.ID] = o.Price.Amount
		}
		return m
	}
	if got := ids(fahrrad.Citybike, at(11, 0), at(13, 0)); len(got) != 1 || got[b] != domain.Euro(10) {
		t.Errorf("11–13: %v", got)
	}
	if got := ids(fahrrad.Citybike, at(12, 0), at(13, 0)); len(got) != 2 || got[a] == 0 || got[b] == 0 {
		t.Errorf("12–13: %v", got)
	}
	if got := ids(fahrrad.EBike, at(12, 0), at(13, 0)); len(got) != 1 || got[e] != domain.Euro(12) {
		t.Errorf("E-Bikes: %v", got)
	}
	_, err := s.Available(fahrrad.Key, "", domain.Period{Start: at(12, 0), End: at(12, 30)})
	assertField(t, err, "end", "Verleihdauer muss mindestens 1 Stunde betragen")
}

// AK-17: Filter und Sortierung der Übersicht.
func TestBookingFilters(t *testing.T) {
	s, c := newService(t)
	b1, b2, b3, person := addBike(t, s, fahrrad.Citybike), addBike(t, s, fahrrad.Citybike), addBike(t, s, fahrrad.Citybike), addPerson(t, s)
	closed := mustBook(t, s, in(b1, person, at(8, 0), at(9, 0)))
	active := mustBook(t, s, in(b2, person, at(8, 0), at(12, 0)))
	later := mustBook(t, s, in(b3, person, at(15, 0), at(16, 0)))
	sooner := mustBook(t, s, in(b1, person, at(13, 0), at(14, 0)))
	c.set(at(8, 30))
	if _, err := s.Return(closed.ID); err != nil {
		t.Fatal(err)
	}
	ids := func(f service.BookingFilter) []int {
		var out []int
		for _, v := range s.Bookings(f) {
			out = append(out, v.Booking.ID)
		}
		return out
	}
	if got := ids(service.FilterActive); len(got) != 1 || got[0] != active.ID {
		t.Errorf("aktiv: %v", got)
	}
	if got := ids(service.FilterReserved); len(got) != 2 || got[0] != sooner.ID || got[1] != later.ID {
		t.Errorf("reserviert (nächster zuerst): %v", got)
	}
	if got := ids(service.FilterClosed); len(got) != 1 || got[0] != closed.ID {
		t.Errorf("abgeschlossen: %v", got)
	}
	if got := ids(service.FilterAll); len(got) != 4 {
		t.Errorf("alle: %v", got)
	}
	if service.ParseBookingFilter("quatsch") != service.FilterActive {
		t.Error("Standardfilter muss aktiv sein")
	}
}
