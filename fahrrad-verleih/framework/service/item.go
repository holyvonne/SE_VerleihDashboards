package service

import (
	"strings"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/plugin"
)

// ItemInput sind die Formularwerte eines Objekts.
type ItemInput struct {
	Kind       string // nur beim Anlegen; beim Bearbeiten bleibt die Verleihart
	Category   string
	Attributes map[string]string
	Status     domain.ItemStatus // leer = unverändert bzw. verfügbar
	Note       string
}

// ItemFilter schränkt die Objektliste ein (F-2.1). Leere Felder filtern nicht.
type ItemFilter struct {
	Kind     string
	Category string
	Status   string // verfuegbar, ausgeliehen, wartung oder reserviert (= hat anstehende Reservierung)
}

// FilterReservedItems ist der Statusfilter für Objekte mit anstehender Reservierung.
const FilterReservedItems = "reserviert"

func (f ItemFilter) match(v ItemView) bool {
	switch {
	case f.Kind != "" && v.Item.Kind != f.Kind:
		return false
	case f.Category != "" && v.Item.Category != f.Category:
		return false
	case f.Status == FilterReservedItems:
		return v.NextReservation != nil
	case f.Status != "":
		return string(v.Status) == f.Status
	}
	return true
}

// Items liefert alle Objekte, die zum Filter passen, nach ID sortiert.
func (s *Service) Items(f ItemFilter) []ItemView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := s.load(s.now())
	var out []ItemView
	for _, it := range s.repo.ListItems() {
		if v := snap.itemView(it); f.match(v) {
			out = append(out, v)
		}
	}
	return out
}

// BookableItems liefert alle Objekte einer Verleihart, die nicht in Wartung sind.
func (s *Service) BookableItems(kind string) []ItemView {
	var out []ItemView
	for _, v := range s.Items(ItemFilter{Kind: kind}) {
		if v.Bookable() {
			out = append(out, v)
		}
	}
	return out
}

// Item liefert ein einzelnes Objekt.
func (s *Service) Item(id int) (ItemView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := s.load(s.now())
	it, ok := snap.items[id]
	if !ok {
		return ItemView{}, domain.ErrNotFound
	}
	return snap.itemView(it), nil
}

// ItemDetail ist die Detailansicht eines Objekts (F-2.3).
type ItemDetail struct {
	View     ItemView
	Current  []BookingView // aktive Ausleihe (höchstens eine, I-2a)
	Upcoming []BookingView // reserviert oder wartend
	History  []BookingView
}

func (d ItemDetail) HasHistory() bool {
	return len(d.Current)+len(d.Upcoming)+len(d.History) > 0
}

func (s *Service) ItemDetail(id int) (ItemDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := s.load(s.now())
	it, ok := snap.items[id]
	if !ok {
		return ItemDetail{}, domain.ErrNotFound
	}
	active, upcoming, closed := split(snap.views(func(b domain.Booking, _ domain.Situation) bool { return b.ItemID == id }))
	return ItemDetail{View: snap.itemView(it), Current: active, Upcoming: upcoming, History: closed}, nil
}

// CreateItem legt ein Objekt an (F-2.2, I-7, AK-23).
func (s *Service) CreateItem(in ItemInput) (domain.Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.reg.Get(in.Kind)
	if err != nil {
		return domain.Item{}, err
	}
	it := domain.Item{Kind: in.Kind, Category: in.Category, Attributes: in.Attributes, Status: in.Status, Note: in.Note, CreatedAt: s.now()}
	if it.Status == "" {
		it.Status = domain.StatusAvailable
	}
	if err := validateItem(p, &it); err != nil {
		return domain.Item{}, err
	}
	return s.repo.CreateItem(it), nil
}

// UpdateItem speichert Kategorie, Merkmale, Notiz und Grundstatus.
// Der Grundstatus ist gesperrt, solange offene Buchungen bestehen (F-2.4, AK-4).
func (s *Service) UpdateItem(id int, in ItemInput) (domain.Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := s.load(s.now())
	old, ok := snap.items[id]
	if !ok {
		return domain.Item{}, domain.ErrNotFound
	}
	p, err := s.reg.Get(old.Kind)
	if err != nil {
		return domain.Item{}, err
	}
	it := domain.Item{ID: id, Kind: old.Kind, Category: in.Category, Attributes: in.Attributes, Status: in.Status, Note: in.Note, CreatedAt: old.CreatedAt}
	if it.Status == "" {
		it.Status = old.Status
	}
	if err := validateItem(p, &it); err != nil {
		return domain.Item{}, err
	}
	if it.Status != old.Status && snap.itemView(old).InUse() {
		return domain.Item{}, domain.ErrItemInUse
	}
	if err := s.repo.UpdateItem(it); err != nil {
		return domain.Item{}, err
	}
	return it, nil
}

// DeleteItem löscht ein Objekt ohne Buchungen (F-2.5).
func (s *Service) DeleteItem(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.repo.Item(id); err != nil {
		return err
	}
	for _, b := range s.repo.ListBookings() {
		if b.ItemID == id {
			return domain.ErrHasHistory
		}
	}
	return s.repo.DeleteItem(id)
}

// validateItem prüft in drei Stufen (Spezifikation 3.2): allgemeine Regeln,
// Merkmale nach ihrer Beschreibung, Regeln des Plugins über mehrere Felder.
func validateItem(p plugin.Plugin, it *domain.Item) error {
	it.Category = strings.TrimSpace(it.Category)
	errs := it.ValidateBase()
	if _, ok := plugin.CategoryOf(p, it.Category); !ok {
		errs.Add("category", "Bitte eine gültige Kategorie wählen")
	}
	attrs, attrErrs := domain.ValidateAttributes(p.Fields(), it.Attributes)
	it.Attributes = attrs
	errs.Merge(attrErrs)
	errs.Merge(p.Validate(*it))
	return errs.ErrOrNil()
}

// Offer ist ein freies Objekt mit seinem Preis für einen Zeitraum (F-4, F-5.6).
type Offer struct {
	View  ItemView
	Price domain.Price
}

// Available liefert alle Objekte einer Verleihart (optional einer Kategorie), die im Zeitraum frei sind (F-4, AK-6).
// Der Zeitraum wird nach denselben Regeln geprüft wie eine Buchung.
func (s *Service) Available(kind, category string, p domain.Period) ([]Offer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pl, err := s.reg.Get(kind)
	if err != nil {
		return nil, err
	}
	snap := s.load(s.now())
	if errs := domain.ValidatePeriod(p, snap.now, pl.Limits()); len(errs) > 0 {
		return nil, errs
	}
	var out []Offer
	for _, it := range s.repo.ListItems() {
		if it.Kind != kind || (category != "" && it.Category != category) || !snap.free(it, p, 0) {
			continue
		}
		out = append(out, Offer{View: snap.itemView(it), Price: pl.Pricing().Quote(it, p)})
	}
	return out, nil
}

// free: nicht in Wartung und keine Belegung überschneidet p (Spezifikation 4.5).
func (snap *snapshot) free(it domain.Item, p domain.Period, exceptBooking int) bool {
	if it.InMaintenance() {
		return false
	}
	_, conflict := domain.FindConflict(p, snap.byItem[it.ID], snap.now, exceptBooking)
	return !conflict
}
