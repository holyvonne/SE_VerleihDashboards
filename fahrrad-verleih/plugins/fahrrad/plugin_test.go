package fahrrad

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/plugin"
)

var loc = time.FixedZone("TEST", 3600)

func at(day, h, m int) time.Time { return time.Date(2026, 11, day, h, m, 0, 0, loc) }

func mustNew(t *testing.T, cfg Config) *Plugin {
	t.Helper()
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// AK-7: Beispiele aus Spezifikation 4.6.
func TestHourlyTariffExamples(t *testing.T) {
	p := mustNew(t, DefaultConfig())
	cases := []struct {
		category   string
		start, end time.Time
		hours      int
		price      domain.Money
	}{
		{Citybike, at(10, 10, 0), at(10, 11, 0), 1, domain.Euro(5)},
		{Citybike, at(10, 10, 0), at(10, 11, 1), 2, domain.Euro(10)},
		{Citybike, at(10, 10, 0), at(10, 12, 15), 3, domain.Euro(15)}, // 2 Std 15 Min → 3 Std (A-2)
		{Mountainbike, at(10, 9, 0), at(10, 12, 30), 4, domain.Euro(32)},
		{EBike, at(10, 9, 0), at(10, 17, 0), 8, domain.Euro(96)},
		{EBike, at(2, 18, 0), at(3, 10, 0), 16, domain.Euro(192)},
		{Citybike, at(10, 10, 0), at(10, 10, 20), 1, domain.Euro(5)}, // Mindestabrechnung
	}
	for _, c := range cases {
		item := domain.Item{Category: c.category}
		got := p.Pricing().Quote(item, domain.Period{Start: c.start, End: c.end})
		if got.Amount != c.price {
			t.Errorf("%s %s–%s: %v, want %v", c.category, c.start.Format("02.01. 15:04"), c.end.Format("15:04"), got.Amount, c.price)
		}
		if !strings.HasPrefix(got.Explanation, strconv.Itoa(c.hours)+" angefangene") {
			t.Errorf("Erklärung %q passt nicht zu %d Stunden", got.Explanation, c.hours)
		}
	}
}

func TestExplanationText(t *testing.T) {
	p := mustNew(t, DefaultConfig())
	got := p.Pricing().Quote(domain.Item{Category: Mountainbike}, domain.Period{Start: at(10, 9, 0), End: at(10, 11, 30)})
	if got.Explanation != "3 angefangene Stunden × 8,00 €" {
		t.Errorf("Erklärung: %q", got.Explanation)
	}
	one := p.Pricing().Quote(domain.Item{Category: EBike}, domain.Period{Start: at(10, 9, 0), End: at(10, 10, 0)})
	if one.Explanation != "1 angefangene Stunde × 12,00 €" {
		t.Errorf("Einzahl: %q", one.Explanation)
	}
	if d := p.Pricing().Describe(Citybike); d != "5,00 € pro angefangene Stunde, mindestens 1 Stunde" {
		t.Errorf("Describe: %q", d)
	}
}

// AK-7: konfigurierte Sätze ändern den Preis.
func TestConfiguredRates(t *testing.T) {
	p := mustNew(t, DefaultConfig().WithRates(map[string]domain.Money{Citybike: domain.Euro(6)}))
	got := p.Pricing().Quote(domain.Item{Category: Citybike}, domain.Period{Start: at(10, 10, 0), End: at(10, 11, 1)})
	if got.Amount != domain.Euro(12) {
		t.Errorf("Citybike 6 €: %v", got.Amount)
	}
	if p.Rate(EBike) != domain.Euro(12) {
		t.Errorf("nicht genannte Typen behalten den Standard: %v", p.Rate(EBike))
	}
}

// Spezifikation 4.6: Gerechnet wird die tatsächlich vergangene Zeit, auch über die Zeitumstellung.
func TestHourlyTariffAcrossDST(t *testing.T) {
	vienna, err := time.LoadLocation("Europe/Vienna")
	if err != nil {
		t.Skip("Zeitzonendaten fehlen")
	}
	p := mustNew(t, DefaultConfig())
	// 25.10.2026: Die Uhr wird um 03:00 auf 02:00 zurückgestellt – 00:00 bis 04:00 sind 5 echte Stunden.
	start := time.Date(2026, 10, 25, 0, 0, 0, 0, vienna)
	end := time.Date(2026, 10, 25, 4, 0, 0, 0, vienna)
	if got := p.Pricing().Quote(domain.Item{Category: Citybike}, domain.Period{Start: start, End: end}); got.Amount != domain.Euro(25) {
		t.Errorf("Winterzeit: %v, want 25,00 € (5 Std)", got.Amount)
	}
	// 29.03.2026: Die Uhr springt von 02:00 auf 03:00 – 00:00 bis 04:00 sind 3 echte Stunden.
	start = time.Date(2026, 3, 29, 0, 0, 0, 0, vienna)
	end = time.Date(2026, 3, 29, 4, 0, 0, 0, vienna)
	if got := p.Pricing().Quote(domain.Item{Category: Citybike}, domain.Period{Start: start, End: end}); got.Amount != domain.Euro(15) {
		t.Errorf("Sommerzeit: %v, want 15,00 € (3 Std)", got.Amount)
	}
}

func TestStartedHours(t *testing.T) {
	cases := map[time.Duration]int{0: 1, time.Minute: 1, time.Hour: 1, time.Hour + time.Second: 2, 2*time.Hour + 15*time.Minute: 3, 48 * time.Hour: 48}
	for d, want := range cases {
		if got := StartedHours(d); got != want {
			t.Errorf("StartedHours(%v) = %d, want %d", d, got, want)
		}
	}
}

// AK-18: Flag -tarife.
func TestParseRates(t *testing.T) {
	rates, err := ParseRates("citybike=6, mountainbike=9.50,EBIKE=14")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]domain.Money{Citybike: 600, Mountainbike: 950, EBike: 1400}
	for k, v := range want {
		if rates[k] != v {
			t.Errorf("%s = %v, want %v", k, rates[k], v)
		}
	}
	if r, err := ParseRates(""); err != nil || len(r) != 0 {
		t.Errorf("leer: %v %v", r, err)
	}
	for _, bad := range []string{"ebike=0", "roller=5", "citybike=abc", "citybike", "citybike=101", "=5"} {
		if _, err := ParseRates(bad); err == nil {
			t.Errorf("ParseRates(%q) sollte fehlschlagen", bad)
		}
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := DefaultConfig()
	delete(cfg.Rates, EBike)
	if _, err := New(cfg); err == nil {
		t.Error("fehlender Satz muss abgelehnt werden")
	}
	cfg = DefaultConfig()
	cfg.MinDuration = cfg.MaxDuration
	if _, err := New(cfg); err == nil {
		t.Error("Mindestdauer ≥ Höchstdauer muss abgelehnt werden")
	}
	cfg = DefaultConfig().WithRates(map[string]domain.Money{"roller": 100})
	if _, err := New(cfg); err == nil {
		t.Error("unbekannter Typ muss abgelehnt werden")
	}
}

// AK-3: Akku-Regel über zwei Felder.
func TestValidateBattery(t *testing.T) {
	p := mustNew(t, DefaultConfig())
	key := "attr_" + AttrBattery
	cases := []struct {
		category, battery, want string
	}{
		{EBike, "", MsgBatteryRequired},
		{EBike, "625", ""},
		{Citybike, "400", MsgBatteryForbidden},
		{Mountainbike, "", ""},
	}
	for _, c := range cases {
		it := domain.Item{Category: c.category, Attributes: map[string]string{AttrBattery: c.battery}}
		if got := p.Validate(it)[key]; got != c.want {
			t.Errorf("%s mit Akku %q: %q, want %q", c.category, c.battery, got, c.want)
		}
	}
}

func TestPluginContract(t *testing.T) {
	p := mustNew(t, DefaultConfig())
	if err := plugin.NewRegistry().Register(p); err != nil {
		t.Fatalf("Fahrrad-Plugin muss registrierbar sein: %v", err)
	}
	if p.Kind().Key != Key || len(p.Categories()) != 3 || len(p.Fields()) != 4 {
		t.Errorf("Kind/Kategorien/Merkmale: %v %d %d", p.Kind(), len(p.Categories()), len(p.Fields()))
	}
	l := p.Limits()
	if l.MinDuration != time.Hour || l.MaxDuration != 30*24*time.Hour {
		t.Errorf("Grenzen: %+v", l)
	}
	name := p.DisplayName(domain.Item{Attributes: map[string]string{AttrBrand: "Cube", AttrModel: "Kathmandu"}})
	if name != "Cube Kathmandu" {
		t.Errorf("DisplayName: %q", name)
	}
}
