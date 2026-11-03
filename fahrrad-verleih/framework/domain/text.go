package domain

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// requireText prüft ein Pflicht-Textfeld auf 1..max Zeichen (nach Trimmen).
func requireText(errs ValidationErrors, field, label, value string, max int) {
	switch {
	case strings.TrimSpace(value) == "":
		errs.Add(field, label+" ist ein Pflichtfeld")
	case utf8.RuneCountInString(value) > max:
		errs.Add(field, fmt.Sprintf("%s darf höchstens %d Zeichen lang sein", label, max))
	}
}

// optionalText prüft ein optionales Textfeld auf höchstens max Zeichen.
func optionalText(errs ValidationErrors, field, label, value string, max int) {
	if utf8.RuneCountInString(value) > max {
		errs.Add(field, fmt.Sprintf("%s darf höchstens %d Zeichen lang sein", label, max))
	}
}
