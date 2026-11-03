package domain

import "time"

// Period ist ein halboffener Zeitraum [Start, End): Start gehört dazu, End nicht.
// Ein leeres End (Nullwert) bedeutet „offen“, also bis unendlich (Spezifikation 3.6).
type Period struct {
	Start time.Time
	End   time.Time
}

// IsOpen meldet, ob der Zeitraum kein Ende hat.
func (p Period) IsOpen() bool { return p.End.IsZero() }

// Overlaps prüft, ob sich zwei Zeiträume überschneiden: [a, b) und [c, d)
// überschneiden sich genau dann, wenn a < d und c < b. Angrenzende Zeiträume
// (b == c) überschneiden sich also nicht (AK-16).
func (p Period) Overlaps(q Period) bool {
	return startsBeforeEnd(p.Start, q) && startsBeforeEnd(q.Start, p)
}

// startsBeforeEnd meldet t < q.End; ein offenes Ende ist größer als jeder Zeitpunkt.
func startsBeforeEnd(t time.Time, q Period) bool {
	return q.IsOpen() || t.Before(q.End)
}

// Duration ist die Länge des Zeitraums; offene Zeiträume haben keine Länge.
func (p Period) Duration() time.Duration {
	if p.IsOpen() {
		return 0
	}
	return p.End.Sub(p.Start)
}

// Grenzen für neue Buchungen (Spezifikation 3.3).
const (
	// StartTolerance deckt die Zeit ab, die das Ausfüllen des Formulars dauert.
	StartTolerance = 15 * time.Minute
	// MaxBookingDuration fängt Tippfehler wie ein falsches Jahr ab.
	MaxBookingDuration = 60 * 24 * time.Hour
)

const msgDateTimeMissing = "Bitte Datum und Uhrzeit angeben"

// ValidateBooking prüft Beginn und geplante Rückgabe einer neuen Buchung (AK-19).
// Ein Nullwert steht für ein fehlendes oder ungültiges Datum im Formular.
func ValidateBooking(p Period, now time.Time) error {
	errs := ValidationErrors{}
	if p.Start.IsZero() {
		errs.add("start", msgDateTimeMissing)
	}
	if p.End.IsZero() {
		errs.add("end", msgDateTimeMissing)
	}
	if len(errs) > 0 {
		return errs
	}
	if p.Start.Before(now.Add(-StartTolerance)) {
		errs.add("start", "Beginn darf nicht in der Vergangenheit liegen")
	}
	switch {
	case !p.End.After(p.Start):
		errs.add("end", "Rückgabe muss nach dem Beginn liegen")
	case p.Duration() > MaxBookingDuration:
		errs.add("end", "Verleihdauer darf höchstens 60 Tage betragen")
	}
	return errs.errOrNil()
}
