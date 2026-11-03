package service

import (
	"sort"
	"time"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/plugin"
)

// BookingFilter wählt die Buchungen der Übersicht (F-6).
type BookingFilter string

const (
	FilterActive   BookingFilter = "aktiv"
	FilterReserved BookingFilter = "reserviert" // inklusive wartender Buchungen
	FilterClosed   BookingFilter = "abgeschlossen"
	FilterAll      BookingFilter = "alle"
)

// ParseBookingFilter liest den Filter aus der URL; Standard ist aktiv.
func ParseBookingFilter(s string) BookingFilter {
	switch f := BookingFilter(s); f {
	case FilterReserved, FilterClosed, FilterAll:
		return f
	default:
		return FilterActive
	}
}

func (f BookingFilter) keep(sit domain.Situation) bool {
	switch f {
	case FilterActive:
		return sit.State == domain.StateActive
	case FilterReserved:
		return sit.NotHandedOut()
	case FilterClosed:
		return sit.State == domain.StateClosed
	default:
		return true
	}
}

// Bookings liefert die Buchungen eines Filters (F-6, AK-17).
func (s *Service) Bookings(f BookingFilter) []BookingView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := s.load(s.now())
	out := snap.views(func(_ domain.Booking, sit domain.Situation) bool { return f.keep(sit) })
	if f == FilterReserved {
		return soonestFirst(out)
	}
	return out
}

// Booking liefert eine einzelne Buchung als Ansicht.
func (s *Service) Booking(id int) (BookingView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := s.load(s.now())
	b, err := s.repo.Booking(id)
	if err != nil {
		return BookingView{}, err
	}
	return snap.bookingView(b), nil
}

// BookingInput ist eine gewünschte Buchung.
type BookingInput struct {
	ItemID   int
	PersonID int
	Start    time.Time
	End      time.Time
}

func (in BookingInput) Period() domain.Period { return domain.Period{Start: in.Start, End: in.End} }

// Quote ist die geprüfte Vorschau einer Buchung (F-5.2).
type Quote struct {
	Item          ItemView
	Person        domain.Person
	Period        domain.Period
	Price         domain.Price
	IsReservation bool
}

func (q Quote) Duration() time.Duration { return q.Period.Duration() }

// Quote prüft eine Buchung und berechnet den Preis, ohne etwas zu speichern (F-5.2, AK-8).
func (s *Service) Quote(in BookingInput) (Quote, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.load(s.now()).quote(in)
}

// Book prüft und speichert eine Buchung unter der Schreibsperre (F-5.3, AK-12).
func (s *Service) Book(in BookingInput) (domain.Booking, error) {
	return s.BookAsOf(in, s.now())
}

// BookAsOf bucht so, als wäre jetzt at. Gedacht für Demo-Daten und Datenübernahmen;
// es gelten dieselben Regeln wie bei Book.
func (s *Service) BookAsOf(in BookingInput, at time.Time) (domain.Booking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	q, err := s.load(at).quote(in)
	if err != nil {
		return domain.Booking{}, err
	}
	return s.repo.CreateBooking(domain.NewBooking(in.ItemID, in.PersonID, q.Period, at, q.Price)), nil
}

// quote prüft in der Reihenfolge aus Spezifikation 3.4.
func (snap *snapshot) quote(in BookingInput) (Quote, error) {
	errs := domain.ValidationErrors{}
	it, itemOK := snap.items[in.ItemID]
	person, personOK := snap.persons[in.PersonID]
	if !itemOK {
		errs.Add("item_id", "Bitte eine gültige Auswahl treffen")
	}
	if !personOK {
		errs.Add("person_id", "Bitte eine Person wählen")
	}
	var p plugin.Plugin
	var limits domain.BookingLimits
	if itemOK {
		var err error
		if p, err = snap.s.reg.Get(it.Kind); err != nil {
			return Quote{}, err
		}
		limits = p.Limits()
	}
	errs.Merge(domain.ValidatePeriod(in.Period(), snap.now, limits))
	if len(errs) > 0 {
		return Quote{}, errs
	}
	if it.InMaintenance() {
		return Quote{}, domain.ErrItemInMaintenance
	}
	if !snap.free(it, in.Period(), 0) {
		return Quote{}, domain.ErrItemBooked
	}
	return Quote{
		Item:          snap.itemView(it),
		Person:        person,
		Period:        in.Period(),
		Price:         p.Pricing().Quote(it, in.Period()),
		IsReservation: in.Start.After(snap.now),
	}, nil
}

