// Package handler ist die HTTP-Schicht der Fahrrad-Webanwendung. Sie übersetzt Anfragen in
// Service-Aufrufe und rendert HTML. Hier liegt keine Geschäftslogik.
package handler

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/service"
	"fahrradverleih/plugins/fahrrad"
	"fahrradverleih/web"
)

// kind ist die Verleihart dieser Anwendung (F-8.2).
const kind = fahrrad.Key

// maxFormBytes begrenzt normale Formulare (NF-4).
const maxFormBytes = 64 << 10

// Server bündelt Routen, Templates, Service und Fahrrad-Plugin.
// Das Plugin wird direkt gebraucht für Texte, Formularfelder und die Tarifseite (A-R4).
type Server struct {
	mux   *http.ServeMux
	pages map[string]*template.Template
	svc   *service.Service
	bikes *fahrrad.Plugin
}

// page ist das gemeinsame Datenobjekt für das Layout.
type page struct {
	Title    string
	Subtitle string
	Active   string // hervorgehobener Navigationspunkt
	Flash    *flash
	Content  any
}

var pageFiles = []string{
	"dashboard.html", "error.html", "confirm_delete.html",
	"bike_list.html", "bike_form.html", "bike_detail.html",
	"person_list.html", "person_form.html", "person_detail.html",
	"booking_list.html", "booking_form.html", "booking_reassign.html",
	"availability.html", "tariffs.html",
}

var templateFuncs = template.FuncMap{
	"formatTime":     func(t time.Time) string { return t.In(time.Local).Format("02.01.2006 15:04") },
	"formatDate":     func(t time.Time) string { return t.In(time.Local).Format("02.01.2006") },
	"formatClock":    func(t time.Time) string { return t.In(time.Local).Format("15:04") },
	"formatDuration": formatDuration,
	"formatLimit":    domain.FormatLimit,
	"dict":           dict,
	"idTypes":        func() []domain.IDType { return domain.IDTypes },
	"baseStatuses":   func() []domain.ItemStatus { return domain.BaseStatuses },
}

// New erstellt den Server und parst alle Templates beim Start.
func New(svc *service.Service, bikes *fahrrad.Plugin) (*Server, error) {
	s := &Server{mux: http.NewServeMux(), pages: map[string]*template.Template{}, svc: svc, bikes: bikes}
	for _, name := range pageFiles {
		t, err := template.New(name).Funcs(templateFuncs).ParseFS(web.FS,
			"templates/layout.html", "templates/icons.html", "templates/partials.html", "templates/"+name)
		if err != nil {
			return nil, fmt.Errorf("template %s: %w", name, err)
		}
		s.pages[name] = t
	}
	static, err := fs.Sub(web.FS, "static")
	if err != nil {
		return nil, err
	}
	s.routes(http.StripPrefix("/static/", http.FileServerFS(static)))
	return s, nil
}

// routes registriert alle Routen aus Spezifikation Abschnitt 7.
func (s *Server) routes(static http.Handler) {
	m := s.mux
	m.HandleFunc("GET /{$}", s.handleDashboard)

	m.HandleFunc("GET /fahrraeder", s.handleBikeList)
	m.HandleFunc("GET /fahrraeder/neu", s.handleBikeNew)
	m.HandleFunc("POST /fahrraeder", s.handleBikeCreate)
	m.HandleFunc("GET /fahrraeder/{id}", s.handleBikeDetail)
	m.HandleFunc("GET /fahrraeder/{id}/bearbeiten", s.handleBikeEdit)
	m.HandleFunc("POST /fahrraeder/{id}", s.handleBikeUpdate)
	m.HandleFunc("GET /fahrraeder/{id}/loeschen", s.handleBikeConfirmDelete)
	m.HandleFunc("POST /fahrraeder/{id}/loeschen", s.handleBikeDelete)

	m.HandleFunc("GET /personen", s.handlePersonList)
	m.HandleFunc("GET /personen/neu", s.handlePersonNew)
	m.HandleFunc("POST /personen", s.handlePersonCreate)
	m.HandleFunc("GET /personen/{id}", s.handlePersonDetail)
	m.HandleFunc("GET /personen/{id}/bearbeiten", s.handlePersonEdit)
	m.HandleFunc("POST /personen/{id}", s.handlePersonUpdate)
	m.HandleFunc("GET /personen/{id}/loeschen", s.handlePersonConfirmDelete)
	m.HandleFunc("POST /personen/{id}/loeschen", s.handlePersonDelete)
	m.HandleFunc("GET /personen/{id}/ausweisbild", s.handlePersonImage)

	m.HandleFunc("GET /verfuegbarkeit", s.handleAvailability)

	m.HandleFunc("GET /buchungen", s.handleBookingList)
	m.HandleFunc("GET /buchungen/neu", s.handleBookingNew)
	m.HandleFunc("POST /buchungen", s.handleBookingCreate)
	m.HandleFunc("POST /buchungen/{id}/rueckgabe", s.handleBookingReturn)
	m.HandleFunc("POST /buchungen/{id}/stornieren", s.handleBookingCancel)
	m.HandleFunc("GET /buchungen/{id}/umbuchen", s.handleReassignForm)
	m.HandleFunc("POST /buchungen/{id}/umbuchen", s.handleReassign)

	m.HandleFunc("GET /tarife", s.handleTariffs)

	m.Handle("GET /static/", static)
	m.HandleFunc("/", s.handleNotFound)
}

