package domain

import (
	"strings"
	"time"
)

// ItemStatus ist der angezeigte Status eines Objekts (Spezifikation 4.2).
// Gespeichert wird nur verfuegbar oder wartung; ausgeliehen wird abgeleitet.
type ItemStatus string

const (
	StatusAvailable   ItemStatus = "verfuegbar"
	StatusRented      ItemStatus = "ausgeliehen"
	StatusMaintenance ItemStatus = "wartung"
)

var itemStatusLabels = map[ItemStatus]string{
	StatusAvailable:   "Verfügbar",
	StatusRented:      "Ausgeliehen",
	StatusMaintenance: "Wartung",
}

// BaseStatuses sind die Werte, die gespeichert und manuell gewählt werden dürfen.
var BaseStatuses = []ItemStatus{StatusAvailable, StatusMaintenance}

func (s ItemStatus) Label() string {
	if l, ok := itemStatusLabels[s]; ok {
		return l
	}
	return string(s)
}

// IsBase meldet, ob s gespeichert werden darf.
func (s ItemStatus) IsBase() bool { return s == StatusAvailable || s == StatusMaintenance }

// Item ist ein Verleihobjekt beliebiger Verleihart. Was ein Objekt fachlich ausmacht,
// steht in Category und Attributes und wird vom Plugin beschrieben.
type Item struct {
	ID         int
	Kind       string
	Category   string
	Attributes map[string]string
	Status     ItemStatus // nur Grundstatus: verfuegbar oder wartung
	Note       string
	CreatedAt  time.Time
}

// Attr liefert ein Merkmal oder "".
func (it Item) Attr(key string) string { return it.Attributes[key] }

// InMaintenance meldet, ob der Grundstatus Wartung ist.
func (it Item) InMaintenance() bool { return it.Status == StatusMaintenance }

// StatusAt leitet den angezeigten Status ab (Spezifikation 4.2, I-1).
// bookings sind die Buchungen dieses Objekts.
func (it Item) StatusAt(bookings []Booking, now time.Time) ItemStatus {
	if it.InMaintenance() {
		return StatusMaintenance
	}
	for _, b := range bookings {
		if b.IsHandedOut(now) {
			return StatusRented
		}
	}
	return StatusAvailable
}

// ValidateBase prüft die Regeln, die für jedes Objekt gelten (Stufe 1 aus Spezifikation 3.2).
// Verleihart und Kategorie prüft der Service, weil er die Registry kennt.
func (it *Item) ValidateBase() ValidationErrors {
	errs := ValidationErrors{}
	it.Note = strings.TrimSpace(it.Note)
	optionalText(errs, "note", "Notiz", it.Note, 200)
	if !it.Status.IsBase() {
		errs.Add("status", "Status kann nur Verfügbar oder Wartung sein")
	}
	return errs
}

// Copy liefert eine Kopie mit eigener Merkmals-Map.
func (it Item) Copy() Item {
	attrs := make(map[string]string, len(it.Attributes))
	for k, v := range it.Attributes {
		attrs[k] = v
	}
	it.Attributes = attrs
	return it
}