// Return schließt eine aktive Ausleihe ab (F-5.4, AK-13).
func (s *Service) Return(id int) (domain.Booking, error) { return s.ReturnAsOf(id, s.now()) }

// ReturnAsOf gibt so zurück, als wäre jetzt at (Demo-Daten).
func (s *Service) ReturnAsOf(id int, at time.Time) (domain.Booking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := s.load(at)
	b, err := s.repo.Booking(id)
	if err != nil {
		return domain.Booking{}, err
	}
	switch snap.situation(b).State {
	case domain.StateClosed:
		return domain.Booking{}, domain.ErrAlreadyReturned
	case domain.StateReserved, domain.StateWaiting:
		return domain.Booking{}, domain.ErrNotStarted
	}
	it := snap.items[b.ItemID]
	p, err := s.reg.Get(it.Kind)
	if err != nil {
		return domain.Booking{}, err
	}
	quote := func(period domain.Period) domain.Price { return p.Pricing().Quote(it, period) }
	if err := b.Return(at, quote); err != nil {
		return domain.Booking{}, err
	}
	return b, s.repo.UpdateBooking(b)
}

// Cancel löscht eine noch nicht ausgegebene Buchung (F-5.5, AK-14).
func (s *Service) Cancel(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := s.load(s.now())
	b, err := s.repo.Booking(id)
	if err != nil {
		return err
	}
	if !snap.situation(b).NotHandedOut() {
		return domain.ErrNotCancellable
	}
	return s.repo.DeleteBooking(id)
}

// Alternatives liefert alle anderen Objekte derselben Verleihart, auf die eine Buchung
// umgebucht werden kann, mit neuem Preis (F-5.6). Gleiche Kategorie steht vorn.
func (s *Service) Alternatives(id int) (BookingView, []Offer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := s.load(s.now())
	b, err := s.repo.Booking(id)
	if err != nil {
		return BookingView{}, nil, err
	}
	view := snap.bookingView(b)
	if !view.NotHandedOut() {
		return view, nil, domain.ErrNotReassignable
	}
	current := snap.items[b.ItemID]
	p, err := s.reg.Get(current.Kind)
	if err != nil {
		return view, nil, err
	}
	var offers []Offer
	for _, it := range s.repo.ListItems() {
		if it.ID == current.ID || it.Kind != current.Kind || !snap.free(it, snap.remaining(b), b.ID) {
			continue
		}
		offers = append(offers, Offer{View: snap.itemView(it), Price: p.Pricing().Quote(it, b.Planned())})
	}
	sort.SliceStable(offers, func(i, j int) bool {
		return offers[i].View.Item.Category == current.Category && offers[j].View.Item.Category != current.Category
	})
	return view, offers, nil
}

// Reassign verlegt eine noch nicht ausgegebene Buchung auf ein anderes, freies Objekt (F-5.6, AK-25).
// Zeitraum und Person bleiben; der Preis wird mit der Strategie des neuen Objekts berechnet.
func (s *Service) Reassign(id, itemID int) (domain.Booking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := s.load(s.now())
	b, err := s.repo.Booking(id)
	if err != nil {
		return domain.Booking{}, err
	}
	if !snap.situation(b).NotHandedOut() {
		return domain.Booking{}, domain.ErrNotReassignable
	}
	target, ok := snap.items[itemID]
	if !ok {
		return domain.Booking{}, domain.ErrNotFound
	}
	if target.ID == b.ItemID || target.Kind != snap.items[b.ItemID].Kind {
		return domain.Booking{}, domain.ErrNotReassignable
	}
	if target.InMaintenance() {
		return domain.Booking{}, domain.ErrItemInMaintenance
	}
	if !snap.free(target, snap.remaining(b), b.ID) {
		return domain.Booking{}, domain.ErrItemBooked
	}
	p, err := s.reg.Get(target.Kind)
	if err != nil {
		return domain.Booking{}, err
	}
	b.ItemID = target.ID
	b.Price = p.Pricing().Quote(target, b.Planned())
	return b, s.repo.UpdateBooking(b)
}

// remaining ist der Teil einer Buchung, der noch bevorsteht. Bei einer wartenden Buchung
// liegt der Beginn schon in der Vergangenheit; frühere, abgeschlossene Ausleihen des
// Zielobjekts sind dann kein Hindernis.
func (snap *snapshot) remaining(b domain.Booking) domain.Period {
	p := b.Planned()
	if p.Start.Before(snap.now) {
		p.Start = snap.now
	}
	if !p.End.After(p.Start) {
		p.End = p.Start.Add(time.Minute)
	}
	return p
}
