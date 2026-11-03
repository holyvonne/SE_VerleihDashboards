package service

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"skiverleih/internal/domain"
)

// AK-8: Buchen ab jetzt macht den Ski zur aktiven Ausleihe und ändert die Kennzahlen.
func TestRent_Now_UpdatesDashboard(t *testing.T) {
	s, c := newTestService(t)
	ski := mustSki(t, s)
	mustSki(t, s)
	p := mustPerson(t, s, "TEST0001")

	before := s.DashboardStats()
	r := mustRent(t, s, c, ski.ID, p.ID)
	after := s.DashboardStats()

	if !r.IsActive(c.now()) || !r.BookedAt.Equal(c.now()) || r.Price != domain.Euro(40) {
		t.Errorf("Buchung falsch: %+v", r) // 10.11. 10:00 – 11.11. 10:00 = 2 Kalendertage
	}
	if v := s.Skis("")[0]; v.Status != domain.SkiRented {
		t.Errorf("Status = %q, want ausgeliehen", v.Status)
	}
	if after.Available != before.Available-1 || after.Rented != before.Rented+1 || after.Reserved != 0 {
		t.Errorf("Dashboard vorher %+v, nachher %+v", before, after)
	}
	if len(after.ActiveRentals) != 1 || after.ActiveRentals[0].Person.FullName() != "Erika Mustermann" {
		t.Errorf("ActiveRentals = %+v", after.ActiveRentals)
	}
	checkInvariants(t, s)
}

// AK-15: Reservierung markiert den Ski nicht sofort; zur Startzeit wird sie aktiv.
func TestRent_FutureReservation(t *testing.T) {
	s, c := newTestService(t)
	ski := mustSki(t, s)
	p := mustPerson(t, s, "TEST0001")

	r := mustBook(t, s, booking(c, ski.ID, p.ID, 2*day, day))
	if !r.IsReserved(c.now()) {
		t.Fatal("Buchung in der Zukunft muss reserviert sein")
	}
	st := s.DashboardStats()
	if st.Available != 1 || st.Rented != 0 || st.Reserved != 1 || len(st.UpcomingReservations) != 1 {
		t.Errorf("Dashboard mit Reservierung: %+v", st)
	}
	v := s.Skis("")[0]
	if v.Status != domain.SkiAvailable || v.NextReservation == nil || v.NextReservation.ID != r.ID {
		t.Errorf("SkiView = %+v", v)
	}
	if rows := s.Persons(""); rows[0].Reservations != 1 || rows[0].ActiveRentals != 0 {
		t.Errorf("PersonRow = %+v", rows[0])
	}
	checkInvariants(t, s)

	c.advance(2 * day) // Beginn erreicht – ohne Speichervorgang
	st = s.DashboardStats()
	if st.Rented != 1 || st.Reserved != 0 || st.Available != 0 {
		t.Errorf("nach Beginn: %+v", st)
	}
	if v := s.Skis("")[0]; v.Status != domain.SkiRented {
		t.Errorf("nach Beginn: Status %q", v.Status)
	}
	checkInvariants(t, s)
}

// AK-16, F-4.7: Überschneidungen für denselben Ski werden abgelehnt.
func TestRent_Overlaps(t *testing.T) {
	s, c := newTestService(t)
	ski := mustSki(t, s)
	other := mustSki(t, s)
	p := mustPerson(t, s, "TEST0001")
	// bestehend: [12.11. 10:00, 14.11. 10:00)
	mustBook(t, s, booking(c, ski.ID, p.ID, 2*day, 2*day))

	tests := []struct {
		name    string
		in      RentalInput
		wantErr bool
	}{
		{"teilweise", booking(c, ski.ID, p.ID, 3*day, 2*day), true},
		{"umschließend", booking(c, ski.ID, p.ID, day, 5*day), true},
		{"enthalten", booking(c, ski.ID, p.ID, 2*day+2*hour, day), true},
		{"identisch", booking(c, ski.ID, p.ID, 2*day, 2*day), true},
		{"angrenzend davor", booking(c, ski.ID, p.ID, 0, 2*day), false},
		{"angrenzend danach", booking(c, ski.ID, p.ID, 4*day, 2*day), false},
		{"anderer Ski", booking(c, other.ID, p.ID, 2*day, 2*day), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.Quote(tt.in)
			if tt.wantErr != errors.Is(err, domain.ErrSkiBooked) {
				t.Errorf("Quote: err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Quote: unerwarteter Fehler %v", err)
			}
		})
	}
	// Die erlaubten auch wirklich buchen – danach darf nichts überlappen.
	mustBook(t, s, booking(c, ski.ID, p.ID, 0, 2*day))
	mustBook(t, s, booking(c, ski.ID, p.ID, 4*day, 2*day))
	if _, err := s.Rent(booking(c, ski.ID, p.ID, 3*day, day)); !errors.Is(err, domain.ErrSkiBooked) {
		t.Errorf("Rent in belegtem Zeitraum: err = %v", err)
	}
	checkInvariants(t, s)
}

