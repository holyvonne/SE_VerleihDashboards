package domain

import (
	"fmt"
	"strconv"
	"time"
)

// Money ist ein Geldbetrag in ganzen Cent – nie float64 (Spezifikation 3.3).
type Money int64

// Euro erzeugt einen Betrag aus ganzen Euro.
func Euro(e int64) Money { return Money(e * 100) }

// String formatiert deutsch mit Tausenderpunkt, z. B. „1.240,00 €“ (F-6.6).
func (m Money) String() string {
	sign := ""
	if m < 0 {
		sign, m = "-", -m
	}
	euros := strconv.FormatInt(int64(m)/100, 10)
	for i := len(euros) - 3; i > 0; i -= 3 {
		euros = euros[:i] + "." + euros[i:]
	}
	return fmt.Sprintf("%s%s,%02d €", sign, euros, int64(m)%100)
}

// PricingPolicy ist die austauschbare Preisregel (Spezifikation 3.7).
// Der Service kennt nur diese Schnittstelle; die konkrete Regel wird in main.go gewählt.
type PricingPolicy interface {
	// Price berechnet den Preis für einen abgeschlossenen Zeitraum.
	Price(p Period) Money
	// Explain beschreibt die Rechnung, z. B. „3 angefangene Kalendertage × 20,00 €“.
	Explain(p Period) string
	// Rule beschreibt die Regel für das Formular.
	Rule() string
}

// DailyRate berechnet einen festen Satz pro angefangenem Kalendertag, mindestens einen Tag.
type DailyRate struct {
	PerDay   Money
	Location *time.Location // Zeitzone für Kalendertage; nil = time.Local
}

// StandardPricing ist das vorläufige Preismodell aus 1b: 20 € pro angefangenem Kalendertag.
func StandardPricing() DailyRate { return DailyRate{PerDay: Euro(20), Location: time.Local} }

func (d DailyRate) location() *time.Location {
	if d.Location == nil {
		return time.Local
	}
	return d.Location
}

// Days liefert die Anzahl angefangener Kalendertage (mindestens 1).
func (d DailyRate) Days(p Period) int { return CalendarDays(p, d.location()) }

func (d DailyRate) Price(p Period) Money { return Money(d.Days(p)) * d.PerDay }

func (d DailyRate) Explain(p Period) string {
	n := d.Days(p)
	unit := "angefangene Kalendertage"
	if n == 1 {
		unit = "angefangener Kalendertag"
	}
	return fmt.Sprintf("%d %s × %s", n, unit, d.PerDay)
}

func (d DailyRate) Rule() string {
	return fmt.Sprintf("%s pro angefangenem Kalendertag, mindestens %s", d.PerDay, d.PerDay)
}

// CalendarDays zählt die Kalendertage (in loc), in denen mindestens ein Teil von
// [Start, End) liegt. Das Ende ist exklusiv: Eine Rückgabe um 00:00 zählt den neuen
// Tag nicht mit. Ergebnis ist mindestens 1 (Mindestpreis).
func CalendarDays(p Period, loc *time.Location) int {
	if p.IsOpen() || !p.End.After(p.Start) {
		return 1
	}
	first := civilDate(p.Start.In(loc))
	last := civilDate(p.End.Add(-time.Nanosecond).In(loc))
	// Datumsangaben in UTC haben immer genau 24 h Abstand – keine Sommerzeit-Effekte.
	days := int(last.Sub(first).Hours()/24) + 1
	if days < 1 {
		return 1
	}
	return days
}

func civilDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
