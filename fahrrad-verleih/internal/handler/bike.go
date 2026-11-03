package handler

import (
	"fmt"
	"net/http"
	"net/url"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/service"
)

// bikeForm hält die Formularwerte; die Merkmalsfelder kommen aus dem Plugin (F-2.2).
type bikeForm struct {
	ID           int // 0 = neues Fahrrad
	Category     string
	Attrs        map[string]string
	Status       string
	Note         string
	StatusLocked bool // offene Buchungen: Status nicht wählbar (F-2.4)
	Errors       domain.ValidationErrors
	Categories   []domain.Category
	Fields       []domain.Field
	Rates        map[string]string // Stundensatz je Typ zur Anzeige
}

func bikeFormFrom(v service.ItemView) bikeForm {
	return bikeForm{
		ID: v.Item.ID, Category: v.Item.Category, Attrs: v.Item.Attributes,
		Status: string(v.Item.Status), Note: v.Item.Note, StatusLocked: v.InUse(),
	}
}

func (s *Server) parseBikeForm(r *http.Request) (bikeForm, service.ItemInput) {
	f := bikeForm{
		Category: r.FormValue("category"),
		Attrs:    map[string]string{},
		Status:   r.FormValue("status"),
		Note:     r.FormValue("note"),
	}
	for _, field := range s.bikes.Fields() {
		f.Attrs[field.Key] = r.FormValue(field.FormKey())
	}
	return f, service.ItemInput{
		Kind: kind, Category: f.Category, Attributes: f.Attrs,
		Status: domain.ItemStatus(f.Status), Note: f.Note,
	}
}

// F-2.1: Liste mit Status- und Typfilter, kombinierbar.
func (s *Server) handleBikeList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status, category := q.Get("status"), q.Get("typ")
	if !validStatusFilter(status) {
		status = ""
	}
	if _, ok := s.category(category); !ok {
		category = ""
	}
	link := func(status, category string) string {
		v := url.Values{}
		if status != "" {
			v.Set("status", status)
		}
		if category != "" {
			v.Set("typ", category)
		}
		if len(v) == 0 {
			return "/fahrraeder"
		}
		return "/fahrraeder?" + v.Encode()
	}
	var statusTabs, typeTabs []tab
	for _, t := range []struct{ label, value string }{
		{"Alle", ""}, {"Verfügbar", "verfuegbar"}, {"Ausgeliehen", "ausgeliehen"}, {"Reserviert", "reserviert"}, {"Wartung", "wartung"},
	} {
		statusTabs = append(statusTabs, tab{Label: t.label, URL: link(t.value, category), Active: status == t.value})
	}
	typeTabs = append(typeTabs, tab{Label: "Alle Typen", URL: link(status, ""), Active: category == ""})
	for _, c := range s.bikes.Categories() {
		typeTabs = append(typeTabs, tab{Label: c.Label, URL: link(status, c.Key), Active: category == c.Key})
	}
	s.render(w, r, http.StatusOK, "bike_list.html", page{
		Title:    "Fahrräder",
		Subtitle: "Bestand, Typen, Stundenpreise und Status",
		Active:   "fahrraeder",
		Content: struct {
			Bikes      []service.ItemView
			StatusTabs []tab
			TypeTabs   []tab
			Filtered   bool
			Rates      map[string]string
		}{s.svc.Items(service.ItemFilter{Kind: kind, Category: category, Status: status}), statusTabs, typeTabs, status != "" || category != "", s.rateLabels()},
	})
}

func validStatusFilter(s string) bool {
	switch s {
	case "", "verfuegbar", "ausgeliehen", "wartung", service.FilterReservedItems:
		return true
	}
	return false
}

func (s *Server) category(key string) (domain.Category, bool) {
	for _, c := range s.bikes.Categories() {
		if c.Key == key {
			return c, true
		}
	}
	return domain.Category{}, false
}

// F-2.2
func (s *Server) handleBikeNew(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("typ")
	if _, ok := s.category(category); !ok {
		category = s.bikes.Categories()[0].Key
	}
	s.renderBikeForm(w, r, http.StatusOK, bikeForm{Category: category, Attrs: map[string]string{}, Status: string(domain.StatusAvailable)}, "")
}

func (s *Server) handleBikeCreate(w http.ResponseWriter, r *http.Request) {
	form, in := s.parseBikeForm(r)
	it, err := s.svc.CreateItem(in)
	if err != nil {
		s.bikeFormFailed(w, r, form, err)
		return
	}
	redirect(w, r, fmt.Sprintf("/fahrraeder/%d", it.ID), "bike_created")
}

// F-2.3
func (s *Server) handleBikeDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.handleNotFound(w, r)
		return
	}
	d, err := s.svc.ItemDetail(id)
	if err != nil || d.View.Item.Kind != kind {
		s.fail(w, r, notFoundIfNil(err))
		return
	}
	s.render(w, r, http.StatusOK, "bike_detail.html", page{
		Title:    d.View.Name,
		Subtitle: fmt.Sprintf("Fahrrad #%d · %s · %s", d.View.Item.ID, d.View.CategoryLabel, d.View.Rule),
		Active:   "fahrraeder",
		Content:  d,
	})
}

