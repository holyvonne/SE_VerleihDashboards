package store

import (
	"errors"
	"sync"
	"testing"
	"time"

	"skiverleih/internal/domain"
)

func newSki() domain.Ski {
	return domain.NewSki("Gipfelwerk", "Pro", 170, domain.SkiTypeAllround, domain.ConditionGood, "")
}

func TestSkiCRUD(t *testing.T) {
	m := NewMemoryStore()
	a := m.CreateSki(newSki())
	b := m.CreateSki(newSki())
	if a.ID != 1 || b.ID != 2 {
		t.Fatalf("IDs = %d, %d; want 1, 2", a.ID, b.ID)
	}

	a.Model = "Neu"
	if err := m.UpdateSki(a); err != nil {
		t.Fatal(err)
	}
	got, err := m.Ski(a.ID)
	if err != nil || got.Model != "Neu" {
		t.Fatalf("Ski(1) = %+v, %v", got, err)
	}

	if err := m.DeleteSki(a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Ski(a.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("nach Löschen: err = %v, want ErrNotFound", err)
	}
	if c := m.CreateSki(newSki()); c.ID != 3 {
		t.Errorf("ID nach Löschen = %d, want 3 (IDs werden nie wiederverwendet)", c.ID)
	}
	if list := m.ListSkis(); len(list) != 2 || list[0].ID != 2 || list[1].ID != 3 {
		t.Errorf("ListSkis = %+v", list)
	}
}

func TestNotFound(t *testing.T) {
	m := NewMemoryStore()
	checks := map[string]error{
		"UpdateSki":    m.UpdateSki(domain.Ski{ID: 9}),
		"DeleteSki":    m.DeleteSki(9),
		"UpdatePerson": m.UpdatePerson(domain.Person{ID: 9}),
		"DeletePerson": m.DeletePerson(9),
		"UpdateRental": m.UpdateRental(domain.Rental{ID: 9}),
		"DeleteRental": m.DeleteRental(9),
	}
	for name, err := range checks {
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", name, err)
		}
	}
	if _, err := m.Person(9); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Person: err = %v", err)
	}
	if _, err := m.Rental(9); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Rental: err = %v", err)
	}
}

// Der Store gibt Kopien heraus: Änderungen am Ergebnis verändern den Store nicht.
func TestPersonImageIsCopied(t *testing.T) {
	m := NewMemoryStore()
	img := &domain.IDImage{Data: []byte{1, 2, 3}, ContentType: "image/png"}
	p := m.CreatePerson(domain.Person{FirstName: "Erika", IDImage: img})

	img.Data[0] = 99 // Original des Aufrufers ändern
	got, _ := m.Person(p.ID)
	if got.IDImage.Data[0] != 1 {
		t.Error("Store teilt Bilddaten mit dem Aufrufer (Create)")
	}

	got.IDImage.Data[1] = 99 // Ergebnis ändern
	again, _ := m.Person(p.ID)
	if again.IDImage.Data[1] != 2 {
		t.Error("Store teilt Bilddaten mit dem Aufrufer (Person)")
	}

	list := m.ListPersons()
	list[0].IDImage.Data[2] = 99
	again, _ = m.Person(p.ID)
	if again.IDImage.Data[2] != 3 {
		t.Error("Store teilt Bilddaten mit dem Aufrufer (ListPersons)")
	}
}

func TestRentalReturnedAtIsCopied(t *testing.T) {
	m := NewMemoryStore()
	start := time.Now()
	r := m.CreateRental(domain.NewRental(1, 1, domain.Period{Start: start, End: start.Add(time.Hour)}, start, domain.Euro(20)))
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	r.ReturnedAt = &at
	if err := m.UpdateRental(r); err != nil {
		t.Fatal(err)
	}
	at = at.Add(time.Hour) // Variable des Aufrufers ändern
	got, _ := m.Rental(r.ID)
	if got.ReturnedAt.Hour() != 12 {
		t.Error("Store teilt ReturnedAt mit dem Aufrufer")
	}
}

func TestConcurrentCreate(t *testing.T) {
	m := NewMemoryStore()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.CreateSki(newSki())
		}()
	}
	wg.Wait()
	list := m.ListSkis()
	if len(list) != 100 || list[99].ID != 100 {
		t.Errorf("len = %d, letzte ID = %d; want 100, 100", len(list), list[len(list)-1].ID)
	}
}
