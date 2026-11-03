// Package domain enthält das Fachmodell des Ski-Verleihs: Structs, Aufzählungen,
// Zeiträume, Preisregel, Validierung und benannte Fehler.
// Es hängt von keinem anderen Projektpaket ab.
package domain

import (
	"errors"
	"sort"
	"strings"
)

// Benannte Fehler der Geschäftsregeln (Spezifikation Abschnitt 7).
// Die Übersetzung in UI-Texte erfolgt im Handler.
var (
	ErrNotFound         = errors.New("eintrag nicht gefunden")
	ErrSkiInMaintenance = errors.New("ski ist in wartung")
	ErrSkiBooked        = errors.New("ski ist im zeitraum bereits gebucht")
	ErrAlreadyReturned  = errors.New("ausleihe wurde bereits zurückgegeben")
	ErrNotStarted       = errors.New("reservierung hat noch nicht begonnen")
	ErrNotCancellable   = errors.New("buchung kann nicht storniert werden")
	ErrSkiInUse         = errors.New("ski ist ausgeliehen oder reserviert")
	ErrHasHistory       = errors.New("eintrag hat ausleih-historie")
)

// ValidationErrors ordnet Formularfeldern eine deutsche Fehlermeldung zu.
// Die Texte stehen direkt hier, weil sie fachliche Regeln beschreiben
// (z. B. „Länge muss zwischen 70 und 220 cm liegen“) und am Feld angezeigt werden.
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

// add merkt sich nur die erste Meldung pro Feld.
func (v ValidationErrors) add(field, msg string) {
	if _, exists := v[field]; !exists {
		v[field] = msg
	}
}

// errOrNil gibt nil zurück, wenn keine Fehler gesammelt wurden.
// So bleibt `return errs.errOrNil()` frei von der typed-nil-Falle.
func (v ValidationErrors) errOrNil() error {
	if len(v) == 0 {
		return nil
	}
	return v
}
