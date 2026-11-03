package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"fahrradverleih/framework/domain"
)

// flash ist eine Hinweisbox oben im Inhalt (F-9).
type flash struct {
	Kind string // "success" oder "error"
	Text string
}

// flashMessages ist die feste Liste erlaubter Meldungen. Über die URL kommt nur der Schlüssel.
var flashMessages = map[string]string{
	"bike_created":   "Fahrrad wurde angelegt.",
	"bike_updated":   "Fahrrad wurde gespeichert.",
	"bike_deleted":   "Fahrrad wurde gelöscht.",
	"person_created": "Person wurde registriert.",
	"person_updated": "Person wurde gespeichert.",
	"person_deleted": "Person wurde gelöscht.",
	"rented":         "Fahrrad wurde ausgeliehen.",
	"reserved":       "Reservierung wurde angelegt.",
	"returned":       "Fahrrad wurde zurückgegeben.",
	"cancelled":      "Reservierung wurde storniert.",
	"reassigned":     "Buchung wurde umgebucht.",
}

func flashFromQuery(r *http.Request) *flash {
	if text, ok := flashMessages[r.URL.Query().Get("msg")]; ok {
		return &flash{Kind: "success", Text: text}
	}
	return nil
}

func errorFlash(text string) *flash {
	if text == "" {
		return nil
	}
	return &flash{Kind: "error", Text: text}
}

// errorText übersetzt die neutralen Framework-Fehler in Texte der Fahrrad-App (Spezifikation 8).
func errorText(err error) string {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return "Eintrag wurde nicht gefunden."
	case errors.Is(err, domain.ErrUnknownKind):
		return "Unbekannte Verleihart."
	case errors.Is(err, domain.ErrItemInMaintenance):
		return "Fahrrad ist in Wartung und kann nicht gebucht werden."
	case errors.Is(err, domain.ErrItemBooked):
		return "Fahrrad ist im gewählten Zeitraum bereits ausgeliehen oder reserviert."
	case errors.Is(err, domain.ErrItemInUse):
		return "Fahrrad ist ausgeliehen oder reserviert – Status kann nicht geändert werden."
	case errors.Is(err, domain.ErrAlreadyReturned):
		return "Ausleihe wurde bereits zurückgegeben."
	case errors.Is(err, domain.ErrNotStarted):
		return "Die Reservierung hat noch nicht begonnen. Storniere sie stattdessen."
	case errors.Is(err, domain.ErrNotCancellable):
		return "Nur Reservierungen, die noch nicht begonnen haben, können storniert werden."
	case errors.Is(err, domain.ErrNotReassignable):
		return "Nur Buchungen, die noch nicht ausgegeben wurden, können umgebucht werden."
	case errors.Is(err, domain.ErrHasHistory):
		return "Eintrag hat Buchungen und kann nicht gelöscht werden."
	default:
		return "Es ist ein unerwarteter Fehler aufgetreten."
	}
}

// statusFor ordnet Fehlern einen HTTP-Status zu.
func statusFor(err error) int {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrItemInMaintenance), errors.Is(err, domain.ErrItemBooked),
		errors.Is(err, domain.ErrItemInUse), errors.Is(err, domain.ErrAlreadyReturned),
		errors.Is(err, domain.ErrNotStarted), errors.Is(err, domain.ErrNotCancellable),
		errors.Is(err, domain.ErrNotReassignable), errors.Is(err, domain.ErrHasHistory):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// formProblem trennt Formularfehler (Feldfehler oder verletzte Regel) von Fehlern,
// die eine eigene Fehlerseite brauchen (z. B. unbekannte ID).
func formProblem(err error) (fields domain.ValidationErrors, message string, isFormError bool) {
	if v, ok := domain.AsValidation(err); ok {
		return v, "Bitte die markierten Felder prüfen.", true
	}
	if statusFor(err) == http.StatusConflict {
		return nil, errorText(err), true
	}
	return nil, "", false
}

// redirect leitet nach einem erfolgreichen POST weiter (Post/Redirect/Get, 303).
func redirect(w http.ResponseWriter, r *http.Request, path, msgKey string) {
	http.Redirect(w, r, withMsg(path, msgKey), http.StatusSeeOther)
}

func withMsg(path, msgKey string) string {
	u, err := url.Parse(path)
	if err != nil {
		return path
	}
	q := u.Query()
	q.Set("msg", msgKey)
	u.RawQuery = q.Encode()
	return u.String()
}

// safeBack erlaubt als Rücksprungziel nur lokale Pfade (F-5.4).
func safeBack(p string) string {
	const fallback = "/buchungen"
	if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.HasPrefix(p, "/\\") {
		return fallback
	}
	u, err := url.Parse(p)
	if err != nil || u.Scheme != "" || u.Host != "" {
		return fallback
	}
	return p
}
