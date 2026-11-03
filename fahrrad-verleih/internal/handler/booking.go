package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/service"
)

// Formate der HTML-Felder <input type="date"> / <input type="time"> und der Vorbelegung per URL.
const (
	dateLayout  = "2006-01-02"
	timeLayout  = "15:04"
	queryLayout = "2006-01-02T15:04"
)

// Werte des Felds start_mode (F-5.1).
const (
	startNow   = "sofort"
	startLater = "spaeter"
)

const msgQuoteChanged = "Die Angaben haben sich geändert – bitte neuen Preis prüfen und erneut bestätigen."

// bookingForm füllt das Buchungsformular (F-5.1 bis F-5.3).
type bookingForm struct {
	Bikes     []service.ItemView
	Persons   []domain.Person
	BikeID    int
	PersonID  int
	StartMode string
	StartDate string
	StartTime string
	EndDate   string
	EndTime   string
	Quote     *service.Quote
	Token     string
	Errors    domain.ValidationErrors
	MinLimit  time.Duration
	MaxLimit  time.Duration
}

// parseBookingForm liest das Formular. Bei „Sofort“ ist der Beginn die aktuelle Minute,
// damit eine Ausleihe ab jetzt nicht daran scheitert, dass beim Ausfüllen Zeit vergeht.
func (s *Server) parseBookingForm(r *http.Request) (bookingForm, service.BookingInput) {
	f := bookingForm{
		BikeID:    formInt(r, "fahrrad"),
		PersonID:  formInt(r, "person"),
		StartMode: r.FormValue("start_mode"),
		StartDate: strings.TrimSpace(r.FormValue("start_date")),
		StartTime: strings.TrimSpace(r.FormValue("start_time")),
		EndDate:   strings.TrimSpace(r.FormValue("end_date")),
		EndTime:   strings.TrimSpace(r.FormValue("end_time")),
	}
	if f.StartMode != startLater {
		f.StartMode = startNow
	}
	in := service.BookingInput{ItemID: f.BikeID, PersonID: f.PersonID, End: parseLocal(f.EndDate, f.EndTime)}
	// Diese Anwendung bucht nur Fahrräder (F-8.2). Ein Objekt einer anderen Verleihart gilt als
	// ungültige Auswahl, auch wenn jemand die ID im Formular manipuliert.
	if v, err := s.svc.Item(in.ItemID); err == nil && v.Item.Kind != kind {
		in.ItemID = 0
	}
	if f.StartMode == startNow {
		in.Start = s.svc.Now().Truncate(time.Minute)
	} else {
		in.Start = parseLocal(f.StartDate, f.StartTime)
	}
	return f, in
}

// parseLocal liest Datum und Uhrzeit in der Zeitzone des Servers. Ungültig → Nullwert → Feldfehler.
func parseLocal(date, clock string) time.Time {
	t, err := time.ParseInLocation(dateLayout+" "+timeLayout, date+" "+clock, time.Local)
	if err != nil {
		return time.Time{}
	}
	return t
}

// quoteToken fasst die geprüften Angaben zusammen (F-5.3, AK-9). Bei „Sofort“ fehlt der Beginn,
// weil er bei jeder Anfrage neu gesetzt wird; ändert sich dadurch der Preis, greift der Preisvergleich.
func quoteToken(mode string, in service.BookingInput, price domain.Money) string {
	start := "sofort"
	if mode == startLater {
		start = fmt.Sprint(in.Start.Unix())
	}
	return fmt.Sprintf("%d|%d|%s|%d|%d", in.ItemID, in.PersonID, start, in.End.Unix(), int64(price))
}

// nextFullHour ist die nächste volle Stunde nach t.
func nextFullHour(t time.Time) time.Time {
	t = t.In(time.Local)
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, time.Local)
}

// F-5.1: Vorbelegung per ?fahrrad=&person=&start=&ende= (z. B. aus der Verfügbarkeitssuche).
func (s *Server) handleBookingNew(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	form := bookingForm{BikeID: formInt(r, "fahrrad"), PersonID: formInt(r, "person"), StartMode: startNow}
	start := nextFullHour(s.svc.Now())
	end := start.Add(2 * time.Hour)
	if t, err := time.ParseInLocation(queryLayout, q.Get("start"), time.Local); err == nil {
		start, form.StartMode = t, startLater
		end = t.Add(2 * time.Hour)
	}
	if t, err := time.ParseInLocation(queryLayout, q.Get("ende"), time.Local); err == nil {
		end = t
	}
	form.StartDate, form.StartTime = start.Format(dateLayout), start.Format(timeLayout)
	form.EndDate, form.EndTime = end.Format(dateLayout), end.Format(timeLayout)
	s.renderBookingForm(w, r, http.StatusOK, form, "")
}

