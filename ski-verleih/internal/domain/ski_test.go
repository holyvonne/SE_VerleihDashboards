package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func validSki() Ski {
	return NewSki("Gipfelwerk", "Allmountain 80", 170, SkiTypeAllround, ConditionGood, "")
}

func TestNewSki_IsAvailableAndTrimmed(t *testing.T) {
	s := NewSki("  Gipfelwerk ", " Pro ", 160, SkiTypeCarving, ConditionNew, "  ")
	if s.Status != SkiAvailable {
		t.Errorf("Status = %q, want %q", s.Status, SkiAvailable)
	}
	if s.Brand != "Gipfelwerk" || s.Model != "Pro" || s.Note != "" {
		t.Errorf("Textfelder nicht getrimmt: %+v", s)
	}
}

func TestSkiValidate(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Ski)
		wantField string // "" = gültig
	}{
		{"gültig", func(*Ski) {}, ""},
		{"Länge Untergrenze", func(s *Ski) { s.LengthCM = MinSkiLengthCM }, ""},
		{"Länge Obergrenze", func(s *Ski) { s.LengthCM = MaxSkiLengthCM }, ""},
		{"AK-3 Länge 300", func(s *Ski) { s.LengthCM = 300 }, "length"},
		{"Länge zu kurz", func(s *Ski) { s.LengthCM = 69 }, "length"},
		{"Marke leer", func(s *Ski) { s.Brand = "" }, "brand"},
		{"Marke nur Leerzeichen", func(s *Ski) { s.Brand = "   " }, "brand"},
		{"Marke 51 Zeichen", func(s *Ski) { s.Brand = strings.Repeat("a", 51) }, "brand"},
		{"Marke 50 Umlaute", func(s *Ski) { s.Brand = strings.Repeat("ä", 50) }, ""},
		{"Modell leer", func(s *Ski) { s.Model = "" }, "model"},
		{"Typ unbekannt", func(s *Ski) { s.Type = "snowboard" }, "type"},
		{"Zustand unbekannt", func(s *Ski) { s.Condition = "kaputt" }, "condition"},
		{"Status unbekannt", func(s *Ski) { s.Status = "weg" }, "status"},
		{"ausgeliehen wird nicht gespeichert", func(s *Ski) { s.Status = SkiRented }, "status"},
		{"Notiz 201 Zeichen", func(s *Ski) { s.Note = strings.Repeat("x", 201) }, "note"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validSki()
			tt.mutate(&s)
			err := s.Validate()
			assertFieldError(t, err, tt.wantField)
		})
	}
}

func TestSkiValidate_LengthMessage(t *testing.T) {
	s := validSki()
	s.LengthCM = 300
	var verr ValidationErrors
	if !errors.As(s.Validate(), &verr) {
		t.Fatal("ValidationErrors erwartet")
	}
	if got, want := verr["length"], "Länge muss zwischen 70 und 220 cm liegen"; got != want {
		t.Errorf("Meldung = %q, want %q", got, want)
	}
}

// F-2.4, AK-4, AK-21: manueller Statuswechsel nur verfügbar ↔ wartung.
// Ob der Ski gebucht ist, prüft der Service.
func TestSkiSetManualStatus(t *testing.T) {
	s := validSki()
	if err := s.SetManualStatus(SkiMaintenance); err != nil || s.Status != SkiMaintenance {
		t.Fatalf("err = %v, Status = %q", err, s.Status)
	}
	if err := s.SetManualStatus(SkiAvailable); err != nil || s.Status != SkiAvailable {
		t.Fatalf("err = %v, Status = %q", err, s.Status)
	}
	assertFieldError(t, s.SetManualStatus(SkiRented), "status")
	if s.Status != SkiAvailable {
		t.Errorf("Status verändert: %q", s.Status)
	}
}

// 3.1, I-1, AK-15: angezeigter Status wird aus Grundstatus und Buchungen abgeleitet.
func TestSkiStatusAt(t *testing.T) {
	s := validSki()
	s.ID = 1
	start := t0.Add(48 * time.Hour)
	reservation := NewRental(1, 9, Period{Start: start, End: start.Add(24 * time.Hour)}, t0, Euro(40))
	otherSki := NewRental(2, 9, Period{Start: t0, End: t0.Add(time.Hour)}, t0, Euro(20))
	rentals := []Rental{reservation, otherSki}

	if got := s.StatusAt(rentals, t0); got != SkiAvailable {
		t.Errorf("vor der Reservierung: %q, want verfuegbar", got)
	}
	if got := s.StatusAt(rentals, start); got != SkiRented {
		t.Errorf("bei Beginn: %q, want ausgeliehen", got)
	}
	// Auch überfällig bleibt der Ski ausgeliehen, bis er zurück ist.
	if got := s.StatusAt(rentals, start.Add(72*time.Hour)); got != SkiRented {
		t.Errorf("überfällig: %q, want ausgeliehen", got)
	}
	_ = rentals[0].Return(start.Add(time.Hour), StandardPricing())
	if got := s.StatusAt(rentals, start.Add(2*time.Hour)); got != SkiAvailable {
		t.Errorf("nach Rückgabe: %q, want verfuegbar", got)
	}
	s.Status = SkiMaintenance
	if got := s.StatusAt(nil, t0); got != SkiMaintenance {
		t.Errorf("Wartung: %q", got)
	}
}

func TestSkiName(t *testing.T) {
	if got, want := validSki().Name(), "Gipfelwerk Allmountain 80 (170 cm)"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestLabels(t *testing.T) {
	if SkiTypeTouring.Label() != "Touren" || SkiAvailable.Label() != "Verfügbar" ||
		ConditionUsed.Label() != "Gebraucht" || IDDriverLicense.Label() != "Führerschein" {
		t.Error("unerwartetes Label")
	}
	if SkiType("xyz").Label() != "xyz" {
		t.Error("unbekannter Wert soll als Rohwert angezeigt werden")
	}
}

// assertFieldError prüft, dass err genau dann ein Fehler für wantField ist,
// wenn wantField nicht leer ist.
func assertFieldError(t *testing.T, err error, wantField string) {
	t.Helper()
	if wantField == "" {
		if err != nil {
			t.Fatalf("unerwarteter Fehler: %v", err)
		}
		return
	}
	var verr ValidationErrors
	if !errors.As(err, &verr) {
		t.Fatalf("err = %v, want ValidationErrors mit Feld %q", err, wantField)
	}
	if _, ok := verr[wantField]; !ok {
		t.Fatalf("Feld %q fehlt in %v", wantField, verr)
	}
}
