package domain

import "time"

// RentalState ist der aus der Zeit abgeleitete Zustand einer Buchung (Spezifikation 3.5).
type RentalState string

const (
	RentalReserved RentalState = "reserviert"
	RentalWaiting  RentalState = "wartend" // Beginn erreicht, Ski aber noch nicht zurück (3.6a)
	RentalActive   RentalState = "aktiv"
	RentalClosed   RentalState = "abgeschlossen"
)

var rentalStateLabels = map[RentalState]string{
	RentalReserved: "Reserviert",
	RentalWaiting:  "Wartet auf Rückgabe",
	RentalActive:   "Aktiv",
	RentalClosed:   "Zurückgegeben",
}

func (s RentalState) Label() string { return labelOr(rentalStateLabels, s) }

// Rental ist eine Buchung: genau ein Ski für genau eine Person in einem Zeitraum
// (Spezifikation 3.3). In der Oberfläche heißt sie Ausleihe oder Reservierung.
type Rental struct {
	ID         int
	SkiID      int
	PersonID   int
	Start      time.Time // Verleihbeginn, kann in der Zukunft liegen
	PlannedEnd time.Time // geplante Rückgabe
	BookedAt   time.Time // Zeitpunkt der Buchung
	Price      Money     // Preis für [Start, PlannedEnd), bei Buchung berechnet
	ReturnedAt *time.Time
	FinalPrice Money // bei Rückgabe berechnet (3.7)
}

// NewRental legt eine Buchung für den geprüften Zeitraum p an.
func NewRental(skiID, personID int, p Period, bookedAt time.Time, price Money) Rental {
	return Rental{
		SkiID: skiID, PersonID: personID,
		Start: p.Start, PlannedEnd: p.End,
		BookedAt: bookedAt, Price: price,
	}
}

// Planned ist der gebuchte Zeitraum [Start, PlannedEnd).
func (r Rental) Planned() Period { return Period{Start: r.Start, End: r.PlannedEnd} }

func (r Rental) IsReturned() bool { return r.ReturnedAt != nil }

// State leitet den Zustand allein aus den eigenen Zeitpunkten ab. Ob eine begonnene Buchung
// noch auf einen überfälligen Vorgänger wartet, berücksichtigt erst Evaluate (3.6a).
func (r Rental) State(now time.Time) RentalState {
	switch {
	case r.IsReturned():
		return RentalClosed
	case now.Before(r.Start):
		return RentalReserved
	default:
		return RentalActive
	}
}

func (r Rental) IsActive(now time.Time) bool   { return r.State(now) == RentalActive }
func (r Rental) IsReserved(now time.Time) bool { return r.State(now) == RentalReserved }

// IsOverdue meldet eine aktive Ausleihe, deren geplante Rückgabe vorbei ist.
func (r Rental) IsOverdue(now time.Time) bool {
	return r.IsActive(now) && !now.Before(r.PlannedEnd)
}

// Blocks ist der Belegungszeitraum, in dem die Buchung den Ski blockiert (3.6).
func (r Rental) Blocks(now time.Time) Period {
	switch {
	case r.IsReturned():
		return Period{Start: r.Start, End: *r.ReturnedAt} // vorzeitige Rückgabe gibt Restzeit frei
	case r.IsOverdue(now):
		return Period{Start: r.Start} // offen: blockiert, bis der Ski zurück ist
	default:
		return r.Planned()
	}
}

// ChargedPrice ist der Endpreis bei abgeschlossenen, sonst der gebuchte Preis.
func (r Rental) ChargedPrice() Money {
	if r.IsReturned() {
		return r.FinalPrice
	}
	return r.Price
}

// Duration ist bei abgeschlossenen Buchungen die tatsächliche, sonst die geplante Dauer (F-5.1).
func (r Rental) Duration() time.Duration {
	if r.IsReturned() {
		return r.ReturnedAt.Sub(r.Start)
	}
	return r.Planned().Duration()
}

// Return schließt eine aktive Ausleihe zum Zeitpunkt at ab (F-4.4).
// Abgerechnet wird [Start, max(PlannedEnd, at)): keine Erstattung bei vorzeitiger,
// Zuschlag bei verspäteter Rückgabe (3.7, AK-17).
func (r *Rental) Return(at time.Time, pricing PricingPolicy) error {
	if r.IsReturned() {
		return ErrAlreadyReturned // AK-10
	}
	if at.Before(r.Start) {
		return ErrNotStarted // Reservierungen werden storniert, nicht zurückgegeben (AK-18)
	}
	billedEnd := r.PlannedEnd
	if at.After(billedEnd) {
		billedEnd = at
	}
	r.ReturnedAt = &at
	r.FinalPrice = pricing.Price(Period{Start: r.Start, End: billedEnd})
	return nil
}

// CheckCancellable erlaubt das Stornieren nur, solange der Ski nicht ausgegeben ist:
// vor dem Beginn oder während die Buchung wartet (F-4.5, AK-18, AK-22).
func (s Situation) CheckCancellable() error {
	if s.State != RentalReserved && s.State != RentalWaiting {
		return ErrNotCancellable
	}
	return nil
}

// Situation ist der Zustand einer Buchung im Zusammenhang mit den übrigen Buchungen ihres Ski (3.5, 3.6a).
type Situation struct {
	State    RentalState
	Overdue  bool // aktiv und geplante Rückgabe vorbei
	Conflict bool // noch nicht ausgegeben, aber von einer früheren, überfälligen Ausleihe blockiert
}

// Evaluate bestimmt den Zustand von r. siblings sind die Buchungen desselben Ski (r darf enthalten sein).
func Evaluate(r Rental, siblings []Rental, now time.Time) Situation {
	if r.IsReturned() {
		return Situation{State: RentalClosed}
	}
	conflict := blockedByEarlier(r, siblings, now)
	switch {
	case now.Before(r.Start):
		return Situation{State: RentalReserved, Conflict: conflict}
	case conflict:
		return Situation{State: RentalWaiting, Conflict: true}
	default:
		return Situation{State: RentalActive, Overdue: r.IsOverdue(now)}
	}
}

// blockedByEarlier: Eine frühere, offene Buchung desselben Ski belegt den Zeitraum von r.
// Weil sich Buchungen beim Anlegen nie überschneiden, passiert das nur bei überfälligen Ausleihen.
func blockedByEarlier(r Rental, siblings []Rental, now time.Time) bool {
	for _, a := range siblings {
		if a.ID == r.ID || a.SkiID != r.SkiID || a.IsReturned() || !a.Start.Before(r.Start) {
			continue
		}
		if a.Blocks(now).Overlaps(r.Planned()) {
			return true
		}
	}
	return false
}

// FindConflict sucht unter den Buchungen eines Ski eine, deren Belegung sich mit p
// überschneidet (3.6, AK-16).
func FindConflict(p Period, rentals []Rental, now time.Time) (Rental, bool) {
	for _, r := range rentals {
		if r.Blocks(now).Overlaps(p) {
			return r, true
		}
	}
	return Rental{}, false
}
