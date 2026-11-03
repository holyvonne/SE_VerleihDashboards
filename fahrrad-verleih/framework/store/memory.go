// Package store hält alle Daten im Arbeitsspeicher (NF-3). Es kennt kein HTTP und keine
// Geschäftsregeln außer der ID-Vergabe und gibt immer Kopien zurück.
package store

import (
	"sort"
	"sync"

	"fahrradverleih/framework/domain"
)

// MemoryStore speichert Objekte, Personen und Buchungen in Maps. IDs werden nie wiederverwendet.
type MemoryStore struct {
	mu                                sync.RWMutex
	items                             map[int]domain.Item
	persons                           map[int]domain.Person
	bookings                          map[int]domain.Booking
	nextItem, nextPerson, nextBooking int
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		items:    map[int]domain.Item{},
		persons:  map[int]domain.Person{},
		bookings: map[int]domain.Booking{},
	}
}

// ---------- Objekte ----------

func (s *MemoryStore) CreateItem(it domain.Item) domain.Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextItem++
	it.ID = s.nextItem
	s.items[it.ID] = it.Copy()
	return it.Copy()
}

func (s *MemoryStore) Item(id int) (domain.Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	it, ok := s.items[id]
	if !ok {
		return domain.Item{}, domain.ErrNotFound
	}
	return it.Copy(), nil
}

func (s *MemoryStore) UpdateItem(it domain.Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[it.ID]; !ok {
		return domain.ErrNotFound
	}
	s.items[it.ID] = it.Copy()
	return nil
}

func (s *MemoryStore) DeleteItem(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.items, id)
	return nil
}

func (s *MemoryStore) ListItems() []domain.Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Item, 0, len(s.items))
	for _, it := range s.items {
		out = append(out, it.Copy())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ---------- Personen ----------

func (s *MemoryStore) CreatePerson(p domain.Person) domain.Person {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextPerson++
	p.ID = s.nextPerson
	s.persons[p.ID] = p.Copy()
	return p.Copy()
}

func (s *MemoryStore) Person(id int) (domain.Person, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.persons[id]
	if !ok {
		return domain.Person{}, domain.ErrNotFound
	}
	return p.Copy(), nil
}

func (s *MemoryStore) UpdatePerson(p domain.Person) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.persons[p.ID]; !ok {
		return domain.ErrNotFound
	}
	s.persons[p.ID] = p.Copy()
	return nil
}

func (s *MemoryStore) DeletePerson(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.persons[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.persons, id)
	return nil
}

func (s *MemoryStore) ListPersons() []domain.Person {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Person, 0, len(s.persons))
	for _, p := range s.persons {
		out = append(out, p.Copy())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ---------- Buchungen ----------

func (s *MemoryStore) CreateBooking(b domain.Booking) domain.Booking {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextBooking++
	b.ID = s.nextBooking
	s.bookings[b.ID] = b.Copy()
	return b.Copy()
}

func (s *MemoryStore) Booking(id int) (domain.Booking, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.bookings[id]
	if !ok {
		return domain.Booking{}, domain.ErrNotFound
	}
	return b.Copy(), nil
}

func (s *MemoryStore) UpdateBooking(b domain.Booking) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bookings[b.ID]; !ok {
		return domain.ErrNotFound
	}
	s.bookings[b.ID] = b.Copy()
	return nil
}

func (s *MemoryStore) DeleteBooking(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bookings[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.bookings, id)
	return nil
}

func (s *MemoryStore) ListBookings() []domain.Booking {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Booking, 0, len(s.bookings))
	for _, b := range s.bookings {
		out = append(out, b.Copy())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
