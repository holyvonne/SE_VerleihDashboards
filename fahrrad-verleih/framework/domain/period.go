package domain

import (
	"fmt"
	"time"
)

// Period ist ein halboffener Zeitraum [Start, End). Ein leeres End bedeutet „offen“ (bis ∞).
type Period struct {
	Start time.Time
	End   time.Time
}

// IsOpen meldet, ob der Zeitraum kein Ende hat.
func (p Period) IsOpen() bool { return p.End.IsZero() }

// Overlaps prüft [a,b) ∩ [c,d) ≠ ∅, also a < d && c < b (Spezifikation 4.5).
// Angrenzende Zeiträume überschneiden sich nicht.
func (p Period) Overlaps(o Period) bool {
	return (o.IsOpen() || p.Start.Before(o.End)) && (p.IsOpen() || o.Start.Before(p.End))
}

// Duration ist die Länge des Zeitraums; offene Zeiträume haben die Länge 0.
func (p Period) Duration() time.Duration {
	if p.IsOpen() {
		return 0
	}
	return p.End.Sub(p.Start)
}

// BookingLimits sind die Grenzen einer Verleihart für die Dauer einer Buchung.
// 0 bedeutet „keine Grenze“.
type BookingLimits struct {
	MinDuration time.Duration
	MaxDuration time.Duration
}

// Meldungen für die Zeitfelder (AK-15).
const (
	MsgMissingTime = "Bitte Datum und Uhrzeit angeben"
	MsgStartPast   = "Beginn darf nicht in der Vergangenheit liegen"
	MsgEndBefore   = "Rückgabe muss nach dem Beginn liegen"
)

// ValidatePeriod prüft einen gewünschten Buchungszeitraum (Spezifikation 4.4).
// Der Beginn darf nicht vor der aktuellen Minute liegen, weil die Formulare Minuten erfassen.
// Feldnamen: "start" und "end".
func ValidatePeriod(p Period, now time.Time, limits BookingLimits) ValidationErrors {
	errs := ValidationErrors{}
	if p.Start.IsZero() {
		errs.Add("start", MsgMissingTime)
	}
	if p.End.IsZero() {
		errs.Add("end", MsgMissingTime)
	}
	if len(errs) > 0 {
		return errs
	}
	if p.Start.Before(now.Truncate(time.Minute)) {
		errs.Add("start", MsgStartPast)
	}
	d := p.End.Sub(p.Start)
	switch {
	case d <= 0:
		errs.Add("end", MsgEndBefore)
	case limits.MinDuration > 0 && d < limits.MinDuration:
		errs.Add("end", "Verleihdauer muss mindestens "+FormatLimit(limits.MinDuration)+" betragen")
	case limits.MaxDuration > 0 && d > limits.MaxDuration:
		errs.Add("end", "Verleihdauer darf höchstens "+FormatLimit(limits.MaxDuration)+" betragen")
	}
	return errs
}

// FormatLimit beschreibt eine Grenze in ganzen Tagen, Stunden oder Minuten, z. B. „30 Tage“, „1 Stunde“.
func FormatLimit(d time.Duration) string {
	switch {
	case d%(24*time.Hour) == 0:
		return plural(int(d/(24*time.Hour)), "Tag", "Tage")
	case d%time.Hour == 0:
		return plural(int(d/time.Hour), "Stunde", "Stunden")
	default:
		return plural(int(d/time.Minute), "Minute", "Minuten")
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
