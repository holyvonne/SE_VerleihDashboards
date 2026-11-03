package handler

import "net/http"

// handleDashboard zeigt Kennzahlen, aktive Ausleihen und Schnellzugriffe (F-1).
// Die Zahlen werden bei jedem Aufruf aus den aktuellen Daten berechnet.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "dashboard.html", page{
		Title:    "Dashboard",
		Subtitle: "Überblick über Bestand und Ausleihen",
		Active:   "dashboard",
		Content:  s.svc.DashboardStats(),
	})
}