// ServeHTTP protokolliert jede Anfrage (NF-10) und begrenzt normale Formulare (NF-4).
// Das Personenformular setzt mit dem Ausweisbild eine eigene, größere Grenze.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	// Präfix-Vergleich, weil manche Clients „; charset=UTF-8“ anhängen.
	if r.Method == http.MethodPost && strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		r.Body = http.MaxBytesReader(rec, r.Body, maxFormBytes)
	}
	s.mux.ServeHTTP(rec, r)
	log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Microsecond))
}

// render schreibt erst in einen Puffer, damit bei Template-Fehlern keine halbe Seite ankommt.
func (s *Server) render(w http.ResponseWriter, r *http.Request, status int, name string, p page) {
	t, ok := s.pages[name]
	if !ok {
		http.Error(w, "Template nicht gefunden: "+name, http.StatusInternalServerError)
		return
	}
	if p.Flash == nil {
		p.Flash = flashFromQuery(r)
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", p); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "Interner Fehler beim Darstellen der Seite", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int, message string) {
	s.render(w, r, status, "error.html", page{Title: errorTitle(status), Content: message})
}

// fail zeigt einen Service-Fehler an: unbekannte ID → 404, verletzte Regel → 409, sonst 500.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	status := statusFor(err)
	if status == http.StatusInternalServerError {
		log.Printf("interner Fehler: %v", err)
	}
	s.renderError(w, r, status, errorText(err))
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	s.renderError(w, r, http.StatusNotFound, "Die angeforderte Seite gibt es nicht.")
}

func errorTitle(status int) string {
	switch status {
	case http.StatusNotFound:
		return "Nicht gefunden"
	case http.StatusConflict:
		return "Aktion nicht möglich"
	case http.StatusRequestEntityTooLarge:
		return "Anfrage zu groß"
	case http.StatusBadRequest:
		return "Ungültige Anfrage"
	default:
		return "Fehler"
	}
}

// pathID liest den Platzhalter {id} aus der URL.
func pathID(r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	return id, err == nil && id > 0
}

// formInt liest eine Ganzzahl; ungültig → 0, was die Validierung als Fehler meldet.
func formInt(r *http.Request, field string) int {
	n, _ := strconv.Atoi(r.FormValue(field))
	return n
}

// formatDuration zeigt „2 T 3 Std“, „3 Std 30 Min“, „3 Std“ oder „45 Min“ (F-6).
func formatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	days, hours, mins := minutes/(24*60), minutes/60%24, minutes%60
	switch {
	case days > 0:
		return fmt.Sprintf("%d T %d Std", days, hours)
	case hours > 0 && mins > 0:
		return fmt.Sprintf("%d Std %d Min", hours, mins)
	case hours > 0:
		return fmt.Sprintf("%d Std", hours)
	default:
		return fmt.Sprintf("%d Min", mins)
	}
}

// dict baut eine Map aus Schlüssel-Wert-Paaren, um Partials mehrere Werte zu übergeben.
func dict(kv ...any) (map[string]any, error) {
	if len(kv)%2 != 0 {
		return nil, fmt.Errorf("dict: ungerade Anzahl Argumente")
	}
	m := make(map[string]any, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict: Schlüssel %v ist kein string", kv[i])
		}
		m[key] = kv[i+1]
	}
	return m, nil
}

// tab ist ein Filter-Reiter über einer Liste.
type tab struct {
	Label  string
	URL    string
	Active bool
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
