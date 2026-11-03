// Package service enthält die Anwendungsfälle (Buchen, Rückgabe, Stornieren, Kennzahlen).
package service

import "skiverleih/internal/domain"

// Länge der beiden Listen auf dem Dashboard (F-1.3).
const dashboardListLimit = 5

// DashboardStats sind die Kennzahlen des Dashboards (F-1).
type DashboardStats struct {
	Available            int // jetzt buchbar und nicht ausgeliehen
	Rented               int // Ski mit aktiver Ausleihe
	Overdue              int // davon überfällig
	Reserved             int // zukünftige Reservierungen (nur Zustand reserviert)
	Maintenance          int // zählt nur zu Total, wird als Hinweis angezeigt
	Total                int
	Persons              int
	ActiveRentals        []RentalView // neuester Beginn zuerst
	UpcomingReservations []RentalView // nächster Beginn zuerst, inklusive wartender Buchungen
	Conflicts            []RentalView // von überfälligen Ausleihen blockierte Buchungen (F-1.4)
}

// DashboardStats berechnet die Kennzahlen bei jedem Aufruf aus den aktuellen Daten
// und der aktuellen Uhrzeit (F-1, AK-1, AK-8, AK-15).
func (s *Service) DashboardStats() DashboardStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()
	rentals := s.repo.ListRentals()

	var st DashboardStats
	for _, sk := range s.repo.ListSkis() {
		st.Total++
		switch sk.StatusAt(rentals, now) {
		case domain.SkiAvailable:
			st.Available++
		case domain.SkiRented:
			st.Rented++
		case domain.SkiMaintenance:
			st.Maintenance++
		}
	}
	st.Persons = len(s.repo.ListPersons())

	active, reserved, _ := splitByState(s.views(rentals))
	for _, v := range active {
		if v.Overdue {
			st.Overdue++
		}
	}
	for _, v := range reserved {
		if v.State == domain.RentalReserved {
			st.Reserved++
		}
		if v.Conflict {
			st.Conflicts = append(st.Conflicts, v)
		}
	}
	st.ActiveRentals = limit(active, dashboardListLimit)
	st.UpcomingReservations = limit(reserved, dashboardListLimit)
	return st
}

func limit(v []RentalView, n int) []RentalView {
	if len(v) > n {
		return v[:n]
	}
	return v
}