// AK-9: Wartung und laufende Ausleihe verhindern eine Buchung.
func TestRent_MaintenanceAndRented(t *testing.T) {
	s, c := newTestService(t)
	rented := mustSki(t, s)
	maint := mustSki(t, s)
	p := mustPerson(t, s, "TEST0001")
	mustRent(t, s, c, rented.ID, p.ID)
	in := skiInput()
	in.Status = domain.SkiMaintenance
	if _, err := s.UpdateSki(maint.ID, in); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Rent(booking(c, maint.ID, p.ID, 3*day, day)); !errors.Is(err, domain.ErrSkiInMaintenance) {
		t.Errorf("Wartung: err = %v, want ErrSkiInMaintenance", err)
	}
	if _, err := s.Rent(booking(c, rented.ID, p.ID, hour, hour)); !errors.Is(err, domain.ErrSkiBooked) {
		t.Errorf("ausgeliehen: err = %v, want ErrSkiBooked", err)
	}
	// Nach Ende der laufenden Ausleihe darf der Ski reserviert werden.
	mustBook(t, s, booking(c, rented.ID, p.ID, day, day))
	if n := len(s.Rentals(FilterAll)); n != 2 {
		t.Errorf("%d Buchungen, want 2", n)
	}
	checkInvariants(t, s)
}

// AK-19: Eingabefehler werden als Feldfehler gemeldet.
func TestRent_Validation(t *testing.T) {
	s, c := newTestService(t)
	ski := mustSki(t, s)
	p := mustPerson(t, s, "TEST0001")

	_, err := s.Rent(booking(c, ski.ID, p.ID, 0, -hour))
	assertField(t, err, "end")
	_, err = s.Rent(booking(c, ski.ID, p.ID, -hour, 2*hour))
	assertField(t, err, "start")
	_, err = s.Rent(booking(c, ski.ID, p.ID, 0, 61*day))
	assertField(t, err, "end")
	_, err = s.Rent(RentalInput{SkiID: 999, PersonID: 0, Start: c.now(), End: c.now().Add(day)})
	assertField(t, err, "ski_id")
	assertField(t, err, "person_id")
	_, err = s.Rent(RentalInput{SkiID: ski.ID, PersonID: p.ID})
	assertField(t, err, "start")
	assertField(t, err, "end")

	if n := len(s.Rentals(FilterAll)); n != 0 {
		t.Errorf("%d Buchungen trotz Fehlern", n)
	}
}

// AK-12, AK-13: Vorschau berechnet Preis und speichert nichts.
func TestQuote(t *testing.T) {
	s, c := newTestService(t)
	ski := mustSki(t, s)
	p := mustPerson(t, s, "TEST0001")

	q, err := s.Quote(booking(c, ski.ID, p.ID, 0, 7*hour)) // 10:00–17:00
	if err != nil {
		t.Fatal(err)
	}
	if q.Price != domain.Euro(20) || q.IsReservation || q.Duration() != 7*hour ||
		q.Explanation != "1 angefangener Kalendertag × 20,00 €" || q.Ski.ID != ski.ID || q.Person.ID != p.ID {
		t.Errorf("Quote = %+v", q)
	}

	q, _ = s.Quote(booking(c, ski.ID, p.ID, 3*day, 3*day)) // 13.11. 10:00 – 16.11. 10:00
	if q.Price != domain.Euro(80) || !q.IsReservation {
		t.Errorf("Reservierung: Price = %v, IsReservation = %v", q.Price, q.IsReservation)
	}
	if n := len(s.Rentals(FilterAll)); n != 0 {
		t.Errorf("Vorschau hat %d Buchungen angelegt", n)
	}
	if s.PricingRule() != "20,00 € pro angefangenem Kalendertag, mindestens 20,00 €" {
		t.Errorf("PricingRule = %q", s.PricingRule())
	}
}

