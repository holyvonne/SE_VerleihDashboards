package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"skiverleih/internal/domain"
	"skiverleih/internal/service"
)

// Formate der HTML-Felder <input type="date"> und <input type="time">.
const (
	dateLayout = "2006-01-02"
	timeLayout = "15:04"
)

const msgQuoteChanged = "Die Angaben haben sich geändert – bitte neuen Preis prüfen und erneut bestätigen."

// rentalForm füllt das Buchungsformular (F-4.1 bis F-4.3).
type rentalForm struct {
	Skis      []service.SkiView // alle Ski außer Wartung
	Persons   []domain.Person
	SkiID     int
	PersonID  int
	StartDate string
	StartTime string
	EndDate   string
	EndTime   string
	Rule      string         // Preisregel als Hinweis
	Quote     *service.Quote // gesetzt nach erfolgreicher Vorschau
	Token     string         // geprüfte Angaben der Vorschau (F-4.3)
	Errors    domain.ValidationErrors
}

func parseRentalForm(r *http.Request) (rentalForm, service.RentalInput) {
	f := rentalForm{
		SkiID:     formInt(r, "ski_id"),
		PersonID:  formInt(r, "person_id"),
		StartDate: strings.TrimSpace(r.FormValue("start_date")),
		StartTime: strings.TrimSpace(r.FormValue("start_time")),
		EndDate:   strings.TrimSpace(r.FormValue("end_date")),
		EndTime:   strings.TrimSpace(r.FormValue("end_time")),
	}
	return f, service.RentalInput{
		SkiID: f.SkiID, PersonID: f.PersonID,
		Start: parseLocal(f.StartDate, f.StartTime),
		End:   parseLocal(f.EndDate, f.EndTime),
	}
}

// parseLocal liest Datum und Uhrzeit in der Zeitzone des Servers (NF-12).
// Fehlende oder ungültige Werte ergeben den Nullwert, den die Domain als Fehler meldet.
func parseLocal(date, clock string) time.Time {
	t, err := time.ParseInLocation(dateLayout+" "+timeLayout, date+" "+clock, time.Local)
	if err != nil {
		return time.Time{}
	}
	return t
}

// quoteToken fasst die geprüften Angaben zusammen. Weicht er bei „Verbindlich buchen“
// vom neu berechneten Wert ab, hat sich seit der Vorschau etwas geändert (AK-14).
func quoteToken(in service.RentalInput, price domain.Money) string {
	return fmt.Sprintf("%d|%d|%d|%d|%d", in.SkiID, in.PersonID, in.Start.Unix(), in.End.Unix(), int64(price))
}

// F-4.1: ?ski= und ?person= belegen die Auswahl vor; Beginn = jetzt, Rückgabe = 17:00.
func (s *Server) handleRentalNew(w http.ResponseWriter, r *http.Request) {
	start := s.svc.Now().In(time.Local).Truncate(time.Minute)
	end := time.Date(start.Year(), start.Month(), start.Day(), 17, 0, 0, 0, time.Local)
	if start.Hour() >= 16 {
		end = end.AddDate(0, 0, 1)
	}
	s.renderRentalForm(w, r, http.StatusOK, rentalForm{
		SkiID:     formInt(r, "ski"),
		PersonID:  formInt(r, "person"),
		StartDate: start.Format(dateLayout), StartTime: start.Format(timeLayout),
		EndDate: end.Format(dateLayout), EndTime: end.Format(timeLayout),
	}, "")
}

