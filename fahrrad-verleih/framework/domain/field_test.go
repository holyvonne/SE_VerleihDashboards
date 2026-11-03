package domain

import "testing"

var testFields = []Field{
	{Key: "name", Label: "Name", Type: FieldText, Required: true, MaxLen: 5},
	{Key: "size", Label: "Größe", Type: FieldChoice, Required: true, Options: []Option{{"S", "Klein"}, {"L", "Groß"}}},
	{Key: "power", Label: "Leistung", Type: FieldNumber, Min: 10, Max: 20, Unit: "W"},
}

// Spezifikation 3.2, Stufe 2: Das Framework prüft Merkmale nur anhand ihrer Beschreibung.
func TestValidateAttributes(t *testing.T) {
	clean, errs := ValidateAttributes(testFields, map[string]string{
		"name": "  Abc ", "size": "L", "power": " 015", "unbekannt": "weg",
	})
	if len(errs) != 0 {
		t.Fatalf("unerwartete Fehler: %v", errs)
	}
	want := map[string]string{"name": "Abc", "size": "L", "power": "15"}
	if len(clean) != len(want) {
		t.Fatalf("clean = %v, want %v (unbekannte Merkmale müssen verworfen werden)", clean, want)
	}
	for k, v := range want {
		if clean[k] != v {
			t.Errorf("clean[%q] = %q, want %q", k, clean[k], v)
		}
	}

	_, errs = ValidateAttributes(testFields, map[string]string{"name": "zu lang", "size": "XXL", "power": "9"})
	wantErrs := map[string]string{
		"attr_name":  "Name darf höchstens 5 Zeichen lang sein",
		"attr_size":  "Bitte eine gültige Auswahl treffen",
		"attr_power": "Wert muss zwischen 10 und 20 liegen",
	}
	for k, v := range wantErrs {
		if errs[k] != v {
			t.Errorf("errs[%q] = %q, want %q", k, errs[k], v)
		}
	}

	_, errs = ValidateAttributes(testFields, map[string]string{"power": "x"})
	if errs["attr_name"] != "Name ist ein Pflichtfeld" || errs["attr_size"] != "Größe ist ein Pflichtfeld" {
		t.Errorf("Pflichtfelder: %v", errs)
	}
	if errs["attr_power"] != "Bitte eine ganze Zahl angeben" {
		t.Errorf("Zahl: %v", errs)
	}
}

func TestFieldDisplay(t *testing.T) {
	if got := testFields[1].Display("S"); got != "Klein" {
		t.Errorf("Auswahl: %q", got)
	}
	if got := testFields[2].Display("15"); got != "15 W" {
		t.Errorf("Einheit: %q", got)
	}
	if got := testFields[2].Display(""); got != "" {
		t.Errorf("leer: %q", got)
	}
}
