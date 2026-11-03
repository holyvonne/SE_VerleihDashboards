package fahrrad

import (
	"fmt"
	"time"

	"fahrradverleih/framework/domain"
)

// HourlyTariff ist die Preisstrategie des Fahrradverleihs (Spezifikation 4.6):
// angefangene Stunden × Stundensatz des Fahrradtyps, mindestens 1 Stunde.
type HourlyTariff struct {
	Rates map[string]domain.Money
}

// StartedHours zählt die angefangenen Stunden einer Dauer, mindestens 1.
// Gerechnet wird mit der tatsächlich vergangenen Zeit, daher keine Sonderfälle bei der Zeitumstellung.
func StartedHours(d time.Duration) int {
	if d <= 0 {
		return 1
	}
	h := int(d / time.Hour)
	if d%time.Hour != 0 {
		h++
	}
	return h
}

// Rate liefert den Stundensatz eines Typs.
func (t HourlyTariff) Rate(category string) domain.Money { return t.Rates[category] }

func (t HourlyTariff) Quote(item domain.Item, p domain.Period) domain.Price {
	hours := StartedHours(p.Duration())
	rate := t.Rate(item.Category)
	unit := "angefangene Stunden"
	if hours == 1 {
		unit = "angefangene Stunde"
	}
	return domain.Price{
		Amount:      domain.Money(hours) * rate,
		Explanation: fmt.Sprintf("%d %s × %s", hours, unit, rate),
	}
}

func (t HourlyTariff) Describe(category string) string {
	return fmt.Sprintf("%s pro angefangene Stunde, mindestens 1 Stunde", t.Rate(category))
}
