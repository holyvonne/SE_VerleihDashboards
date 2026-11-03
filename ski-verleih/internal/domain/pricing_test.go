package domain

import (
	"testing"
	"time"
)

func at(day, hour, minute int) time.Time {
	return time.Date(2026, 11, day, hour, minute, 0, 0, testLoc)
}

// AK-12: Beispiele aus Spezifikation 3.7.
func TestDailyRate_SpecExamples(t *testing.T) {
	tests := []struct {
		name      string
		p         Period
		wantDays  int
		wantPrice Money
	}{
		{"30 Minuten (Mindestpreis)", Period{at(2, 10, 0), at(2, 10, 30)}, 1, Euro(20)},
		{"gleicher Tag", Period{at(2, 10, 0), at(2, 17, 0)}, 1, Euro(20)},
		{"über Mitternacht, < 24 h", Period{at(2, 22, 0), at(3, 8, 0)}, 2, Euro(40)},
		{"Rückgabe genau 00:00", Period{at(2, 10, 0), at(3, 0, 0)}, 1, Euro(20)},
		{"drei Tage später, gleiche Uhrzeit", Period{at(2, 10, 0), at(5, 10, 0)}, 4, Euro(80)},
		{"ganzer Kalendertag", Period{at(2, 0, 0), at(3, 0, 0)}, 1, Euro(20)},
		{"eine Minute über Mitternacht", Period{at(2, 0, 0), at(3, 0, 1)}, 2, Euro(40)},
		{"Monatswechsel", Period{at(30, 9, 0), time.Date(2026, 12, 2, 9, 0, 0, 0, testLoc)}, 3, Euro(60)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := testPricing.Days(tt.p); got != tt.wantDays {
				t.Errorf("Days = %d, want %d", got, tt.wantDays)
			}
			if got := testPricing.Price(tt.p); got != tt.wantPrice {
				t.Errorf("Price = %v, want %v", got, tt.wantPrice)
			}
		})
	}
}

// Kalendertage richten sich nach der Zeitzone der Preisregel, nicht nach der des Zeitpunkts.
func TestCalendarDays_UsesPolicyLocation(t *testing.T) {
	// 23:30–00:30 Testzeit = 22:30–23:30 UTC: in Testzeit 2 Tage, in UTC 1 Tag.
	p := Period{at(2, 23, 30), at(3, 0, 30)}
	if got := CalendarDays(p, testLoc); got != 2 {
		t.Errorf("Testzeit: %d, want 2", got)
	}
	if got := CalendarDays(Period{p.Start.UTC(), p.End.UTC()}, time.UTC); got != 1 {
		t.Errorf("UTC: %d, want 1", got)
	}
}

// Sommerzeitwechsel (Europa/Wien, 29.03.2026) darf keinen Tag verschlucken.
func TestCalendarDays_DST(t *testing.T) {
	vienna, err := time.LoadLocation("Europe/Vienna")
	if err != nil {
		t.Skip("Zeitzonendatenbank nicht verfügbar")
	}
	p := Period{time.Date(2026, 3, 28, 12, 0, 0, 0, vienna), time.Date(2026, 3, 30, 12, 0, 0, 0, vienna)}
	if got := CalendarDays(p, vienna); got != 3 {
		t.Errorf("über Zeitumstellung: %d, want 3", got)
	}
}

func TestDailyRate_ExplainAndRule(t *testing.T) {
	if got := testPricing.Explain(Period{at(2, 10, 0), at(2, 12, 0)}); got != "1 angefangener Kalendertag × 20,00 €" {
		t.Errorf("Explain(1) = %q", got)
	}
	if got := testPricing.Explain(Period{at(2, 10, 0), at(4, 12, 0)}); got != "3 angefangene Kalendertage × 20,00 €" {
		t.Errorf("Explain(3) = %q", got)
	}
	if got := testPricing.Rule(); got != "20,00 € pro angefangenem Kalendertag, mindestens 20,00 €" {
		t.Errorf("Rule = %q", got)
	}
}

// F-6.6: deutsches Geldformat.
func TestMoneyString(t *testing.T) {
	cases := map[Money]string{
		0:             "0,00 €",
		5:             "0,05 €",
		Euro(20):      "20,00 €",
		Money(12345):  "123,45 €",
		Euro(1240):    "1.240,00 €",
		Euro(1234567): "1.234.567,00 €",
		-Euro(20):     "-20,00 €",
	}
	for m, want := range cases {
		if got := m.String(); got != want {
			t.Errorf("Money(%d) = %q, want %q", int64(m), got, want)
		}
	}
}

// Die Standardregel entspricht 1b: 20 € pro Kalendertag.
func TestStandardPricing(t *testing.T) {
	if StandardPricing().PerDay != Euro(20) {
		t.Error("Standardpreis ist nicht 20 €")
	}
	var _ PricingPolicy = DailyRate{} // DailyRate erfüllt die Schnittstelle
}
