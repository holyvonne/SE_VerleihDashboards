package service_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/plugin"
	"fahrradverleih/framework/service"
	"fahrradverleih/framework/store"
	"fahrradverleih/plugins/fahrrad"
)

// Die Tests nutzen das echte Fahrrad-Plugin; das ist erlaubt, weil A-R1 nur für Nicht-Test-Code gilt.

var testLoc = time.FixedZone("TEST", 3600)

// at liefert 10.11.2026 h:m; der Testtag beginnt um 08:00.
func at(h, m int) time.Time { return time.Date(2026, 11, 10, h, m, 0, 0, testLoc) }

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) set(t time.Time)     { c.mu.Lock(); defer c.mu.Unlock(); c.t = t }
func (c *clock) add(d time.Duration) { c.mu.Lock(); defer c.mu.Unlock(); c.t = c.t.Add(d) }

func newService(t *testing.T, plugins ...plugin.Plugin) (*service.Service, *clock) {
	t.Helper()
	if len(plugins) == 0 {
		p, err := fahrrad.New(fahrrad.DefaultConfig())
		if err != nil {
			t.Fatal(err)
		}
		plugins = []plugin.Plugin{p}
	}
	reg := plugin.NewRegistry()
	for _, p := range plugins {
		if err := reg.Register(p); err != nil {
			t.Fatal(err)
		}
	}
	c := &clock{t: at(8, 0)}
	return service.New(store.NewMemoryStore(), reg, c.now), c
}

func bikeAttrs(brand string) map[string]string {
	return map[string]string{fahrrad.AttrBrand: brand, fahrrad.AttrModel: "Test", fahrrad.AttrFrame: "M"}
}

func addBike(t *testing.T, s *service.Service, category string) int {
	t.Helper()
	attrs := bikeAttrs("Rad")
	if category == fahrrad.EBike {
		attrs[fahrrad.AttrBattery] = "500"
	}
	it, err := s.CreateItem(service.ItemInput{Kind: fahrrad.Key, Category: category, Attributes: attrs})
	if err != nil {
		t.Fatal(err)
	}
	return it.ID
}

var personSeq int
var personMu sync.Mutex

func addPerson(t *testing.T, s *service.Service) int {
	t.Helper()
	personMu.Lock()
	personSeq++
	n := personSeq
	personMu.Unlock()
	p, err := s.CreatePerson(service.PersonInput{FirstName: "Test", LastName: fmt.Sprintf("Person%d", n), IDType: domain.IDCard, IDNumber: fmt.Sprintf("TEST%04d", n)})
	if err != nil {
		t.Fatal(err)
	}
	return p.ID
}

func in(item, person int, start, end time.Time) service.BookingInput {
	return service.BookingInput{ItemID: item, PersonID: person, Start: start, End: end}
}

func mustBook(t *testing.T, s *service.Service, b service.BookingInput) domain.Booking {
	t.Helper()
	bk, err := s.Book(b)
	if err != nil {
		t.Fatalf("Buchung %v–%v: %v", b.Start.Format("15:04"), b.End.Format("15:04"), err)
	}
	return bk
}

func assertErr(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("Fehler %v, want %v", err, want)
	}
}

func assertField(t *testing.T, err error, field, msg string) {
	t.Helper()
	v, ok := domain.AsValidation(err)
	if !ok {
		t.Fatalf("erwartet Feldfehler, got %v", err)
	}
	if v[field] != msg {
		t.Fatalf("Feld %q: %q, want %q (alle: %v)", field, v[field], msg, v)
	}
}

// checkInvariants prüft I-1, I-2a, I-3, I-4 und I-6 (Spezifikation 4.9).
func checkInvariants(t *testing.T, s *service.Service) {
	t.Helper()
	activePerItem := map[int]int{}
	openPerItem := map[int]int{}
	for _, v := range s.Bookings(service.FilterAll) {
		b := v.Booking
		if v.Item.Item.ID == 0 || v.Person.ID == 0 {
			t.Errorf("I-3: Buchung %d verweist ins Leere", b.ID)
		}
		if !b.PlannedEnd.After(b.Start) || (b.ReturnedAt != nil && b.ReturnedAt.Before(b.Start)) {
			t.Errorf("I-4: Buchung %d hat ungültige Zeiten", b.ID)
		}
		if v.State == domain.StateActive {
			activePerItem[b.ItemID]++
		}
		if v.Open() {
			openPerItem[b.ItemID]++
		}
	}
	for _, it := range s.Items(service.ItemFilter{}) {
		id := it.Item.ID
		if activePerItem[id] > 1 {
			t.Errorf("I-2a: Objekt %d hat %d aktive Ausleihen", id, activePerItem[id])
		}
		if (it.Status == domain.StatusRented) != (activePerItem[id] == 1) {
			t.Errorf("I-1: Objekt %d hat Status %s bei %d aktiven Ausleihen", id, it.Status, activePerItem[id])
		}
		if it.Item.InMaintenance() && openPerItem[id] > 0 {
			t.Errorf("I-6: Objekt %d in Wartung hat offene Buchungen", id)
		}
	}
}
