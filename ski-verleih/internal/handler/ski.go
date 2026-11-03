package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"skiverleih/internal/domain"
	"skiverleih/internal/service"
)

// skiForm hält die Formularwerte als Text, damit ungültige Eingaben
// beim erneuten Anzeigen erhalten bleiben (F-2.2).
type skiForm struct {
	ID           int // 0 = neuer Ski
	Brand        string
	Model        string
	Length       string
	Type         string
	Condition    string
	Status       string
	Note         string
	StatusLocked bool // ausgeliehen oder reserviert: Status nicht wählbar (F-2.4)
	Errors       domain.ValidationErrors
}

func skiFormFrom(sk domain.Ski, locked bool) skiForm {
	return skiForm{
		ID: sk.ID, Brand: sk.Brand, Model: sk.Model, Length: strconv.Itoa(sk.LengthCM),
		Type: string(sk.Type), Condition: string(sk.Condition), Status: string(sk.Status), Note: sk.Note,
		StatusLocked: locked,
	}
}

func parseSkiForm(r *http.Request) (skiForm, service.SkiInput) {
	f := skiForm{
		Brand:     r.FormValue("brand"),
		Model:     r.FormValue("model"),
		Length:    strings.TrimSpace(r.FormValue("length")),
		Type:      r.FormValue("type"),
		Condition: r.FormValue("condition"),
		Status:    r.FormValue("status"),
		Note:      r.FormValue("note"),
	}
	length, _ := strconv.Atoi(f.Length) // ungültig → 0 → Validierungsfehler
	in := service.SkiInput{
		Brand: f.Brand, Model: f.Model, LengthCM: length,
		Type: domain.SkiType(f.Type), Condition: domain.Condition(f.Condition),
		Status: domain.SkiStatus(f.Status), Note: f.Note,
	}
	return f, in
}

// F-2.1
func (s *Server) handleSkiList(w http.ResponseWriter, r *http.Request) {
	filter := domain.SkiStatus(r.URL.Query().Get("status"))
	if !filter.Valid() {
		filter = "" // alle
	}
	tabs := []tab{{Label: "Alle", URL: "/ski", Active: filter == ""}}
	for _, st := range domain.SkiStatuses {
		tabs = append(tabs, tab{Label: st.Label(), URL: "/ski?status=" + string(st), Active: filter == st})
	}
	s.render(w, r, http.StatusOK, "ski_list.html", page{
		Title:    "Ski",
		Subtitle: "Bestand verwalten",
		Active:   "ski",
		Content: struct {
			Skis     []service.SkiView
			Tabs     []tab
			Filtered bool
		}{s.svc.Skis(filter), tabs, filter != ""},
	})
}

// F-2.2
func (s *Server) handleSkiNew(w http.ResponseWriter, r *http.Request) {
	s.renderSkiForm(w, r, http.StatusOK, skiForm{
		Type: string(domain.SkiTypeAllround), Condition: string(domain.ConditionGood),
	}, "")
}

func (s *Server) handleSkiCreate(w http.ResponseWriter, r *http.Request) {
	form, in := parseSkiForm(r)
	sk, err := s.svc.CreateSki(in)
	if err != nil {
		s.skiFormFailed(w, r, form, err)
		return
	}
	redirect(w, r, fmt.Sprintf("/ski/%d", sk.ID), "ski_created")
}

// F-2.3
func (s *Server) handleSkiDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.handleNotFound(w, r)
		return
	}
	d, err := s.svc.SkiDetail(id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "ski_detail.html", page{
		Title:    d.View.Ski.Brand + " " + d.View.Ski.Model,
		Subtitle: fmt.Sprintf("Ski #%d · %d cm · %s", d.View.Ski.ID, d.View.Ski.LengthCM, d.View.Ski.Type.Label()),
		Active:   "ski",
		Content:  d,
	})
}

// F-2.4
func (s *Server) handleSkiEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.handleNotFound(w, r)
		return
	}
	d, err := s.svc.SkiDetail(id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.renderSkiForm(w, r, http.StatusOK, skiFormFrom(d.View.Ski, d.View.InUse()), "")
}

func (s *Server) handleSkiUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.handleNotFound(w, r)
		return
	}
	form, in := parseSkiForm(r)
	form.ID = id
	if _, err := s.svc.UpdateSki(id, in); err != nil {
		if d, derr := s.svc.SkiDetail(id); derr == nil {
			form.StatusLocked = d.View.InUse()
			if form.StatusLocked {
				form.Status = string(d.View.Ski.Status) // Grundstatus bleibt unverändert
			}
		}
		s.skiFormFailed(w, r, form, err)
		return
	}
	redirect(w, r, fmt.Sprintf("/ski/%d", id), "ski_updated")
}

// skiFormFailed zeigt das Formular mit Fehlern erneut (422) oder die Fehlerseite.
func (s *Server) skiFormFailed(w http.ResponseWriter, r *http.Request, form skiForm, err error) {
	fields, msg, isFormError := formProblem(err)
	if !isFormError {
		s.fail(w, r, err)
		return
	}
	form.Errors = fields
	s.renderSkiForm(w, r, http.StatusUnprocessableEntity, form, msg)
}

func (s *Server) renderSkiForm(w http.ResponseWriter, r *http.Request, status int, form skiForm, errMsg string) {
	title := "Neuer Ski"
	if form.ID != 0 {
		title = "Ski bearbeiten"
	}
	s.render(w, r, status, "ski_form.html", page{
		Title:    title,
		Subtitle: "Pflichtfelder sind mit * markiert",
		Active:   "ski",
		Flash:    errorFlash(errMsg),
		Content:  form,
	})
}

// F-2.5, F-6.3
func (s *Server) handleSkiConfirmDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.handleNotFound(w, r)
		return
	}
	d, err := s.svc.SkiDetail(id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.renderConfirmSki(w, r, http.StatusOK, d)
}

func (s *Server) handleSkiDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.handleNotFound(w, r)
		return
	}
	if err := s.svc.DeleteSki(id); err != nil {
		if d, derr := s.svc.SkiDetail(id); derr == nil && statusFor(err) == http.StatusConflict {
			s.renderConfirmSki(w, r, http.StatusConflict, d)
			return
		}
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/ski", "ski_deleted")
}

func (s *Server) renderConfirmSki(w http.ResponseWriter, r *http.Request, status int, d service.SkiDetail) {
	c := confirmData{
		What:   "den Ski „" + d.View.Ski.Name() + "“",
		Action: fmt.Sprintf("/ski/%d/loeschen", d.View.Ski.ID),
		Cancel: fmt.Sprintf("/ski/%d", d.View.Ski.ID),
	}
	if d.HasHistory() {
		c.Blocked = "Ski hat Ausleih-Historie und kann nicht gelöscht werden. Setze ihn stattdessen auf Wartung."
	}
	s.renderConfirm(w, r, status, "Ski löschen", "ski", c)
}

// confirmData füllt die gemeinsame Lösch-Bestätigungsseite (F-6.3).
type confirmData struct {
	What    string // z. B. „den Ski „…““
	Action  string // POST-Ziel
	Cancel  string // Link „Abbrechen“
	Blocked string // gesetzt, wenn Löschen nicht erlaubt ist
}

func (s *Server) renderConfirm(w http.ResponseWriter, r *http.Request, status int, title, active string, c confirmData) {
	s.render(w, r, status, "confirm_delete.html", page{Title: title, Active: active, Content: c})
}
