package store

import (
	"errors"
	"testing"
	"time"

	"fahrradverleih/framework/domain"
)

func TestIDsAreNeverReused(t *testing.T) {
	s := NewMemoryStore()
	a := s.CreateItem(domain.Item{Kind: "k"})
	_ = s.DeleteItem(a.ID)
	b := s.CreateItem(domain.Item{Kind: "k"})
	if a.ID != 1 || b.ID != 2 {
		t.Errorf("IDs %d, %d; erwartet 1, 2", a.ID, b.ID)
	}
}

func TestStoreReturnsCopies(t *testing.T) {
	s := NewMemoryStore()
	it := s.CreateItem(domain.Item{Kind: "k", Attributes: map[string]string{"a": "1"}})
	it.Attributes["a"] = "geändert"
	got, _ := s.Item(it.ID)
	if got.Attributes["a"] != "1" {
		t.Error("Store gibt die interne Map heraus")
	}

	now := time.Now()
	b := s.CreateBooking(domain.Booking{ItemID: 1, PersonID: 1, ReturnedAt: &now})
	*b.ReturnedAt = now.Add(time.Hour)
	stored, _ := s.Booking(b.ID)
	if !stored.ReturnedAt.Equal(now) {
		t.Error("Store teilt ReturnedAt")
	}
}

func TestNotFound(t *testing.T) {
	s := NewMemoryStore()
	checks := map[string]error{
		"Item":          func() error { _, err := s.Item(9); return err }(),
		"UpdateItem":    s.UpdateItem(domain.Item{ID: 9}),
		"DeleteItem":    s.DeleteItem(9),
		"Person":        func() error { _, err := s.Person(9); return err }(),
		"UpdatePerson":  s.UpdatePerson(domain.Person{ID: 9}),
		"DeletePerson":  s.DeletePerson(9),
		"Booking":       func() error { _, err := s.Booking(9); return err }(),
		"UpdateBooking": s.UpdateBooking(domain.Booking{ID: 9}),
		"DeleteBooking": s.DeleteBooking(9),
	}
	for name, err := range checks {
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