// F-4.4, AK-10, AK-17: Rückgabe, Endpreis, Freigabe der Restzeit.
func TestReturn(t *testing.T) {
	s, c := newTestService(t)
	ski := mustSki(t, s)
	p := mustPerson(t, s, "TEST0001")
	r := mustBook(t, s, booking(c, ski.ID, p.ID, 0, 50*hour)) // 3 Kalendertage = 60 €

	c.advance(2 * hour) // vorzeitig
	back, err := s.Return(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !back.ReturnedAt.Equal(c.now()) || back.FinalPrice != domain.Euro(60) {
		t.Errorf("Rückgabe: %+v", back)
	}
	if v := s.Skis("")[0]; v.Status != domain.SkiAvailable {
		t.Errorf("Status nach Rückgabe: %q", v.Status)
	}
	// Restzeit ist wieder frei.
	mustBook(t, s, booking(c, ski.ID, p.ID, hour, day))

	if _, err := s.Return(r.ID); !errors.Is(err, domain.ErrAlreadyReturned) {
		t.Errorf("zweite Rückgabe: err = %v", err)
	}
	if _, err := s.Return(999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unbekannt: err = %v", err)
	}
	checkInvariants(t, s)
}

// AK-17: verspätete Rückgabe kostet zusätzliche Kalendertage; überfällig blockiert.
func TestReturn_LateAndOverdueBlocks(t *testing.T) {
	s, c := newTestService(t)
	ski := mustSki(t, s)
	p := mustPerson(t, s, "TEST0001")
	r := mustBook(t, s, booking(c, ski.ID, p.ID, 0, 50*hour)) // bis 12.11. 12:00

	c.advance(70 * hour) // 13.11. 08:00 – überfällig
	st := s.DashboardStats()
	if st.Overdue != 1 || !st.ActiveRentals[0].Overdue {
		t.Errorf("überfällig nicht erkannt: %+v", st)
	}
	if _, err := s.Rent(booking(c, ski.ID, p.ID, 2*day, day)); !errors.Is(err, domain.ErrSkiBooked) {
		t.Errorf("überfällige Ausleihe blockiert nicht: err = %v", err)
	}

	back, err := s.Return(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.FinalPrice != domain.Euro(80) { // 10.–13.11. = 4 Kalendertage
		t.Errorf("FinalPrice = %v, want 80,00 €", back.FinalPrice)
	}
	if v := s.Rentals(FilterClosed)[0]; v.Price != domain.Euro(80) || v.Duration != 70*hour {
		t.Errorf("Übersicht: Price %v, Duration %v", v.Price, v.Duration)
	}
	mustBook(t, s, booking(c, ski.ID, p.ID, 2*day, day))
	checkInvariants(t, s)
}

// F-4.5, AK-18: Stornieren nur vor Beginn; Reservierung nicht zurückgebbar.
func TestCancel(t *testing.T) {
	s, c := newTestService(t)
	ski := mustSki(t, s)
	p := mustPerson(t, s, "TEST0001")
	res := mustBook(t, s, booking(c, ski.ID, p.ID, day, day))
	active := mustRent(t, s, c, mustSki(t, s).ID, p.ID)

	if _, err := s.Return(res.ID); !errors.Is(err, domain.ErrNotStarted) {
		t.Errorf("Rückgabe einer Reservierung: err = %v, want ErrNotStarted", err)
	}
	if err := s.Cancel(active.ID); !errors.Is(err, domain.ErrNotCancellable) {
		t.Errorf("Stornieren einer aktiven Ausleihe: err = %v", err)
	}
	if err := s.Cancel(res.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if _, err := s.repo.Rental(res.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Error("stornierte Reservierung existiert noch")
	}
	mustBook(t, s, booking(c, ski.ID, p.ID, day, day)) // Zeitraum wieder frei
	if err := s.Cancel(res.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("zweites Stornieren: err = %v", err)
	}
	checkInvariants(t, s)
}

// F-4.6: Eine Person darf mehrere Ski gleichzeitig buchen.
func TestRent_PersonMayRentSeveralSkis(t *testing.T) {
	s, c := newTestService(t)
	p := mustPerson(t, s, "TEST0001")
	for i := 0; i < 3; i++ {
		mustRent(t, s, c, mustSki(t, s).ID, p.ID)
	}
	if rows := s.Persons(""); rows[0].ActiveRentals != 3 {
		t.Errorf("ActiveRentals = %d, want 3", rows[0].ActiveRentals)
	}
	checkInvariants(t, s)
}

// AK-11: Viele gleichzeitige Anfragen für denselben Ski und Zeitraum – genau eine gewinnt.
func TestRent_ConcurrentSameSki(t *testing.T) {
	s, c := newTestService(t)
	ski := mustSki(t, s)
	persons := makePersons(t, s, 100)
	ok, rejected := runConcurrent(t, persons, func(i, personID int) error {
		_, err := s.Rent(booking(c, ski.ID, personID, 0, day))
		return err
	})
	if ok != 1 || rejected != len(persons)-1 {
		t.Errorf("erfolgreich = %d, abgelehnt = %d; want 1, %d", ok, rejected, len(persons)-1)
	}
	checkInvariants(t, s)
}

// AK-20: Parallele, unterschiedlich überlappende Reservierungen – genau eine gewinnt.
func TestRent_ConcurrentOverlappingReservations(t *testing.T) {
	s, c := newTestService(t)
	ski := mustSki(t, s)
	persons := makePersons(t, s, 50)
	// Alle Zeiträume enthalten den 20.11. 12:00 und überlappen sich deshalb paarweise.
	ok, rejected := runConcurrent(t, persons, func(i, personID int) error {
		_, err := s.Rent(booking(c, ski.ID, personID, 10*day-time.Duration(i)*hour, 2*day+time.Duration(i)*hour))
		return err
	})
	if ok != 1 || rejected != len(persons)-1 {
		t.Errorf("erfolgreich = %d, abgelehnt = %d", ok, rejected)
	}
	if n := len(s.Rentals(FilterReserved)); n != 1 {
		t.Errorf("%d Reservierungen, want 1", n)
	}
	checkInvariants(t, s)
}

// Paralleles Buchen, Zurückgeben und Lesen über mehrere Ski hält die Invarianten ein.
func TestRentReturn_ConcurrentMixed(t *testing.T) {
	s, c := newTestService(t)
	const skis = 5
	var ids []int
	for i := 0; i < skis; i++ {
		ids = append(ids, mustSki(t, s).ID)
	}
	p := mustPerson(t, s, "TEST0001")

	var wg sync.WaitGroup
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				skiID := ids[(g+i)%skis]
				if r, err := s.Rent(booking(c, skiID, p.ID, time.Duration(i%3)*day, day)); err == nil && i%2 == 0 {
					if r.IsActive(c.now()) {
						_, _ = s.Return(r.ID)
					} else {
						_ = s.Cancel(r.ID)
					}
				}
				_ = s.DashboardStats() // Leser parallel zu Schreibern
			}
		}(g)
	}
	wg.Wait()
	checkInvariants(t, s)
}

