// Package plugin definiert, was eine Verleihart dem Framework liefern muss (Spezifikation 3.2),
// und die Registry, in der Verleiharten beim Start eingetragen werden (Spezifikation 3.3).
package plugin

import "fahrradverleih/framework/domain"

// Kind beschreibt eine Verleihart.
type Kind struct {
	Key      string // URL-tauglich und eindeutig
	Singular string
	Plural   string
}

// Plugin legt fest, was verliehen wird und wie abgerechnet wird.
// Das Framework legt fest, wie verliehen wird.
type Plugin interface {
	Kind() Kind
	// Categories sind die Untertypen; mindestens einer ist Pflicht.
	Categories() []domain.Category
	// Fields beschreibt die zusätzlichen Merkmale eines Objekts.
	Fields() []domain.Field
	// Validate prüft Regeln über mehrere Merkmale. Einzelne Felder hat das Framework schon geprüft.
	Validate(item domain.Item) domain.ValidationErrors
	// DisplayName ist der Anzeigename eines Objekts.
	DisplayName(item domain.Item) string
	// Pricing ist die Preisstrategie der Verleihart.
	Pricing() PricingStrategy
	// Limits sind Mindest- und Höchstdauer einer Buchung.
	Limits() domain.BookingLimits
}

// PricingStrategy berechnet den Preis für ein Objekt und einen Zeitraum.
// Sie ist austauschbar: Das Framework kennt nur diese Schnittstelle.
type PricingStrategy interface {
	Quote(item domain.Item, p domain.Period) domain.Price
	// Describe ist der Regeltext für eine Kategorie, z. B. für Formulare und Tarifseiten.
	Describe(category string) string
}

// CategoryOf sucht eine Kategorie des Plugins.
func CategoryOf(p Plugin, key string) (domain.Category, bool) {
	for _, c := range p.Categories() {
		if c.Key == key {
			return c, true
		}
	}
	return domain.Category{}, false
}
