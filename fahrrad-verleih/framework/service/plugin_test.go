package service_test

import (
	"testing"
	"time"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/plugin"
	"fahrradverleih/framework/service"
	"fahrradverleih/plugins/fahrrad"
	"fahrradverleih/plugins/ski"
)

// testgeraet ist eine Verleihart, die es nur in diesem Test gibt (Spezifikation 3.5, AK-20).
// Sie zeigt: Eine neue Verleihart braucht nur eine Implementierung von plugin.Plugin.
type testgeraet struct{}

type flatRate struct{ amount domain.Money }

func (f flatRate) Quote(domain.Item, domain.Period) domain.Price {
	return domain.Price{Amount: f.amount, Explanation: "Pauschale"}
}
func (f flatRate) Describe(string) string { return f.amount.String() + " pauschal" }

func (testgeraet) Kind() plugin.Kind {
	return plugin.Kind{Key: "testgeraet", Singular: "Testgerät", Plural: "Testgeräte"}
}
func (testgeraet) Categories() []domain.Category {
	return []domain.Category{{Key: "standard", Label: "Standard"}}
}
func (testgeraet) Fields() []domain.Field {
	return []domain.Field{{Key: "seriennummer", Label: "Seriennummer", Type: domain.FieldText, Required: true}}
}
func (testgeraet) Validate(domain.Item) domain.ValidationErrors { return nil }
func (testgeraet) DisplayName(it domain.Item) string            { return "Gerät " + it.Attr("seriennummer") }
func (testgeraet) Pricing() plugin.PricingStrategy              { return flatRate{amount: domain.Euro(7)} }
func (testgeraet) Limits() domain.BookingLimits                 { return domain.BookingLimits{} }

// AK-20: Das Framework funktioniert mit einer Verleihart, die nur im Test existiert.
func TestFrameworkWithTestPlugin(t *testing.T) {
	s, c := newService(t, testgeraet{})
	_, err := s.CreateItem(service.ItemInput{Kind: "testgeraet", Category: "standard"})
	assertField(t, err, "attr_seriennummer", "Seriennummer ist ein Pflichtfeld")

	it, err := s.CreateItem(service.ItemInput{Kind: "testgeraet", Category: "standard", Attributes: map[string]string{"seriennummer": "SN-1"}})
	if err != nil {
		t.Fatal(err)
	}
	person := addPerson(t, s)
	// Ohne Mindestdauer sind auch 10 Minuten erlaubt – die Grenzen kommen allein aus dem Plugin.
	b := mustBook(t, s, in(it.ID, person, at(10, 0), at(10, 10)))
	if b.Price.Amount != domain.Euro(7) || b.Price.Explanation != "Pauschale" {
		t.Errorf("Preis aus der Test-Strategie: %v", b.Price)
	}
	_, err = s.Book(in(it.ID, person, at(10, 5), at(11, 0)))
	assertErr(t, err, domain.ErrItemBooked)

	res := mustBook(t, s, in(it.ID, person, at(12, 0), at(13, 0)))
	if err := s.Cancel(res.ID); err != nil {
		t.Fatal(err)
	}
	c.set(at(10, 5))
	if _, err := s.Return(b.ID); err != nil {
		t.Fatal(err)
	}
	st, _ := s.Stats("testgeraet")
	if st.Total != 1 || st.Available != 1 || st.Kind.Plural != "Testgeräte" {
		t.Errorf("Statistik: %+v", st)
	}
	if v, _ := s.Item(it.ID); v.Name != "Gerät SN-1" {
		t.Errorf("Anzeigename aus dem Plugin: %q", v.Name)
	}
	checkInvariants(t, s)
}

// AK-21: Fahrrad und Ski im selben Service, unterschiedliche Abrechnung, getrennte Belegung.
func TestTwoRentalKindsSideBySide(t *testing.T) {
	bikes, err := fahrrad.New(fahrrad.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	s, _ := newService(t, bikes, ski.New(domain.Euro(20), testLoc))
	city := addBike(t, s, fahrrad.Citybike)
	skiItem, err := s.CreateItem(service.ItemInput{Kind: ski.Key, Category: "carving", Attributes: map[string]string{
		ski.AttrBrand: "Atomic", ski.AttrModel: "Redster", ski.AttrLength: "170",
	}})
	if err != nil {
		t.Fatal(err)
	}
	person := addPerson(t, s)
	start := at(10, 0)
	end := start.Add(48 * time.Hour)

	bikeQuote, err := s.Quote(in(city, person, start, end))
	if err != nil {
		t.Fatal(err)
	}
	skiQuote, err := s.Quote(in(skiItem.ID, person, start, end))
	if err != nil {
		t.Fatal(err)
	}
	if bikeQuote.Price.Amount != domain.Euro(240) || skiQuote.Price.Amount != domain.Euro(60) {
		t.Errorf("Citybike %v (want 240 €), Ski %v (want 60 €)", bikeQuote.Price.Amount, skiQuote.Price.Amount)
	}
	if skiQuote.Price.Explanation != "3 angefangene Kalendertage × 20,00 €" {
		t.Errorf("Ski-Erklärung: %q", skiQuote.Price.Explanation)
	}

	mustBook(t, s, in(skiItem.ID, person, start, end))
	mustBook(t, s, in(city, person, start, end)) // Ski blockiert das Fahrrad nicht

	bikeStats, _ := s.Stats(fahrrad.Key)
	skiStats, _ := s.Stats(ski.Key)
	if bikeStats.Total != 1 || skiStats.Total != 1 || bikeStats.Reserved != 1 || skiStats.Reserved != 1 {
		t.Errorf("Statistik je Verleihart: Fahrrad %+v, Ski %+v", bikeStats, skiStats)
	}
	if v, _ := s.Item(skiItem.ID); v.Name != "Atomic Redster (170 cm)" || v.Kind.Singular != "Ski" {
		t.Errorf("Ski-Ansicht: %q %q", v.Name, v.Kind.Singular)
	}
	// Ski haben keine Mindestdauer, Fahrräder schon: Grenzen kommen je Verleihart aus dem Plugin.
	if _, err := s.Quote(in(skiItem.ID, person, end, end.Add(30*time.Minute))); err != nil {
		t.Errorf("Ski 30 Minuten: %v", err)
	}
	_, err = s.Quote(in(city, person, end, end.Add(30*time.Minute)))
	assertField(t, err, "end", "Verleihdauer muss mindestens 1 Stunde betragen")
	checkInvariants(t, s)
}