// F-5: Filter und Sortierung der Übersicht.
func TestRentals_Filters(t *testing.T) {
	s, c := newTestService(t)
	p := mustPerson(t, s, "TEST0001")
	closed := mustRent(t, s, c, mustSki(t, s).ID, p.ID)
	active := mustRent(t, s, c, mustSki(t, s).ID, p.ID)
	later := mustBook(t, s, booking(c, mustSki(t, s).ID, p.ID, 5*day, day))
	sooner := mustBook(t, s, booking(c, mustSki(t, s).ID, p.ID, 2*day, day))
	c.advance(hour)
	if _, err := s.Return(closed.ID); err != nil {
		t.Fatal(err)
	}

	ids := func(v []RentalView) []int {
		var out []int
		for _, x := range v {
			out = append(out, x.Rental.ID)
		}
		return out
	}
	if got := ids(s.Rentals(FilterActive)); len(got) != 1 || got[0] != active.ID {
		t.Errorf("aktiv = %v", got)
	}
	if got := ids(s.Rentals(FilterReserved)); len(got) != 2 || got[0] != sooner.ID || got[1] != later.ID {
		t.Errorf("reserviert (nächste zuerst) = %v", got)
	}
	if got := ids(s.Rentals(FilterClosed)); len(got) != 1 || got[0] != closed.ID {
		t.Errorf("abgeschlossen = %v", got)
	}
	if got := ids(s.Rentals(FilterAll)); len(got) != 4 || got[0] != later.ID {
		t.Errorf("alle (neuester Beginn zuerst) = %v", got)
	}
}

func TestParseRentalFilter(t *testing.T) {
	cases := map[string]RentalFilter{"": FilterActive, "aktiv": FilterActive, "reserviert": FilterReserved,
		"abgeschlossen": FilterClosed, "alle": FilterAll, "quatsch": FilterActive}
	for in, want := range cases {
		if got := ParseRentalFilter(in); got != want {
			t.Errorf("ParseRentalFilter(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---------- Hilfen für Nebenläufigkeitstests ----------

func makePersons(t *testing.T, s *Service, n int) []domain.Person {
	t.Helper()
	out := make([]domain.Person, n)
	for i := range out {
		out[i] = mustPerson(t, s, "TEST"+string(rune('A'+i/26))+string(rune('A'+i%26))+"00")
	}
	return out
}

// runConcurrent startet für jede Person eine Goroutine, die möglichst gleichzeitig
// losläuft, und zählt Erfolge und Ablehnungen wegen Überschneidung.
func runConcurrent(t *testing.T, persons []domain.Person, op func(i, personID int) error) (ok, rejected int) {
	t.Helper()
	var okN, rejN atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i, p := range persons {
		wg.Add(1)
		go func(i, personID int) {
			defer wg.Done()
			<-start
			switch err := op(i, personID); {
			case err == nil:
				okN.Add(1)
			case errors.Is(err, domain.ErrSkiBooked):
				rejN.Add(1)
			default:
				t.Errorf("unerwarteter Fehler: %v", err)
			}
		}(i, p.ID)
	}
	close(start)
	wg.Wait()
	return int(okN.Load()), int(rejN.Load())
}
