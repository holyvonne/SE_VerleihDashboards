// Package domain enthält die Typen und Regeln, die jeder Verleih braucht:
// Personen, Verleihobjekte, Buchungen, Zeiträume, Geld, Merkmale und benannte Fehler.
// Es hängt von keinem anderen Projektpaket ab und kennt keine konkrete Verleihart.
package domain

import (
	"errors"
	"sort"
	"strings"
)

// Benannte Fehler der Geschäftsregeln (Spezifikation Abschnitt 8).
// Sie sind neutral formuliert; die Anwendung übersetzt sie in eigene UI-Texte.
var (
	ErrNotFound          = errors.New("eintrag nicht gefunden")
	ErrUnknownKind       = errors.New("unbekannte verleihart")
	ErrDuplicateKind     = errors.New("verleihart bereits registriert")
	ErrInvalidPlugin     = errors.New("ungültiges plugin")
	ErrItemInMaintenance = errors.New("objekt ist in wartung")
	ErrItemBooked        = errors.New("objekt ist im zeitraum bereits gebucht")
	ErrItemInUse         = errors.New("objekt hat offene buchungen")
	ErrAlreadyReturned   = errors.New("buchung wurde bereits zurückgegeben")
	ErrNotStarted        = errors.New("buchung wurde noch nicht ausgegeben")
	ErrNotCancellable    = errors.New("buchung kann nicht storniert werden")
	ErrNotReassignable   = errors.New("buchung kann nicht umgebucht werden")
	ErrHasHistory        = errors.New("eintrag hat buchungen")
)

// ValidationErrors ordnet Formularfeldern eine deutsche Fehlermeldung zu.
// Die Texte stehen direkt im Code, weil sie fachliche Regeln beschreiben und am Feld erscheinen.
type ValidationErrors map[string]string

func (v ValidationErrors) Error() string {
	fields := make([]string, 0, len(v))
	for f := range v {
		fields = append(fields, f)
	}
	sort.Strings(fields)
	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = f + ": " + v[f]
	}
	return "ungültige eingabe: " + strings.Join(parts, "; ")
}

// Add merkt sich nur die erste Meldung pro Feld.
func (v ValidationErrors) Add(field, msg string) {
	if _, exists := v[field]; !exists {
		v[field] = msg
	}
}

// Merge übernimmt alle Meldungen aus o, die für ein Feld noch fehlen.
func (v ValidationErrors) Merge(o ValidationErrors) {
	for f, msg := range o {
		v.Add(f, msg)
	}
}

// ErrOrNil gibt nil zurück, wenn keine Fehler gesammelt wurden.
// So bleibt `return errs.ErrOrNil()` frei von der typed-nil-Falle.
func (v ValidationErrors) ErrOrNil() error {
	if len(v) == 0 {
		return nil
	}
	return v
}

// AsValidation liefert die Feldfehler aus err, falls err welche enthält.
func AsValidation(err error) (ValidationErrors, bool) {
	var v ValidationErrors
	if errors.As(err, &v) {
		return v, true
	}
	return nil, false
}
