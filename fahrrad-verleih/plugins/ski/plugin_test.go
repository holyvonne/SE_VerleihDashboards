package ski

import (
	"testing"
	"time"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/plugin"
)

// Regel aus Aufgabe 1b: 20 € pro angefangenem Kalendertag, mindestens 20 €.
func TestDailyRate(t *testing.T) {
	loc := time.FixedZone("TEST", 3600)
	d := func(day, h, m int) time.Time { return time.Date(2026, 11, day, h, m, 0, 0, loc) }
	p := New(domain.Euro(20), loc)
	cases := []struct {
		start, end time.Time
		want       domain.Money
	}{
		{d(2, 10, 0), d(2, 10, 30), domain.Euro(20)},
		{d(2, 22, 0), d(3, 8, 0), domain.Euro(40)},
		{d(2, 10, 0), d(3, 0, 0), domain.Euro(20)},
		{d(2, 10, 0), d(5, 10, 0), domain.Euro(80)},
		{d(10, 10, 0), d(12, 10, 0), domain.Euro(60)},
	}
	for _, c := range cases {
		got := p.Pricing().Quote(domain.Item{}, domain.Period{Start: c.start, End: c.end})
		if got.Amount != c.want {
			t.Errorf("%s – %s: %v, want %v", c.start.Format("02.01. 15:04"), c.end.Format("02.01. 15:04"), got.Amount, c.want)
		}
	}
}

func TestDailyRateAcrossDST(t *testing.T) {
	vienna, err := time.LoadLocation("Europe/Vienna")
	if err != nil {
		t.Skip("Zeitzonendaten fehlen")
	}
	// 25.10.2026: Umstellung auf Winterzeit, der Tag hat 25 Stunden.
	start := time.Date(2026, 10, 24, 12, 0, 0, 0, vienna)
	end := time.Date(2026, 10, 26, 12, 0, 0, 0, vienna)
	if n := CalendarDays(domain.Period{Start: start, End: end}, vienna); n != 3 {
		t.Errorf("Kalendertage über die Zeitumstellung: %d, want 3", n)
	}
}

func TestSkiPluginContract(t *testing.T) {
	p := New(domain.Euro(20), nil)
	if err := plugin.NewRegistry().Register(p); err != nil {
		t.Fatalf("Ski-Plugin muss registrierbar sein: %v", err)
	}
	it := domain.Item{Attributes: map[string]string{AttrBrand: "Atomic", AttrModel: "Redster", AttrLength: "170"}}
	if got := p.DisplayName(it); got != "Atomic Redster (170 cm)" {
		t.Errorf("DisplayName: %q", got)
	}
	if p.Limits().MaxDuration != 60*24*time.Hour || p.Limits().MinDuration != 0 {
		t.Errorf("Grenzen: %+v", p.Limits())
	}
}
