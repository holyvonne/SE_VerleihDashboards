package domain

import (
	"fmt"
	"strings"
	"time"
)

// SkiType ist die Art des Ski (Spezifikation 3.1).
type SkiType string

const (
	SkiTypeAllround SkiType = "allround"
	SkiTypeCarving  SkiType = "carving"
	SkiTypeFreeride SkiType = "freeride"
	SkiTypeTouring  SkiType = "touren"
	SkiTypeKids     SkiType = "kinder"
)

// SkiTypes listet alle Typen in Anzeigereihenfolge (für Auswahllisten).
var SkiTypes = []SkiType{SkiTypeAllround, SkiTypeCarving, SkiTypeFreeride, SkiTypeTouring, SkiTypeKids}

var skiTypeLabels = map[SkiType]string{
	SkiTypeAllround: "Allround",
	SkiTypeCarving:  "Carving",
	SkiTypeFreeride: "Freeride",
	SkiTypeTouring:  "Touren",
	SkiTypeKids:     "Kinder",
}

func (t SkiType) Valid() bool   { _, ok := skiTypeLabels[t]; return ok }
func (t SkiType) Label() string { return labelOr(skiTypeLabels, t) }

// Condition ist der Zustand des Materials.
type Condition string

const (
	ConditionNew  Condition = "neu"
	ConditionGood Condition = "gut"
	ConditionUsed Condition = "gebraucht"
)

var Conditions = []Condition{ConditionNew, ConditionGood, ConditionUsed}

var conditionLabels = map[Condition]string{
	ConditionNew:  "Neu",
	ConditionGood: "Gut",
	ConditionUsed: "Gebraucht",
}

func (c Condition) Valid() bool   { _, ok := conditionLabels[c]; return ok }
func (c Condition) Label() string { return labelOr(conditionLabels, c) }

// SkiStatus beschreibt, ob ein Ski ausgeliehen werden kann.
// Gespeichert werden nur verfuegbar und wartung (Grundstatus);
// ausgeliehen wird aus den Buchungen abgeleitet (Spezifikation 3.1).
type SkiStatus string

const (
	SkiAvailable   SkiStatus = "verfuegbar"
	SkiRented      SkiStatus = "ausgeliehen"
	SkiMaintenance SkiStatus = "wartung"
)

var SkiStatuses = []SkiStatus{SkiAvailable, SkiRented, SkiMaintenance}

var skiStatusLabels = map[SkiStatus]string{
	SkiAvailable:   "Verfügbar",
	SkiRented:      "Ausgeliehen",
	SkiMaintenance: "Wartung",
}

func (s SkiStatus) Valid() bool   { _, ok := skiStatusLabels[s]; return ok }
func (s SkiStatus) Label() string { return labelOr(skiStatusLabels, s) }

// Längengrenzen in cm (Spezifikation 3.1).
const (
	MinSkiLengthCM = 70
	MaxSkiLengthCM = 220
)

// Ski ist ein einzelnes Paar Ski im Bestand.
type Ski struct {
	ID        int
	Brand     string
	Model     string
	LengthCM  int
	Type      SkiType
	Condition Condition
	Status    SkiStatus // Grundstatus: verfuegbar oder wartung
	Note      string
}

// NewSki erzeugt einen Ski im Status „verfügbar“ (neu angelegte Ski sind immer verfügbar).
func NewSki(brand, model string, lengthCM int, typ SkiType, cond Condition, note string) Ski {
	s := Ski{
		Brand:     brand,
		Model:     model,
		LengthCM:  lengthCM,
		Type:      typ,
		Condition: cond,
		Status:    SkiAvailable,
		Note:      note,
	}
	s.Normalize()
	return s
}

// Normalize entfernt führende und abschließende Leerzeichen aus allen Textfeldern.
func (s *Ski) Normalize() {
	s.Brand = strings.TrimSpace(s.Brand)
	s.Model = strings.TrimSpace(s.Model)
	s.Note = strings.TrimSpace(s.Note)
}

// Validate prüft die Feldregeln aus Spezifikation 3.1.
func (s Ski) Validate() error {
	errs := ValidationErrors{}
	requireText(errs, "brand", "Marke", s.Brand, 50)
	requireText(errs, "model", "Modell", s.Model, 50)
	if s.LengthCM < MinSkiLengthCM || s.LengthCM > MaxSkiLengthCM {
		errs.add("length", fmt.Sprintf("Länge muss zwischen %d und %d cm liegen", MinSkiLengthCM, MaxSkiLengthCM))
	}
	if !s.Type.Valid() {
		errs.add("type", "Bitte einen gültigen Ski-Typ wählen")
	}
	if !s.Condition.Valid() {
		errs.add("condition", "Bitte einen gültigen Zustand wählen")
	}
	if s.Status != SkiAvailable && s.Status != SkiMaintenance {
		errs.add("status", msgManualStatus)
	}
	optionalText(errs, "note", "Notiz", s.Note, 200)
	return errs.errOrNil()
}

// Name ist die Kurzbezeichnung für Listen, z. B. „Atomic Redster (170 cm)“.
func (s Ski) Name() string {
	return fmt.Sprintf("%s %s (%d cm)", s.Brand, s.Model, s.LengthCM)
}

const msgManualStatus = "Status kann nur „Verfügbar“ oder „Wartung“ sein"

// InMaintenance meldet den Grundstatus Wartung; solche Ski sind nicht buchbar (AK-9).
func (s Ski) InMaintenance() bool { return s.Status == SkiMaintenance }

// StatusAt leitet den angezeigten Status zum Zeitpunkt now ab (3.1, I-1):
// Wartung vor aktiver Ausleihe vor verfügbar. rentals sind die Buchungen dieses Ski.
// Eine Reservierung wird so zur Startzeit automatisch „ausgeliehen“ (AK-15).
func (s Ski) StatusAt(rentals []Rental, now time.Time) SkiStatus {
	if s.InMaintenance() {
		return SkiMaintenance
	}
	for _, r := range rentals {
		if r.SkiID == s.ID && r.IsActive(now) {
			return SkiRented
		}
	}
	return SkiAvailable
}

// SetManualStatus ist der Statuswechsel aus dem Bearbeiten-Formular (F-2.4):
// nur verfügbar ↔ wartung. Ob der Ski gerade gebucht ist, prüft der Service (AK-4, AK-21).
func (s *Ski) SetManualStatus(status SkiStatus) error {
	if status != SkiAvailable && status != SkiMaintenance {
		return ValidationErrors{"status": msgManualStatus}
	}
	s.Status = status
	return nil
}

// labelOr liefert das Anzeige-Label oder den Rohwert, falls unbekannt.
func labelOr[K ~string](labels map[K]string, k K) string {
	if l, ok := labels[k]; ok {
		return l
	}
	return string(k)
}
