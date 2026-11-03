package domain

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Category ist ein Untertyp einer Verleihart, z. B. „Standard“ oder „Premium“ (Spezifikation 3.2).
type Category struct {
	Key   string
	Label string
}

// FieldType bestimmt, wie ein Merkmal geprüft und im Formular dargestellt wird.
type FieldType string

const (
	FieldText   FieldType = "text"
	FieldNumber FieldType = "number"
	FieldChoice FieldType = "choice"
)

// Option ist ein erlaubter Wert eines Auswahl-Merkmals.
type Option struct {
	Key   string
	Label string
}

// Field beschreibt ein Merkmal, das ein Plugin für seine Objekte festlegt.
// Das Framework prüft Werte allein anhand dieser Beschreibung.
type Field struct {
	Key      string
	Label    string
	Type     FieldType
	Required bool
	MaxLen   int      // nur Text; 0 = 100
	Min, Max int      // nur Zahl
	Options  []Option // nur Auswahl
	Unit     string   // z. B. „Wh“, nur zur Anzeige
	Hint     string   // Hilfetext im Formular
}

// FormKey ist der Name des Formularfelds und der Schlüssel für Feldfehler.
func (f Field) FormKey() string { return "attr_" + f.Key }

// OptionLabel liefert die Beschriftung eines Auswahlwerts.
func (f Field) OptionLabel(value string) string {
	for _, o := range f.Options {
		if o.Key == value {
			return o.Label
		}
	}
	return value
}

// Display formatiert einen gespeicherten Wert für die Anzeige, z. B. „625 Wh“.
func (f Field) Display(value string) string {
	switch {
	case value == "":
		return ""
	case f.Type == FieldChoice:
		return f.OptionLabel(value)
	case f.Unit != "":
		return value + " " + f.Unit
	default:
		return value
	}
}

// check prüft einen Wert und gibt ihn normalisiert zurück (Spezifikation 3.2, Stufe 2).
func (f Field) check(raw string) (string, string) {
	v := strings.TrimSpace(raw)
	if v == "" {
		if f.Required {
			return "", f.Label + " ist ein Pflichtfeld"
		}
		return "", ""
	}
	switch f.Type {
	case FieldNumber:
		n, err := strconv.Atoi(v)
		if err != nil {
			return v, "Bitte eine ganze Zahl angeben"
		}
		if n < f.Min || n > f.Max {
			return v, fmt.Sprintf("Wert muss zwischen %d und %d liegen", f.Min, f.Max)
		}
		return strconv.Itoa(n), ""
	case FieldChoice:
		for _, o := range f.Options {
			if o.Key == v {
				return v, ""
			}
		}
		return v, "Bitte eine gültige Auswahl treffen"
	default:
		max := f.MaxLen
		if max == 0 {
			max = 100
		}
		if utf8.RuneCountInString(v) > max {
			return v, fmt.Sprintf("%s darf höchstens %d Zeichen lang sein", f.Label, max)
		}
		return v, ""
	}
}

// ValidateAttributes prüft alle Merkmale gegen ihre Beschreibung. Das Ergebnis enthält nur
// bekannte, nicht leere Werte; unbekannte Schlüssel werden verworfen. Fehler stehen unter Field.FormKey().
func ValidateAttributes(fields []Field, attrs map[string]string) (map[string]string, ValidationErrors) {
	clean := map[string]string{}
	errs := ValidationErrors{}
	for _, f := range fields {
		v, msg := f.check(attrs[f.Key])
		if msg != "" {
			errs.Add(f.FormKey(), msg)
		}
		if v != "" {
			clean[f.Key] = v
		}
	}
	return clean, errs
}
