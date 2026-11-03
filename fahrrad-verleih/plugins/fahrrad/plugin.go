// Package fahrrad ist das Fahrrad-Plugin (Spezifikation 4.3): Verleihart „Fahrrad“ mit den Typen
// Citybike, Mountainbike und E-Bike, Merkmalen, einer Regel für den Akku und Stundentarif.
// Es importiert nur framework/domain und framework/plugin (A-R2).
package fahrrad

import (
	"strings"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/plugin"
)

// Key ist der Schlüssel der Verleihart.
const Key = "fahrrad"

// Fahrradtypen (Kategorien).
const (
	Citybike     = "citybike"
	Mountainbike = "mountainbike"
	EBike        = "ebike"
)

// Merkmale.
const (
	AttrBrand   = "marke"
	AttrModel   = "modell"
	AttrFrame   = "rahmengroesse"
	AttrBattery = "akku_wh"
)

var categories = []domain.Category{
	{Key: Citybike, Label: "Citybike"},
	{Key: Mountainbike, Label: "Mountainbike"},
	{Key: EBike, Label: "E-Bike"},
}

var fields = []domain.Field{
	{Key: AttrBrand, Label: "Marke", Type: domain.FieldText, Required: true, MaxLen: 50},
	{Key: AttrModel, Label: "Modell", Type: domain.FieldText, Required: true, MaxLen: 50},
	{Key: AttrFrame, Label: "Rahmengröße", Type: domain.FieldChoice, Required: true, Options: []domain.Option{
		{Key: "S", Label: "S"}, {Key: "M", Label: "M"}, {Key: "L", Label: "L"}, {Key: "XL", Label: "XL"},
	}},
	{Key: AttrBattery, Label: "Akkukapazität", Type: domain.FieldNumber, Min: 200, Max: 1000, Unit: "Wh",
		Hint: "nur bei E-Bikes, 200–1000 Wh"},
}

// Meldungen der Akku-Regel (AK-3).
const (
	MsgBatteryRequired  = "Für E-Bikes bitte die Akkukapazität angeben"
	MsgBatteryForbidden = "Akkukapazität nur bei E-Bikes angeben"
)

// Plugin implementiert plugin.Plugin für Fahrräder.
type Plugin struct {
	cfg    Config
	tariff HourlyTariff
}

// New prüft die Konfiguration und erstellt das Plugin.
func New(cfg Config) (*Plugin, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cfg = cfg.WithRates(nil) // eigene Kopie, damit spätere Änderungen am Aufrufer nichts verschieben
	return &Plugin{cfg: cfg, tariff: HourlyTariff{Rates: cfg.Rates}}, nil
}

var _ plugin.Plugin = (*Plugin)(nil)

func (p *Plugin) Kind() plugin.Kind {
	return plugin.Kind{Key: Key, Singular: "Fahrrad", Plural: "Fahrräder"}
}

func (p *Plugin) Categories() []domain.Category   { return append([]domain.Category(nil), categories...) }
func (p *Plugin) Fields() []domain.Field          { return append([]domain.Field(nil), fields...) }
func (p *Plugin) Pricing() plugin.PricingStrategy { return p.tariff }

func (p *Plugin) Limits() domain.BookingLimits {
	return domain.BookingLimits{MinDuration: p.cfg.MinDuration, MaxDuration: p.cfg.MaxDuration}
}

// Rate liefert den konfigurierten Stundensatz eines Typs (Tarifseite, F-7).
func (p *Plugin) Rate(category string) domain.Money { return p.tariff.Rate(category) }

// Validate prüft die Akku-Regel, die zwei Felder betrifft (Spezifikation 4.3).
func (p *Plugin) Validate(it domain.Item) domain.ValidationErrors {
	errs := domain.ValidationErrors{}
	battery := strings.TrimSpace(it.Attr(AttrBattery))
	key := fieldKey(AttrBattery)
	switch {
	case it.Category == EBike && battery == "":
		errs.Add(key, MsgBatteryRequired)
	case it.Category != EBike && battery != "":
		errs.Add(key, MsgBatteryForbidden)
	}
	return errs
}

// DisplayName ist „Marke Modell“.
func (p *Plugin) DisplayName(it domain.Item) string {
	return strings.TrimSpace(it.Attr(AttrBrand) + " " + it.Attr(AttrModel))
}

func fieldKey(attr string) string {
	for _, f := range fields {
		if f.Key == attr {
			return f.FormKey()
		}
	}
	return attr
}
