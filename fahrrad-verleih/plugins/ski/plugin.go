// Package ski ist das Beispiel-Plugin aus Spezifikation 4.8: Ski mit Tagestarif.
// Es zeigt, dass das Framework eine zweite Verleihart mit ganz anderer Abrechnung
// unterstützt, ohne dass Framework-Code geändert wird. Es hat keine Weboberfläche.
package ski

import (
	"fmt"
	"strings"
	"time"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/plugin"
)

const Key = "ski"

const (
	AttrBrand  = "marke"
	AttrModel  = "modell"
	AttrLength = "laenge_cm"
)

// Plugin implementiert plugin.Plugin für Ski.
type Plugin struct {
	tariff DailyRate
}

// New erstellt das Plugin mit einem Tagessatz. loc ist die Zeitzone der Kalendertage (nil = time.Local).
func New(perDay domain.Money, loc *time.Location) *Plugin {
	if loc == nil {
		loc = time.Local
	}
	return &Plugin{tariff: DailyRate{PerDay: perDay, Location: loc}}
}

var _ plugin.Plugin = (*Plugin)(nil)

func (p *Plugin) Kind() plugin.Kind { return plugin.Kind{Key: Key, Singular: "Ski", Plural: "Ski"} }

func (p *Plugin) Categories() []domain.Category {
	return []domain.Category{
		{Key: "allround", Label: "Allround"},
		{Key: "carving", Label: "Carving"},
		{Key: "freeride", Label: "Freeride"},
	}
}

func (p *Plugin) Fields() []domain.Field {
	return []domain.Field{
		{Key: AttrBrand, Label: "Marke", Type: domain.FieldText, Required: true, MaxLen: 50},
		{Key: AttrModel, Label: "Modell", Type: domain.FieldText, Required: true, MaxLen: 50},
		{Key: AttrLength, Label: "Länge", Type: domain.FieldNumber, Required: true, Min: 70, Max: 220, Unit: "cm"},
	}
}

// Validate: Ski haben keine Regeln über mehrere Felder.
func (p *Plugin) Validate(domain.Item) domain.ValidationErrors { return domain.ValidationErrors{} }

func (p *Plugin) DisplayName(it domain.Item) string {
	return strings.TrimSpace(fmt.Sprintf("%s %s (%s cm)", it.Attr(AttrBrand), it.Attr(AttrModel), it.Attr(AttrLength)))
}

func (p *Plugin) Pricing() plugin.PricingStrategy { return p.tariff }

func (p *Plugin) Limits() domain.BookingLimits {
	return domain.BookingLimits{MaxDuration: 60 * 24 * time.Hour}
}

// DailyRate berechnet einen festen Satz pro angefangenem Kalendertag, mindestens einen Tag (Regel aus 1b).
type DailyRate struct {
	PerDay   domain.Money
	Location *time.Location
}

func (d DailyRate) Quote(_ domain.Item, p domain.Period) domain.Price {
	n := CalendarDays(p, d.Location)
	unit := "angefangene Kalendertage"
	if n == 1 {
		unit = "angefangener Kalendertag"
	}
	return domain.Price{Amount: domain.Money(n) * d.PerDay, Explanation: fmt.Sprintf("%d %s × %s", n, unit, d.PerDay)}
}

func (d DailyRate) Describe(string) string {
	return fmt.Sprintf("%s pro angefangenem Kalendertag, mindestens %s", d.PerDay, d.PerDay)
}

// CalendarDays zählt die Kalendertage in loc, in denen ein Teil von [Start, End) liegt, mindestens 1.
func CalendarDays(p domain.Period, loc *time.Location) int {
	if p.IsOpen() || !p.End.After(p.Start) {
		return 1
	}
	first := civilDate(p.Start.In(loc))
	last := civilDate(p.End.Add(-time.Nanosecond).In(loc))
	// Datumsangaben in UTC haben immer 24 h Abstand – keine Sommerzeit-Effekte.
	return int(last.Sub(first).Hours()/24) + 1
}

func civilDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
