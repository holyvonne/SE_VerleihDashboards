package domain

import (
	"testing"
	"time"
)

// AK-16: bestehend [10.11. 10:00, 12.11. 10:00).
func TestPeriodOverlaps(t *testing.T) {
	existing := period(t0, 48*time.Hour)
	day := 24 * time.Hour
	tests := []struct {
		name string
		p    Period
		want bool
	}{
		{"teilweise (danach)", period(t0.Add(day), 2*day), true},
		{"teilweise (davor)", period(t0.Add(-day), 2*day), true},
		{"umschließend", period(t0.Add(-day), 5*day), true},
		{"enthalten", period(t0.Add(2*time.Hour), day), true},
		{"identisch", existing, true},
		{"angrenzend danach", period(t0.Add(2*day), 2*day), false},
		{"angrenzend davor", period(t0.Add(-2*day), 2*day), false},
		{"weit danach", period(t0.Add(10*day), day), false},
		{"offen ab davor", Period{Start: t0.Add(-day)}, true},
		{"offen ab Ende", Period{Start: existing.End}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := existing.Overlaps(tt.p); got != tt.want {
				t.Errorf("Overlaps = %v, want %v", got, tt.want)
			}
			if got := tt.p.Overlaps(existing); got != tt.want {
				t.Errorf("Overlaps ist nicht symmetrisch")
			}
		})
	}
	if !(Period{Start: t0}).Overlaps(Period{Start: t0.Add(day)}) {
		t.Error("zwei offene Zeiträume müssen sich überschneiden")
	}
}

// AK-19: Regeln für neue Buchungen.
func TestValidateBooking(t *testing.T) {
	now := t0
	day := 24 * time.Hour
	tests := []struct {
		name      string
		p         Period
		wantField string
		wantMsg   string
	}{
		{"ab jetzt", period(now, day), "", ""},
		{"Reservierung in der Zukunft", period(now.Add(30*day), day), "", ""},
		{"14 Minuten zurück (Toleranz)", period(now.Add(-14*time.Minute), day), "", ""},
		{"16 Minuten zurück", period(now.Add(-16*time.Minute), day), "start", "Beginn darf nicht in der Vergangenheit liegen"},
		{"Rückgabe gleich Beginn", period(now, 0), "end", "Rückgabe muss nach dem Beginn liegen"},
		{"Rückgabe vor Beginn", period(now, -time.Hour), "end", "Rückgabe muss nach dem Beginn liegen"},
		{"genau 60 Tage", period(now, 60*day), "", ""},
		{"61 Tage (AK-12)", period(now, 61*day), "end", "Verleihdauer darf höchstens 60 Tage betragen"},
		{"Beginn fehlt", Period{End: now.Add(day)}, "start", "Bitte Datum und Uhrzeit angeben"},
		{"Ende fehlt", Period{Start: now}, "end", "Bitte Datum und Uhrzeit angeben"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBooking(tt.p, now)
			assertFieldError(t, err, tt.wantField)
			if tt.wantField != "" {
				if got := err.(ValidationErrors)[tt.wantField]; got != tt.wantMsg {
					t.Errorf("Meldung = %q, want %q", got, tt.wantMsg)
				}
			}
		})
	}
}
