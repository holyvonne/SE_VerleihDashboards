package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"skiverleih/internal/domain"
	"skiverleih/internal/service"
)

// maxPersonRequestBytes begrenzt die gesamte Formular-Anfrage inkl. Bild (Spezifikation 3.4).
// Dateien zwischen 2 und 3 MB erreichen so noch die Validierung und erhalten eine Feldmeldung.
const maxPersonRequestBytes = 3 << 20

const msgImageInvalid = "Ausweisbild muss JPEG oder PNG und höchstens 2 MB groß sein."

// personForm hält die Formularwerte als Text für das erneute Anzeigen.
type personForm struct {
	ID          int // 0 = neue Person
	FirstName   string
	LastName    string
	Phone       string
	Email       string
	IDType      string
	IDNumber    string
	HasImage    bool // es ist bereits ein Bild gespeichert (nur beim Bearbeiten)
	RemoveImage bool
	Errors      domain.ValidationErrors
}

func personFormFrom(p domain.Person) personForm {
	return personForm{
		ID: p.ID, FirstName: p.FirstName, LastName: p.LastName, Phone: p.Phone, Email: p.Email,
		IDType: string(p.IDType), IDNumber: p.IDNumber, HasImage: p.HasIDImage(),
	}
}

// parsePersonForm liest das multipart-Formular vollständig im Arbeitsspeicher.
// ok == false bedeutet: Es wurde bereits eine Fehlerseite geschrieben.
func (s *Server) parsePersonForm(w http.ResponseWriter, r *http.Request) (personForm, service.PersonInput, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPersonRequestBytes)
	// maxMemory = Obergrenze der Anfrage → nichts landet in temporären Dateien.
	if err := r.ParseMultipartForm(maxPersonRequestBytes); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			s.renderError(w, r, http.StatusRequestEntityTooLarge, msgImageInvalid)
		} else {
			s.renderError(w, r, http.StatusBadRequest, "Das Formular konnte nicht gelesen werden.")
		}
		return personForm{}, service.PersonInput{}, false
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	f := personForm{
		FirstName:   r.FormValue("first_name"),
		LastName:    r.FormValue("last_name"),
		Phone:       r.FormValue("phone"),
		Email:       r.FormValue("email"),
		IDType:      r.FormValue("id_type"),
		IDNumber:    r.FormValue("id_number"),
		RemoveImage: r.FormValue("remove_image") == "1",
	}
	in := service.PersonInput{
		FirstName: f.FirstName, LastName: f.LastName, Phone: f.Phone, Email: f.Email,
		IDType: domain.IDType(f.IDType), IDNumber: f.IDNumber, RemoveImage: f.RemoveImage,
	}

	file, _, err := r.FormFile("id_image")
	switch {
	case errors.Is(err, http.ErrMissingFile):
		// kein Bild gewählt – erlaubt (AK-7)
	case err != nil:
		s.renderError(w, r, http.StatusBadRequest, "Das Ausweisbild konnte nicht gelesen werden.")
		return personForm{}, service.PersonInput{}, false
	default:
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, domain.MaxIDImageBytes+1))
		if err != nil {
			s.renderError(w, r, http.StatusBadRequest, "Das Ausweisbild konnte nicht gelesen werden.")
			return personForm{}, service.PersonInput{}, false
		}
		in.Image = data // Größe und Typ prüft die Domain (AK-6)
	}
	return f, in, true
}

// F-3.1
func (s *Server) handlePersonList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	s.render(w, r, http.StatusOK, "person_list.html", page{
		Title:    "Personen",
		Subtitle: "Kundinnen und Kunden verwalten",
		Active:   "personen",
		Content: struct {
			Rows  []service.PersonRow
			Query string
		}{s.svc.Persons(q), q},
	})
}

// F-3.2
func (s *Server) handlePersonNew(w http.ResponseWriter, r *http.Request) {
	s.renderPersonForm(w, r, http.StatusOK, personForm{IDType: string(domain.IDCard)}, "")
}

