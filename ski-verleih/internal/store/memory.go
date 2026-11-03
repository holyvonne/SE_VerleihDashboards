// Package store hält alle Daten im Arbeitsspeicher (NF-3).
// Er kennt weder HTTP noch Geschäftsregeln – nur Speichern, Laden und ID-Vergabe.
package store

import (
	"bytes"
	"sort"
	"sync"

	"skiverleih/internal/domain"
)

// MemoryStore speichert Ski, Personen und Ausleihen in Maps.
// Alle Methoden sind threadsicher und geben Kopien zurück, damit niemand
// die gespeicherten Daten an der Sperre vorbei verändern kann.
type MemoryStore struct {
	mu      sync.RWMutex
	skis    map[int]domain.Ski
	persons map[int]domain.Person
	rentals map[int]domain.Rental
	// Zähler für fortlaufende IDs; gelöschte IDs werden nie wiederverwendet.
	lastSkiID, lastPersonID, lastRentalID int
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		skis:    map[int]domain.Ski{},
		persons: map[int]domain.Person{},
		rentals: map[int]domain.Rental{},
	}
}

// ---------- Ski ----------

func (m *MemoryStore) CreateSki(s domain.Ski) domain.Ski {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastSkiID++
	s.ID = m.lastSkiID
	m.skis[s.ID] = s
	return s
}

func (m *MemoryStore) UpdateSki(s domain.Ski) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.skis[s.ID]; !ok {
		return domain.ErrNotFound
	}
	m.skis[s.ID] = s
	return nil
}

func (m *MemoryStore) Ski(id int) (domain.Ski, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.skis[id]
	if !ok {
		return domain.Ski{}, domain.ErrNotFound
	}
	return s, nil
}

// ListSkis liefert alle Ski, sortiert nach ID.
func (m *MemoryStore) ListSkis() []domain.Ski {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.Ski, 0, len(m.skis))
	for _, s := range m.skis {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (m *MemoryStore) DeleteSki(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.skis[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.skis, id)
	return nil
}

// ---------- Personen ----------

func (m *MemoryStore) CreatePerson(p domain.Person) domain.Person {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastPersonID++
	p.ID = m.lastPersonID
	m.persons[p.ID] = copyPerson(p)
	return copyPerson(p)
}

func (m *MemoryStore) UpdatePerson(p domain.Person) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.persons[p.ID]; !ok {
		return domain.ErrNotFound
	}
	m.persons[p.ID] = copyPerson(p)
	return nil
}

func (m *MemoryStore) Person(id int) (domain.Person, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.persons[id]
	if !ok {
		return domain.Person{}, domain.ErrNotFound
	}
	return copyPerson(p), nil
}

// ListPersons liefert alle Personen, sortiert nach ID.
func (m *MemoryStore) ListPersons() []domain.Person {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.Person, 0, len(m.persons))
	for _, p := range m.persons {
		out = append(out, copyPerson(p))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (m *MemoryStore) DeletePerson(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.persons[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.persons, id)
	return nil
}

// ---------- Ausleihen ----------

func (m *MemoryStore) CreateRental(r domain.Rental) domain.Rental {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastRentalID++
	r.ID = m.lastRentalID
	m.rentals[r.ID] = copyRental(r)
	return copyRental(r)
}

func (m *MemoryStore) UpdateRental(r domain.Rental) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rentals[r.ID]; !ok {
		return domain.ErrNotFound
	}
	m.rentals[r.ID] = copyRental(r)
	return nil
}

func (m *MemoryStore) Rental(id int) (domain.Rental, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.rentals[id]
	if !ok {
		return domain.Rental{}, domain.ErrNotFound
	}
	return copyRental(r), nil
}

// DeleteRental entfernt eine Buchung (Stornieren einer Reservierung, F-4.5).
func (m *MemoryStore) DeleteRental(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rentals[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.rentals, id)
	return nil
}

// ListRentals liefert alle Ausleihen, sortiert nach ID.
func (m *MemoryStore) ListRentals() []domain.Rental {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.Rental, 0, len(m.rentals))
	for _, r := range m.rentals {
		out = append(out, copyRental(r))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ---------- Kopien ----------

// copyPerson kopiert auch die Bild-Bytes, weil Slices sonst geteilt würden.
func copyPerson(p domain.Person) domain.Person {
	if p.IDImage != nil {
		img := *p.IDImage
		img.Data = bytes.Clone(img.Data)
		p.IDImage = &img
	}
	return p
}

func copyRental(r domain.Rental) domain.Rental {
	if r.ReturnedAt != nil {
		t := *r.ReturnedAt
		r.ReturnedAt = &t
	}
	return r
}
