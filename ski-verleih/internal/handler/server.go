// Package handler übersetzt HTTP-Anfragen in Service-Aufrufe und rendert HTML.
// Hier liegt keine Geschäftslogik.
package handler

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"time"

	"skiverleih/internal/domain"
	"skiverleih/internal/service"
	"skiverleih/web"
)

// Server bündelt Routen, Templates und den Service.
type Server struct {
	mux   *http.ServeMux
	pages map[string]*template.Template
	svc   *service.Service
}

// page ist das gemeinsame Datenobjekt für das Layout.
type page struct {
	Title    string
	Subtitle string
	Active   string // hervorgehobener Navigationspunkt (F-6.1)
	Flash    *flash // Hinweisbox (F-6.2)
	Content  any
}

// Seiten-Templates; jedes wird zusammen mit Layout und Icons geparst.
var pageFiles = []string{
	"dashboard.html", "error.html", "confirm_delete.html",
	"ski_list.html", "ski_form.html", "ski_detail.html",
	"person_list.html", "person_form.html", "person_detail.html",
	"rental_list.html", "rental_form.html",
}

var templateFuncs = template.FuncMap{
	"formatTime":     func(t time.Time) string { return t.Format("02.01.2006 15:04") }, // F-6.6
	"formatDate":     func(t time.Time) string { return t.Format("02.01.2006") },
	"formatDuration": formatDuration,
	"dict":           dict,
	"skiTypes":       func() []domain.SkiType { return domain.SkiTypes },
	"conditions":     func() []domain.Condition { return domain.Conditions },
	"idTypes":        func() []domain.IDType { return domain.IDTypes },
	"manualStatuses": func() []domain.SkiStatus { return []domain.SkiStatus{domain.SkiAvailable, domain.SkiMaintenance} },
}

// New erstellt den Server und parst alle Templates beim Start,
// damit Template-Fehler sofort und nicht erst bei der ersten Anfrage auffallen.
func New(svc *service.Service) (*Server, error) {
	s := &Server{
		mux:   http.NewServeMux(),
		pages: map[string]*template.Template{},
		svc:   svc,
	}
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

// routes registriert alle Routen aus Spezifikation Abschnitt 6.
func (s *Server) routes(static http.Handler) {
	m := s.mux
	m.HandleFunc("GET /{$}", s.handleDashboard)

	m.HandleFunc("GET /ski", s.handleSkiList)
	m.HandleFunc("GET /ski/neu", s.handleSkiNew)
	m.HandleFunc("POST /ski", s.handleSkiCreate)
	m.HandleFunc("GET /ski/{id}", s.handleSkiDetail)
	m.HandleFunc("GET /ski/{id}/bearbeiten", s.handleSkiEdit)
	m.HandleFunc("POST /ski/{id}", s.handleSkiUpdate)
	m.HandleFunc("GET /ski/{id}/loeschen", s.handleSkiConfirmDelete)
	m.HandleFunc("POST /ski/{id}/loeschen", s.handleSkiDelete)

	m.HandleFunc("GET /personen", s.handlePersonList)
	m.HandleFunc("GET /personen/neu", s.handlePersonNew)
	m.HandleFunc("POST /personen", s.handlePersonCreate)
	m.HandleFunc("GET /personen/{id}", s.handlePersonDetail)
	m.HandleFunc("GET /personen/{id}/bearbeiten", s.handlePersonEdit)
	m.HandleFunc("POST /personen/{id}", s.handlePersonUpdate)
	m.HandleFunc("GET /personen/{id}/loeschen", s.handlePersonConfirmDelete)
	m.HandleFunc("POST /personen/{id}/loeschen", s.handlePersonDelete)
	m.HandleFunc("GET /personen/{id}/ausweisbild", s.handlePersonImage)

	m.HandleFunc("GET /ausleihen", s.handleRentalList)
	m.HandleFunc("GET /ausleihen/neu", s.handleRentalNew)
	m.HandleFunc("POST /ausleihen", s.handleRentalCreate)
	m.HandleFunc("POST /ausleihen/{id}/rueckgabe", s.handleRentalReturn)
	m.HandleFunc("POST /ausleihen/{id}/stornieren", s.handleRentalCancel)

	m.Handle("GET /static/", static)
	m.HandleFunc("/", s.handleNotFound)
}

// ServeHTTP protokolliert jede Anfrage (NF-10) und leitet sie an die Routen weiter.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	s.mux.ServeHTTP(rec, r)
	log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Microsecond))
}

// render schreibt erst in einen Puffer, damit bei Template-Fehlern
// keine halbe Seite mit Status 200 beim Browser ankommt.
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

// renderError zeigt die eigene Fehlerseite (Spezifikation Abschnitt 6).
func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int, message string) {
	s.render(w, r, status, "error.html", page{
		Title:   errorTitle(status),
		Content: message,
	})
}

// fail zeigt einen Fehler aus dem Service passend an:
// unbekannte ID → 404, verletzte Geschäftsregel → 409, sonst 500.
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

// errorTitle liefert deutsche Seitentitel (F-6.6); http.StatusText ist englisch.
func errorTitle(status int) string {
	switch status {
	case http.StatusNotFound:
		return "Nicht gefunden"
	case http.StatusConflict:
		return "Aktion nicht möglich"
	case http.StatusRequestEntityTooLarge:
		return "Datei zu groß"
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

// formInt liest eine Ganzzahl aus einem Formularfeld; ungültig → 0,
// was die Validierung anschließend als Fehler meldet.
func formInt(r *http.Request, field string) int {
	n, _ := strconv.Atoi(r.FormValue(field))
	return n
}

// formatDuration zeigt eine Dauer als „2 T 3 Std“, „3 Std 30 Min“, „3 Std“ oder „45 Min“ (F-5.1).
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

// tab ist ein Filter-Reiter über einer Liste (z. B. Status-Filter).
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
