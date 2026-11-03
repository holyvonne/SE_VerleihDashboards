package plugin

import (
	"errors"
	"sync"
	"testing"

	"fahrradverleih/framework/domain"
)

// fake ist ein minimales Plugin; die Felder steuern, was Register prüfen soll.
type fake struct {
	key     string
	cats    []domain.Category
	fields  []domain.Field
	pricing PricingStrategy
}

type flat struct{}

func (flat) Quote(domain.Item, domain.Period) domain.Price { return domain.Price{Amount: 100} }
func (flat) Describe(string) string                        { return "pauschal" }

func newFake(key string) *fake {
	return &fake{key: key, cats: []domain.Category{{Key: "a", Label: "A"}}, pricing: flat{}}
}

func (f *fake) Kind() Kind                                   { return Kind{Key: f.key, Singular: f.key, Plural: f.key} }
func (f *fake) Categories() []domain.Category                { return f.cats }
func (f *fake) Fields() []domain.Field                       { return f.fields }
func (f *fake) Validate(domain.Item) domain.ValidationErrors { return nil }
func (f *fake) DisplayName(domain.Item) string               { return f.key }
func (f *fake) Pricing() PricingStrategy                     { return f.pricing }
func (f *fake) Limits() domain.BookingLimits                 { return domain.BookingLimits{} }

// AK-19
func TestRegister(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(newFake("eins")); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(newFake("zwei")); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(newFake("eins")); !errors.Is(err, domain.ErrDuplicateKind) {
		t.Errorf("doppelt: %v", err)
	}
	all := r.All()
	if len(all) != 2 || all[0].Kind().Key != "eins" || all[1].Kind().Key != "zwei" {
		t.Errorf("Reihenfolge: %v", all)
	}
	if p, err := r.Get("zwei"); err != nil || p.Kind().Key != "zwei" {
		t.Errorf("Get: %v %v", p, err)
	}
	if _, err := r.Get("drei"); !errors.Is(err, domain.ErrUnknownKind) {
		t.Errorf("unbekannt: %v", err)
	}
}

func TestRegisterRejectsInvalid(t *testing.T) {
	var nilFake *fake
	noCats := newFake("x")
	noCats.cats = nil
	noPricing := newFake("x")
	noPricing.pricing = nil
	dupCat := newFake("x")
	dupCat.cats = []domain.Category{{Key: "a"}, {Key: "a"}}
	dupField := newFake("x")
	dupField.fields = []domain.Field{{Key: "f", Type: domain.FieldText}, {Key: "f", Type: domain.FieldText}}
	emptyChoice := newFake("x")
	emptyChoice.fields = []domain.Field{{Key: "f", Type: domain.FieldChoice}}

	cases := map[string]Plugin{
		"nil":                nil,
		"typisierter nil":    nilFake,
		"leerer Schlüssel":   newFake(""),
		"Großbuchstaben":     newFake("Rad"),
		"Leerzeichen":        newFake("e bike"),
		"ohne Kategorien":    noCats,
		"ohne Preis":         noPricing,
		"doppelte Kategorie": dupCat,
		"doppeltes Merkmal":  dupField,
		"Auswahl ohne Werte": emptyChoice,
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			if err := NewRegistry().Register(p); !errors.Is(err, domain.ErrInvalidPlugin) {
				t.Errorf("erwartet ErrInvalidPlugin, got %v", err)
			}
		})
	}
}

// AK-19: parallele Zugriffe (mit -race prüfen).
func TestRegistryConcurrent(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newFake("basis"))
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = r.Get("basis"); _ = r.All() }()
		go func(i int) { defer wg.Done(); _ = r.Register(newFake("k" + string(rune('a'+i)))) }(i)
	}
	wg.Wait()
	if len(r.All()) != 21 {
		t.Errorf("erwartet 21 Plugins, got %d", len(r.All()))
	}
}