// F-4.2 (aktion=vorschau) und F-4.3 (aktion=bestaetigen).
func (s *Server) handleRentalCreate(w http.ResponseWriter, r *http.Request) {
	form, in := parseRentalForm(r)

	q, err := s.svc.Quote(in)
	if err != nil {
		s.rentalFormFailed(w, r, form, err)
		return
	}
	form.Quote, form.Token = &q, quoteToken(in, q.Price)

	if r.FormValue("aktion") != "bestaetigen" {
		s.renderRentalForm(w, r, http.StatusOK, form, "") // AK-13: nur Vorschau, nichts gespeichert
		return
	}
	if r.FormValue("geprueft") != form.Token {
		s.renderRentalForm(w, r, http.StatusOK, form, msgQuoteChanged) // AK-14
		return
	}

	rental, err := s.svc.Rent(in) // berechnet und prüft alles erneut unter Sperre
	if err != nil {
		form.Quote = nil
		s.rentalFormFailed(w, r, form, err)
		return
	}
	if rental.IsReserved(s.svc.Now()) {
		redirect(w, r, "/ausleihen?status=reserviert", "reserved")
		return
	}
	redirect(w, r, "/ausleihen?status=aktiv", "rented")
}

func (s *Server) rentalFormFailed(w http.ResponseWriter, r *http.Request, form rentalForm, err error) {
	fields, msg, isFormError := formProblem(err)
	if !isFormError {
		s.fail(w, r, err)
		return
	}
	form.Errors = fields
	s.renderRentalForm(w, r, http.StatusUnprocessableEntity, form, msg)
}

func (s *Server) renderRentalForm(w http.ResponseWriter, r *http.Request, status int, form rentalForm, errMsg string) {
	form.Skis = s.svc.BookableSkis()
	form.Persons = s.svc.AllPersons()
	form.Rule = s.svc.PricingRule()
	subtitle := "Zeitraum wählen, Preis prüfen, verbindlich buchen"
	if form.Quote != nil {
		subtitle = "Bitte Preis prüfen und bestätigen"
	}
	s.render(w, r, status, "rental_form.html", page{
		Title:    "Neue Ausleihe",
		Subtitle: subtitle,
		Active:   "ausleihen",
		Flash:    errorFlash(errMsg),
		Content:  form,
	})
}

// F-5
func (s *Server) handleRentalList(w http.ResponseWriter, r *http.Request) {
	filter := service.ParseRentalFilter(r.URL.Query().Get("status"))
	var tabs []tab
	for _, t := range []struct {
		label  string
		filter service.RentalFilter
	}{
		{"Aktiv", service.FilterActive},
		{"Reserviert", service.FilterReserved},
		{"Abgeschlossen", service.FilterClosed},
		{"Alle", service.FilterAll},
	} {
		tabs = append(tabs, tab{Label: t.label, URL: "/ausleihen?status=" + string(t.filter), Active: filter == t.filter})
	}
	s.render(w, r, http.StatusOK, "rental_list.html", page{
		Title:    "Ausleihen",
		Subtitle: "Aktive Ausleihen, Reservierungen und Historie",
		Active:   "ausleihen",
		Content: struct {
			Rentals []service.RentalView
			Tabs    []tab
			Filter  string
			Back    string // Rücksprung nach „Zurückgeben“/„Stornieren“
		}{s.svc.Rentals(filter), tabs, string(filter), "/ausleihen?status=" + string(filter)},
	})
}

// F-4.4, AK-10, AK-17
func (s *Server) handleRentalReturn(w http.ResponseWriter, r *http.Request) {
	s.rentalAction(w, r, "returned", func(id int) error {
		_, err := s.svc.Return(id)
		return err
	})
}

// F-4.5, AK-18
func (s *Server) handleRentalCancel(w http.ResponseWriter, r *http.Request) {
	s.rentalAction(w, r, "cancelled", s.svc.Cancel)
}

// rentalAction führt Rückgabe oder Stornieren aus und springt zur Herkunftsseite zurück.
func (s *Server) rentalAction(w http.ResponseWriter, r *http.Request, msgKey string, action func(id int) error) {
	id, ok := pathID(r)
	if !ok {
		s.handleNotFound(w, r)
		return
	}
	back := safeBack(r.FormValue("zurueck"))
	if err := action(id); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, back, msgKey)
}