func (s *Server) handlePersonCreate(w http.ResponseWriter, r *http.Request) {
	form, in, ok := s.parsePersonForm(w, r)
	if !ok {
		return
	}
	p, err := s.svc.CreatePerson(in)
	if err != nil {
		s.personFormFailed(w, r, form, err)
		return
	}
	redirect(w, r, fmt.Sprintf("/personen/%d", p.ID), "person_created")
}

// F-3.3
func (s *Server) handlePersonDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.handleNotFound(w, r)
		return
	}
	d, err := s.svc.PersonDetail(id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "person_detail.html", page{
		Title:    d.Person.FullName(),
		Subtitle: fmt.Sprintf("Person #%d · registriert am %s", d.Person.ID, d.Person.CreatedAt.Format("02.01.2006")),
		Active:   "personen",
		Content:  d,
	})
}

// F-3.4
func (s *Server) handlePersonEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.handleNotFound(w, r)
		return
	}
	p, err := s.svc.Person(id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.renderPersonForm(w, r, http.StatusOK, personFormFrom(p), "")
}

func (s *Server) handlePersonUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.handleNotFound(w, r)
		return
	}
	form, in, ok := s.parsePersonForm(w, r)
	if !ok {
		return
	}
	form.ID = id
	if _, err := s.svc.UpdatePerson(id, in); err != nil {
		if p, perr := s.svc.Person(id); perr == nil {
			form.HasImage = p.HasIDImage()
		}
		s.personFormFailed(w, r, form, err)
		return
	}
	redirect(w, r, fmt.Sprintf("/personen/%d", id), "person_updated")
}

func (s *Server) personFormFailed(w http.ResponseWriter, r *http.Request, form personForm, err error) {
	fields, msg, isFormError := formProblem(err)
	if !isFormError {
		s.fail(w, r, err)
		return
	}
	form.Errors = fields
	s.renderPersonForm(w, r, http.StatusUnprocessableEntity, form, msg)
}

func (s *Server) renderPersonForm(w http.ResponseWriter, r *http.Request, status int, form personForm, errMsg string) {
	title := "Neue Person"
	if form.ID != 0 {
		title = "Person bearbeiten"
	}
	s.render(w, r, status, "person_form.html", page{
		Title:    title,
		Subtitle: "Pflichtfelder sind mit * markiert",
		Active:   "personen",
		Flash:    errorFlash(errMsg),
		Content:  form,
	})
}

// Spezifikation 3.4: Bild nur über diese Route, nicht zwischenspeichern.
func (s *Server) handlePersonImage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.handleNotFound(w, r)
		return
	}
	p, err := s.svc.Person(id)
	if err != nil || !p.HasIDImage() {
		s.handleNotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Content-Type", p.IDImage.ContentType)
	h.Set("Content-Length", strconv.Itoa(len(p.IDImage.Data)))
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(p.IDImage.Data)
}

// F-3.5, F-6.3
func (s *Server) handlePersonConfirmDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.handleNotFound(w, r)
		return
	}
	d, err := s.svc.PersonDetail(id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.renderConfirmPerson(w, r, http.StatusOK, d)
}

func (s *Server) handlePersonDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.handleNotFound(w, r)
		return
	}
	if err := s.svc.DeletePerson(id); err != nil {
		if d, derr := s.svc.PersonDetail(id); derr == nil && statusFor(err) == http.StatusConflict {
			s.renderConfirmPerson(w, r, http.StatusConflict, d)
			return
		}
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/personen", "person_deleted")
}

func (s *Server) renderConfirmPerson(w http.ResponseWriter, r *http.Request, status int, d service.PersonDetail) {
	c := confirmData{
		What:   d.Person.FullName(),
		Action: fmt.Sprintf("/personen/%d/loeschen", d.Person.ID),
		Cancel: fmt.Sprintf("/personen/%d", d.Person.ID),
	}
	if d.HasHistory() {
		c.Blocked = "Person hat Ausleih-Historie und kann nicht gelöscht werden."
	}
	s.renderConfirm(w, r, status, "Person löschen", "personen", c)
}
