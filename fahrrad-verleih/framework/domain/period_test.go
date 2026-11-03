package domain

import (
	"testing"
	"time"
)

var testLoc = time.FixedZone("TEST", 3600)

// at liefert 10.11.2026 h:m in der Testzeitzone.
func at(h, m int) time.Time { return time.Date(2026, 11, 10, h, m, 0, 0, testLoc) }

// AK-11: Überschneidungen am Beispiel [10:00, 12:00).
func TestPeriodOverlaps(t *testing.T) {
	booked := Period{Start: at(10, 0), End: at(12, 0)}
	cases := []struct {
		name string
		p    Period
		want bool
	}{
		{"teilweise danach", Period{at(11, 0), at(13, 0)}, true},
		{"umschließend", Period{at(9, 0), at(14, 0)}, true},
		{"enthalten", Period{at(10, 30), at(11, 30)}, true},
		{"identisch", booked, true},
		{"angrenzend danach", Period{at(12, 0), at(13, 0)}, false},
		{"angrenzend davor", Period{at(9, 0), at(10, 0)}, false},
		{"offen ab vorher", Period{Start: at(8, 0)}, true},
		{"offen ab danach", Period{Start: at(12, 0)}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := booked.Overlaps(c.p); got != c.want {
				t.Errorf("Overlaps = %v, want %v", got, c.want)
			}
			if got := c.p.Overlaps(booked); got != c.want {
				t.Errorf("symmetrisch: Overlaps = %v, want %v", got, c.want)
			}
		})
	}
}

// AK-15: Eingaberegeln mit den Grenzen des Fahrrad-Plugins (1 Stunde bis 30 Tage).
func TestValidatePeriod(t *testing.T) {
	limits := BookingLimits{MinDuration: time.Hour, MaxDuration: 30 * 24 * time.Hour}
	now := at(10, 0).Add(42 * time.Second) // mitten in der Minute 10:00
	cases := []struct {
		name  string
		p     Period
		field string
		msg   string
	}{
		{"gültig", Period{at(11, 0), at(13, 0)}, "", ""},
		{"aktuelle Minute erlaubt", Period{at(10, 0), at(11, 0)}, "", ""},
		{"genau 30 Tage", Period{at(11, 0), at(11, 0).Add(30 * 24 * time.Hour)}, "", ""},
		{"Beginn fehlt", Period{End: at(12, 0)}, "start", MsgMissingTime},
		{"Ende fehlt", Period{Start: at(12, 0)}, "end", MsgMissingTime},
		{"Vergangenheit", Period{at(9, 59), at(12, 0)}, "start", MsgStartPast},
		{"Ende gleich Beginn", Period{at(11, 0), at(11, 0)}, "end", MsgEndBefore},
		{"Ende vor Beginn", Period{at(12, 0), at(11, 0)}, "end", MsgEndBefore},
		{"59 Minuten", Period{at(11, 0), at(11, 59)}, "end", "Verleihdauer muss mindestens 1 Stunde betragen"},
		{"30 Tage + 1 Minute", Period{at(11, 0), at(11, 1).Add(30 * 24 * time.Hour)}, "end", "Verleihdauer darf höchstens 30 Tage betragen"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			errs := ValidatePeriod(c.p, now, limits)
			if c.field == "" {
				if len(errs) != 0 {
					t.Fatalf("unerwartete Fehler: %v", errs)
				}
				return
			}
			if errs[c.field] != c.msg {
				t.Errorf("errs[%q] = %q, want %q (alle: %v)", c.field, errs[c.field], c.msg, errs)
			}
		})
	}
}

func TestValidatePeriodWithoutLimits(t *testing.T) {
	if errs := ValidatePeriod(Period{at(11, 0), at(11, 5)}, at(10, 0), BookingLimits{}); len(errs) != 0 {
		t.Errorf("ohne Grenzen erwartet keine Fehler, got %v", errs)
	}
}

func TestFormatLimit(t *testing.T) {
	cases := map[time.Duration]string{
		time.Hour:           "1 Stunde",
		3 * time.Hour:       "3 Stunden",
		24 * time.Hour:      "1 Tag",
		30 * 24 * time.Hour: "30 Tage",
		90 * time.Minute:    "90 Minuten",
	}
	for d, want := range cases {
		if got := FormatLimit(d); got != want {
			t.Errorf("FormatLimit(%v) = %q, want %q", d, got, want)
		}
	}
}