// F-2.2, F-2.4
func (s *Server) handleBikeEdit(w http.ResponseWriter, r *http.Request) {
	v, ok := s.bikeFromPath(w, r)
	if !ok {
		return
	}
	s.renderBikeForm(w, r, http.StatusOK, bikeFormFrom(v), "")
}

func (s *Server) handleBikeUpdate(w http.ResponseWriter, r *http.Request) {
	v, ok := s.bikeFromPath(w, r)
	if !ok {
		return
	}
	form, in := s.parseBikeForm(r)
	form.ID, form.StatusLocked = v.Item.ID, v.InUse()
	if _, err := s.svc.UpdateItem(v.Item.ID, in); err != nil {
		s.bikeFormFailed(w, r, form, err)
		return
	}
	redirect(w, r, fmt.Sprintf("/fahrraeder/%d", v.Item.ID), "bike_updated")
}

func (s *Server) bikeFromPath(w http.ResponseWriter, r *http.Request) (service.ItemView, bool) {
	id, ok := pathID(r)
	if !ok {
		s.handleNotFound(w, r)
		return service.ItemView{}, false
	}
	v, err := s.svc.Item(id)
	if err != nil || v.Item.Kind != kind {
		s.fail(w, r, notFoundIfNil(err))
		return service.ItemView{}, false
	}
	return v, true
}

func (s *Server) bikeFormFailed(w http.ResponseWriter, r *http.Request, form bikeForm, err error) {
	fields, msg, isFormError := formProblem(err)
	if !isFormError {
		s.fail(w, r, err)
		return
	}
	form.Errors = fields
	s.renderBikeForm(w, r, http.StatusUnprocessableEntity, form, msg)
}

func (s *Server) renderBikeForm(w http.ResponseWriter, r *http.Request, status int, form bikeForm, errMsg string) {
	form.Categories, form.Fields, form.Rates = s.bikes.Categories(), s.bikes.Fields(), s.rateLabels()
	title := "Neues Fahrrad"
	if form.ID != 0 {
		title = "Fahrrad bearbeiten"
	}
	s.render(w, r, status, "bike_form.html", page{
		Title:    title,
		Subtitle: "Pflichtfelder sind mit * markiert",
		Active:   "fahrraeder",
		Flash:    errorFlash(errMsg),
		Content:  form,
	})
}

// rateLabels zeigt den Stundensatz neben jedem Typ im Formular.
func (s *Server) rateLabels() map[string]string {
	m := map[string]string{}
	for _, c := range s.bikes.Categories() {
		m[c.Key] = s.bikes.Rate(c.Key).String() + "/Std"
	}
	return m
}

// F-2.5
func (s *Server) handleBikeConfirmDelete(w http.ResponseWriter, r *http.Request) {
	v, ok := s.bikeFromPath(w, r)
	if !ok {
		return
	}
	d, err := s.svc.ItemDetail(v.Item.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.renderConfirmBike(w, r, http.StatusOK, d)
}

func (s *Server) handleBikeDelete(w http.ResponseWriter, r *http.Request) {
	v, ok := s.bikeFromPath(w, r)
	if !ok {
		return
	}
	if err := s.svc.DeleteItem(v.Item.ID); err != nil {
		if d, derr := s.svc.ItemDetail(v.Item.ID); derr == nil && statusFor(err) == http.StatusConflict {
			s.renderConfirmBike(w, r, http.StatusConflict, d)
			return
		}
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/fahrraeder", "bike_deleted")
}

func (s *Server) renderConfirmBike(w http.ResponseWriter, r *http.Request, status int, d service.ItemDetail) {
	c := confirmData{
		What:   "das Fahrrad „" + d.View.Name + "“",
		Action: fmt.Sprintf("/fahrraeder/%d/loeschen", d.View.Item.ID),
		Cancel: fmt.Sprintf("/fahrraeder/%d", d.View.Item.ID),
	}
	if d.HasHistory() {
		c.Blocked = "Fahrrad hat Buchungen und kann nicht gelöscht werden. Setze es stattdessen auf Wartung."
	}
	s.renderConfirm(w, r, status, "Fahrrad löschen", "fahrraeder", c)
}

// confirmData füllt die gemeinsame Lösch-Bestätigungsseite.
type confirmData struct {
	What    string
	Action  string
	Cancel  string
	Blocked string
}

func (s *Server) renderConfirm(w http.ResponseWriter, r *http.Request, status int, title, active string, c confirmData) {
	s.render(w, r, status, "confirm_delete.html", page{Title: title, Active: active, Content: c})
}

// notFoundIfNil: Ein Objekt einer anderen Verleihart gilt in dieser Anwendung als nicht gefunden.
func notFoundIfNil(err error) error {
	if err == nil {
		return domain.ErrNotFound
	}
	return err
}
