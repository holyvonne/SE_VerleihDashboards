// Package service enthält die Anwendungsfälle des Verleih-Frameworks.
// Er arbeitet mit jeder Verleihart, die in der Registry steht, und kennt nur die
// Schnittstellen aus dem Paket plugin (F-8.1).
package service

import (
	"sort"
	"strconv"
	"sync"
	"time"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/plugin"
)

// Service bündelt alle Anwendungsfälle. Schreibende Fälle laufen unter der Schreibsperre,
// damit Prüfen und Speichern atomar sind (AK-12). Methoden mit kleinem Anfangsbuchstaben
// erwarten, dass die Sperre schon gehalten wird.
type Service struct {
	mu   sync.RWMutex
	repo Repository
	reg  *plugin.Registry
	now  func() time.Time
}

// New erstellt den Service. now ist die Uhr (NF-12); nil bedeutet time.Now.
func New(repo Repository, reg *plugin.Registry, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, reg: reg, now: now}
}

// Now liefert die Uhrzeit des Service, damit Handler dieselbe Uhr verwenden.
func (s *Service) Now() time.Time { return s.now() }

// Plugin liefert das Plugin einer Verleihart.
func (s *Service) Plugin(kind string) (plugin.Plugin, error) { return s.reg.Get(kind) }

// snapshot ist ein konsistenter Stand aller Daten für eine Operation.
type snapshot struct {
	s        *Service
	now      time.Time
	items    map[int]domain.Item
	persons  map[int]domain.Person
	bookings []domain.Booking
	byItem   map[int][]domain.Booking
}

func (s *Service) load(now time.Time) *snapshot {
	snap := &snapshot{
		s: s, now: now,
		items:   map[int]domain.Item{},
		persons: map[int]domain.Person{},
		byItem:  map[int][]domain.Booking{},
	}
	for _, it := range s.repo.ListItems() {
		snap.items[it.ID] = it
	}
	for _, p := range s.repo.ListPersons() {
		snap.persons[p.ID] = p
	}
	snap.bookings = s.repo.ListBookings()
	for _, b := range snap.bookings {
		snap.byItem[b.ItemID] = append(snap.byItem[b.ItemID], b)
	}
	return snap
}

func (snap *snapshot) situation(b domain.Booking) domain.Situation {
	return domain.Evaluate(b, snap.byItem[b.ItemID], snap.now)
}

// ---------- Ansichten ----------

// AttrView ist ein Merkmal mit Beschreibung und Anzeigewert.
type AttrView struct {
	Field   domain.Field
	Value   string
	Display string
}

// ItemView ist ein Objekt mit allem, was zur Anzeige gebraucht wird.
type ItemView struct {
	Item            domain.Item
	Kind            plugin.Kind
	Name            string
	CategoryLabel   string
	Rule            string // Preisregel der Kategorie
	Status          domain.ItemStatus
	NextReservation *domain.Booking
	OpenBookings    int
	Attributes      []AttrView
}

// InUse meldet offene Buchungen; dann ist der Statuswechsel gesperrt (F-2.4).
func (v ItemView) InUse() bool { return v.OpenBookings > 0 }

// Bookable meldet, ob das Objekt gebucht werden kann (nicht in Wartung).
func (v ItemView) Bookable() bool { return !v.Item.InMaintenance() }

func (snap *snapshot) itemView(it domain.Item) ItemView {
	v := ItemView{Item: it, Name: "#" + strconv.Itoa(it.ID), CategoryLabel: it.Category, Status: it.StatusAt(snap.byItem[it.ID], snap.now)}
	if p, err := snap.s.reg.Get(it.Kind); err == nil {
		v.Kind = p.Kind()
		v.Name = p.DisplayName(it)
		v.Rule = p.Pricing().Describe(it.Category)
		if c, ok := plugin.CategoryOf(p, it.Category); ok {
			v.CategoryLabel = c.Label
		}
		for _, f := range p.Fields() {
			val := it.Attr(f.Key)
			v.Attributes = append(v.Attributes, AttrView{Field: f, Value: val, Display: f.Display(val)})
		}
	}
	for _, b := range snap.byItem[it.ID] {
		sit := snap.situation(b)
		if sit.Open() {
			v.OpenBookings++
		}
		if sit.State == domain.StateReserved && (v.NextReservation == nil || b.Start.Before(v.NextReservation.Start)) {
			next := b
			v.NextReservation = &next
		}
	}
	return v
}

// BookingView ist eine Buchung mit Objekt, Person und abgeleitetem Zustand.
type BookingView struct {
	Booking domain.Booking
	Item    ItemView
	Person  domain.Person
	domain.Situation
	Duration time.Duration
	Price    domain.Price
}

// CanReturn: nur aktive Ausleihen werden zurückgegeben.
func (v BookingView) CanReturn() bool { return v.State == domain.StateActive }

// CanCancel und CanReassign: nur noch nicht ausgegebene Buchungen (4.4).
func (v BookingView) CanCancel() bool   { return v.NotHandedOut() }
func (v BookingView) CanReassign() bool { return v.NotHandedOut() }

func (snap *snapshot) bookingView(b domain.Booking) BookingView {
	return BookingView{
		Booking:   b,
		Item:      snap.itemView(snap.items[b.ItemID]),
		Person:    snap.persons[b.PersonID],
		Situation: snap.situation(b),
		Duration:  b.Duration(),
		Price:     b.ChargedPrice(),
	}
}

// views baut Ansichten für alle Buchungen, die keep erfüllen, neuester Beginn zuerst.
func (snap *snapshot) views(keep func(domain.Booking, domain.Situation) bool) []BookingView {
	var out []BookingView
	for _, b := range snap.bookings {
		if keep(b, snap.situation(b)) {
			out = append(out, snap.bookingView(b))
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Booking.Start.After(out[j].Booking.Start) })
	return out
}

// soonestFirst sortiert Reservierungen mit dem nächsten Beginn zuerst (F-6).
func soonestFirst(v []BookingView) []BookingView {
	sort.SliceStable(v, func(i, j int) bool { return v[i].Booking.Start.Before(v[j].Booking.Start) })
	return v
}

// split teilt Buchungen in ausgegebene (aktiv), noch nicht ausgegebene und abgeschlossene.
func split(all []BookingView) (active, upcoming, closed []BookingView) {
	for _, v := range all {
		switch {
		case v.State == domain.StateActive:
			active = append(active, v)
		case v.NotHandedOut():
			upcoming = append(upcoming, v)
		default:
			closed = append(closed, v)
		}
	}
	return active, soonestFirst(upcoming), closed
}
