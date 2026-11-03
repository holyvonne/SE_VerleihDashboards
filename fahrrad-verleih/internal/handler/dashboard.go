package handler

import (
	"net/http"

	"fahrradverleih/framework/service"
)

// F-1
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	st, err := s.svc.Stats(kind)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "dashboard.html", page{
		Title:    "Dashboard",
		Subtitle: "Fahrräder, Ausleihen und Reservierungen auf einen Blick",
		Active:   "dashboard",
		Content: struct {
			service.Stats
			Rates map[string]string // kurzer Stundensatz je Typ, z. B. „5,00 €/Std“
		}{st, s.rateLabels()},
	})
}