// F-5.2 (aktion=vorschau) und F-5.3 (aktion=bestaetigen).
func (s *Server) handleBookingCreate(w http.ResponseWriter, r *http.Request) {
	form, in := s.parseBookingForm(r)
	q, err := s.svc.Quote(in)
	if err != nil {
		s.bookingFormFailed(w, r, form, err)
		return
	}
	form.Quote, form.Token = &q, quoteToken(form.StartMode, in, q.Price.Amount)

	if r.FormValue("aktion") != "bestaetigen" {
		s.renderBookingForm(w, r, http.StatusOK, form, "") // AK-8: nur Vorschau
		return
	}
	if r.FormValue("geprueft") != form.Token {
		s.renderBookingForm(w, r, http.StatusOK, form, msgQuoteChanged) // AK-9
		return
	}
	b, err := s.svc.Book(in) // prüft und berechnet alles erneut unter Sperre
	if err != nil {
		form.Quote = nil
		s.bookingFormFailed(w, r, form, err)
		return
	}
	if b.Start.After(s.svc.Now()) {
		redirect(w, r, "/buchungen?status=reserviert", "reserved")
		return
	}
	redirect(w, r, "/buchungen?status=aktiv", "rented")
}

func (s *Server) bookingFormFailed(w http.ResponseWriter, r *http.Request, form bookingForm, err error) {
	fields, msg, isFormError := formProblem(err)
	if !isFormError {
		s.fail(w, r, err)
		return
	}
	form.Errors = fields
	s.renderBookingForm(w, r, http.StatusUnprocessableEntity, form, msg)
}

func (s *Server) renderBookingForm(w http.ResponseWriter, r *http.Request, status int, form bookingForm, errMsg string) {
	form.Bikes = s.svc.BookableItems(kind)
	form.Persons = s.svc.AllPersons()
	limits := s.bikes.Limits()
	form.MinLimit, form.MaxLimit = limits.MinDuration, limits.MaxDuration
	subtitle := "Fahrrad und Zeitraum wählen, Preis prüfen, verbindlich buchen"
	if form.Quote != nil {
		subtitle = "Bitte Preis prüfen und bestätigen"
	}
	s.render(w, r, status, "booking_form.html", page{
		Title:    "Neue Buchung",
		Subtitle: subtitle,
		Active:   "buchungen",
		Flash:    errorFlash(errMsg),
		Content:  form,
	})
}

// F-6
func (s *Server) handleBookingList(w http.ResponseWriter, r *http.Request) {
	filter := service.ParseBookingFilter(r.URL.Query().Get("status"))
	var tabs []tab
	for _, t := range []struct {
		label  string
		filter service.BookingFilter
	}{
		{"Aktiv", service.FilterActive},
		{"Reserviert", service.FilterReserved},
		{"Abgeschlossen", service.FilterClosed},
		{"Alle", service.FilterAll},
	} {
		tabs = append(tabs, tab{Label: t.label, URL: "/buchungen?status=" + string(t.filter), Active: filter == t.filter})
	}
	s.render(w, r, http.StatusOK, "booking_list.html", page{
		Title:    "Buchungen",
		Subtitle: "Aktive Ausleihen, Reservierungen und Historie",
		Active:   "buchungen",
		Content: struct {
			Bookings []service.BookingView
			Tabs     []tab
			Filter   string
			Back     string
		}{s.svc.Bookings(filter), tabs, string(filter), "/buchungen?status=" + string(filter)},
	})
}

// F-5.4, AK-13
func (s *Server) handleBookingReturn(w http.ResponseWriter, r *http.Request) {
	s.bookingAction(w, r, "returned", func(id int) error {
		_, err := s.svc.Return(id)
		return err
	})
}

// F-5.5, AK-14
func (s *Server) handleBookingCancel(w http.ResponseWriter, r *http.Request) {
	s.bookingAction(w, r, "cancelled", s.svc.Cancel)
}

func (s *Server) bookingAction(w http.ResponseWriter, r *http.Request, msgKey string, action func(id int) error) {
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

// F-5.6: freie Alternativen zeigen.
func (s *Server) handleReassignForm(w http.ResponseWriter, r *http.Request) {
	s.renderReassign(w, r, http.StatusOK, "")
}

// F-5.6, AK-25
func (s *Server) handleReassign(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.handleNotFound(w, r)
		return
	}
	b, err := s.svc.Reassign(id, formInt(r, "fahrrad"))
	if err != nil {
		if statusFor(err) == http.StatusConflict {
			s.renderReassign(w, r, http.StatusConflict, errorText(err))
			return
		}
		s.fail(w, r, err)
		return
	}
	// Eine wartende Buchung ist nach dem Umbuchen sofort aktiv und steht dann unter „Aktiv“.
	if b.Start.After(s.svc.Now()) {
		redirect(w, r, "/buchungen?status=reserviert", "reassigned")
		return
	}
	redirect(w, r, "/buchungen?status=aktiv", "reassigned")
}

func (s *Server) renderReassign(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	id, ok := pathID(r)
	if !ok {
		s.handleNotFound(w, r)
		return
	}
	view, offers, err := s.svc.Alternatives(id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, status, "booking_reassign.html", page{
		Title:    "Buchung umbuchen",
		Subtitle: fmt.Sprintf("Buchung #%d · %s · %s", view.Booking.ID, view.Person.FullName(), view.Item.Name),
		Active:   "buchungen",
		Flash:    errorFlash(errMsg),
		Content: struct {
			View   service.BookingView
			Offers []service.Offer
		}{view, offers},
	})
}
