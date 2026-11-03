package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// Money ist ein Geldbetrag in ganzen Cent – nie float64.
type Money int64

// Euro erzeugt einen Betrag aus ganzen Euro.
func Euro(e int64) Money { return Money(e * 100) }

// String formatiert deutsch mit Tausenderpunkt, z. B. „1.240,00 €“ (F-9).
func (m Money) String() string {
	sign := ""
	if m < 0 {
		sign, m = "-", -m
	}
	euros := strconv.FormatInt(int64(m)/100, 10)
	for i := len(euros) - 3; i > 0; i -= 3 {
		euros = euros[:i] + "." + euros[i:]
	}
	return fmt.Sprintf("%s%s,%02d €", sign, euros, int64(m)%100)
}

// ParseMoney liest einen Euro-Betrag wie „6“, „9.5“, „9,50“ oder „12,05“.
// Mehr als zwei Nachkommastellen sind ein Fehler, damit nichts still gerundet wird.
func ParseMoney(s string) (Money, error) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "€"))
	s = strings.ReplaceAll(s, ",", ".")
	whole, frac, hasFrac := strings.Cut(s, ".")
	if whole == "" || len(frac) > 2 || (hasFrac && frac == "") {
		return 0, fmt.Errorf("ungültiger Betrag %q", s)
	}
	euros, err := strconv.ParseInt(whole, 10, 64)
	if err != nil || euros < 0 {
		return 0, fmt.Errorf("ungültiger Betrag %q", s)
	}
	cents := int64(0)
	if frac != "" {
		for len(frac) < 2 {
			frac += "0"
		}
		if cents, err = strconv.ParseInt(frac, 10, 64); err != nil || cents < 0 {
			return 0, fmt.Errorf("ungültiger Betrag %q", s)
		}
	}
	return Money(euros*100 + cents), nil
}

// Price ist das Ergebnis einer Preisstrategie: Betrag und nachvollziehbare Rechnung.
type Price struct {
	Amount      Money
	Explanation string // z. B. „3 angefangene Stunden × 8,00 €“
}
