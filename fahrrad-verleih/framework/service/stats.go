package service

import (
	"sort"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/plugin"
)

// dashboardListLimit begrenzt die Listen auf dem Dashboard (F-1.3).
const dashboardListLimit = 5

// CategoryStats ist eine Zeile „Bestand nach Typ“ (F-1.2).
type CategoryStats struct {
	Category    domain.Category
	Rule        string
	Total       int
	Available   int
	Rented      int
	Maintenance int
}

// Stats sind die Kennzahlen einer Verleihart (F-1, AK-1).
type Stats struct {
	Kind        plugin.Kind
	Available   int
	Rented      int
	Overdue     int
	Reserved    int // anstehende Reservierungen (Buchungen, nicht Objekte)
	Maintenance int
	Total       int
	Persons     int
	Categories  []CategoryStats
	Active      []BookingView // überfällige zuerst
	Upcoming    []BookingView // nächster Beginn zuerst
	Conflicts   []BookingView // alle Konflikte (F-1.5)
}

// Stats berechnet die Kennzahlen einer Verleihart aus den aktuellen Daten.
func (s *Service) Stats(kind string) (Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, err := s.reg.Get(kind)
	if err != nil {
		return Stats{}, err
	}
	snap := s.load(s.now())
	st := Stats{Kind: p.Kind(), Persons: len(snap.persons)}

	cats := p.Categories()
	st.Categories = make([]CategoryStats, len(cats))
	byCategory := map[string]*CategoryStats{}
	for i, c := range cats {
		st.Categories[i] = CategoryStats{Category: c, Rule: p.Pricing().Describe(c.Key)}
		byCategory[c.Key] = &st.Categories[i]
	}
	for _, it := range s.repo.ListItems() {
		if it.Kind != kind {
			continue
		}
		st.Total++
		cs := byCategory[it.Category]
		if cs == nil {
			cs = &CategoryStats{} // unbekannte Kategorie zählt nur im Gesamtbestand
		}
		cs.Total++
		switch it.StatusAt(snap.byItem[it.ID], snap.now) {
		case domain.StatusAvailable:
			st.Available++
			cs.Available++
		case domain.StatusRented:
			st.Rented++
			cs.Rented++
		case domain.StatusMaintenance:
			st.Maintenance++
			cs.Maintenance++
		}
	}

	all := snap.views(func(b domain.Booking, _ domain.Situation) bool { return snap.items[b.ItemID].Kind == kind })
	active, upcoming, _ := split(all)
	for _, v := range active {
		if v.Overdue {
			st.Overdue++
		}
	}
	for _, v := range all {
		if v.State == domain.StateReserved {
			st.Reserved++
		}
		if v.Conflict {
			st.Conflicts = append(st.Conflicts, v)
		}
	}
	sort.SliceStable(active, func(i, j int) bool {
		if active[i].Overdue != active[j].Overdue {
			return active[i].Overdue
		}
		return active[i].Booking.PlannedEnd.Before(active[j].Booking.PlannedEnd)
	})
	st.Active = limit(active)
	st.Upcoming = limit(upcoming)
	st.Conflicts = soonestFirst(st.Conflicts)
	return st, nil
}

func limit(v []BookingView) []BookingView {
	if len(v) > dashboardListLimit {
		return v[:dashboardListLimit]
	}
	return v
}
