package service

import (
	"errors"
	"testing"
	"time"

	"skiverleih/internal/domain"
	"skiverleih/internal/store"
)

// Feste Zeitzone, damit Kalendertage in Tests nicht von der Maschine abhängen.
var testLoc = time.FixedZone("Testzeit", 3600)

const (
	hour = time.Hour
	day  = 24 * time.Hour
)

// clock ist eine steuerbare Uhr für deterministische Tests (NF-12).
// Nebenläufigkeitstests lesen sie nur; advance läuft nie parallel.
type clock struct {
	t time.Time
}

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestService(t *testing.T) (*Service, *clock) {
	t.Helper()
	c := &clock{t: time.Date(2026, 11, 10, 10, 0, 0, 0, testLoc)}
	pricing := domain.DailyRate{PerDay: domain.Euro(20), Location: testLoc}
	return New(store.NewMemoryStore(), c.now, pricing), c
}

// Nur fiktive Testdaten (NF-11).
func skiInput() SkiInput {
	return SkiInput{Brand: "Gipfelwerk", Model: "Pro", LengthCM: 170, Type: domain.SkiTypeAllround, Condition: domain.ConditionGood}
}

func personInput(idNumber string) PersonInput {
	return PersonInput{FirstName: "Erika", LastName: "Mustermann", IDType: domain.IDPassport, IDNumber: idNumber}
}

func mustSki(t *testing.T, s *Service) domain.Ski {
	t.Helper()
	sk, err := s.CreateSki(skiInput())
	if err != nil {
		t.Fatalf("CreateSki: %v", err)
	}
	return sk
}

func mustPerson(t *testing.T, s *Service, idNumber string) domain.Person {
	t.Helper()
	p, err := s.CreatePerson(personInput(idNumber))
	if err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}
	return p
}

// booking erzeugt Eingaben für [now+startIn, now+startIn+length).
func booking(c *clock, skiID, personID int, startIn, length time.Duration) RentalInput {
	start := c.now().Add(startIn)
	return RentalInput{SkiID: skiID, PersonID: personID, Start: start, End: start.Add(length)}
}

func mustBook(t *testing.T, s *Service, in RentalInput) domain.Rental {
	t.Helper()
	r, err := s.Rent(in)
	if err != nil {
		t.Fatalf("Rent(%+v): %v", in, err)
	}
	return r
}

// mustRent bucht ab jetzt für einen Tag (aktive Ausleihe).
func mustRent(t *testing.T, s *Service, c *clock, skiID, personID int) domain.Rental {
	t.Helper()
	return mustBook(t, s, booking(c, skiID, personID, 0, day))
}

// checkInvariants prüft I-1 bis I-4 und I-6 aus Spezifikation 3.8 direkt am Speicher.
func checkInvariants(t *testing.T, s *Service) {
	t.Helper()
	now := s.now()
	rentals := s.repo.ListRentals()
	for _, r := range rentals {
		if _, err := s.repo.Ski(r.SkiID); err != nil {
			t.Errorf("I-3: Buchung %d verweist auf fehlenden Ski %d", r.ID, r.SkiID)
		}
		if _, err := s.repo.Person(r.PersonID); err != nil {
			t.Errorf("I-3: Buchung %d verweist auf fehlende Person %d", r.ID, r.PersonID)
		}
		if !r.PlannedEnd.After(r.Start) || (r.ReturnedAt != nil && r.ReturnedAt.Before(r.Start)) {
			t.Errorf("I-4: Buchung %d hat ungültige Zeitpunkte", r.ID)
		}
	}
	// I-2: Offene Buchungen überschneiden sich nur durch überfällige Ausleihen, und dann ist die spätere
	// als Konflikt markiert (3.6a). Eine verspätete, abgeschlossene Rückgabe darf in der Historie überlappen.
	for i, a := range rentals {
		for _, b := range rentals[i+1:] {
			if a.SkiID != b.SkiID || a.IsReturned() || b.IsReturned() || !a.Blocks(now).Overlaps(b.Blocks(now)) {
				continue
			}
			later := b
			if b.Start.Before(a.Start) {
				later = a
			}
			if !domain.Evaluate(later, rentals, now).Conflict {
				t.Errorf("I-2: Buchungen %d und %d für Ski %d überschneiden sich ohne Konflikt-Markierung", a.ID, b.ID, a.SkiID)
			}
		}
	}
	for _, sk := range s.repo.ListSkis() {
		active := 0
		for _, r := range rentals {
			if r.SkiID != sk.ID {
				continue
			}
			if domain.Evaluate(r, rentals, now).State == domain.RentalActive {
				active++
			}
			if sk.InMaintenance() && !r.IsReturned() {
				t.Errorf("I-6: Ski %d in Wartung hat offene Buchung %d", sk.ID, r.ID)
			}
		}
		if active > 1 {
			t.Errorf("I-2a: Ski %d hat %d aktive Ausleihen", sk.ID, active)
		}
		if (sk.StatusAt(rentals, now) == domain.SkiRented) != (active == 1) {
			t.Errorf("I-1: Ski %d Status %q bei %d aktiven Ausleihen", sk.ID, sk.StatusAt(rentals, now), active)
		}
	}
}

func assertField(t *testing.T, err error, field string) {
	t.Helper()
	var verr domain.ValidationErrors
	if !errors.As(err, &verr) {
		t.Fatalf("err = %v, want ValidationErrors mit Feld %q", err, field)
	}
	if _, ok := verr[field]; !ok {
		t.Fatalf("Feld %q fehlt in %v", field, verr)
	}
}
