package domain

import "time"

// BookingState ist der aus der Zeit abgeleitete Zustand einer Buchung (Spezifikation 4.4).
type BookingState string

const (
	StateReserved BookingState = "reserviert"
	StateWaiting  BookingState = "wartend"
	StateActive   BookingState = "aktiv"
	StateClosed   BookingState = "abgeschlossen"
)

var bookingStateLabels = map[BookingState]string{
	StateReserved: "Reserviert",
	StateWaiting:  "Wartet auf Rückgabe",
	StateActive:   "Aktiv",
	StateClosed:   "Zurückgegeben",
}

func (s BookingState) Label() string {
	if l, ok := bookingStateLabels[s]; ok {
		return l
	}
	return string(s)
}

// Booking verbindet ein Objekt und eine Person für einen Zeitraum.
type Booking struct {
	ID         int
	ItemID     int
	PersonID   int
	Start      time.Time
	PlannedEnd time.Time
	BookedAt   time.Time
	Price      Price      // bei Buchung berechnet
	ReturnedAt *time.Time // nil = nicht zurückgegeben
	FinalPrice Price      // bei Rückgabe berechnet
}

// NewBooking legt eine Buchung an. Die Regeln prüft vorher der Service.
func NewBooking(itemID, personID int, p Period, bookedAt time.Time, price Price) Booking {
	return Booking{ItemID: itemID, PersonID: personID, Start: p.Start, PlannedEnd: p.End, BookedAt: bookedAt, Price: price}
}

// Planned ist der gebuchte Zeitraum [Beginn, geplante Rückgabe).
func (b Booking) Planned() Period { return Period{Start: b.Start, End: b.PlannedEnd} }

func (b Booking) IsReturned() bool { return b.ReturnedAt != nil }

// IsHandedOut meldet, ob der Beginn erreicht ist und die Buchung offen ist.
// Das gilt für aktive und für wartende Buchungen; das Objekt ist dann nicht frei.
func (b Booking) IsHandedOut(now time.Time) bool { return !b.IsReturned() && !now.Before(b.Start) }

// IsOverdue meldet eine offene Buchung, deren geplante Rückgabe vorbei ist.
func (b Booking) IsOverdue(now time.Time) bool {
	return b.IsHandedOut(now) && !now.Before(b.PlannedEnd)
}

// Blocks ist die Belegung (Spezifikation 4.5): überfällig → offen bis ∞,
// zurückgegeben → bis zur tatsächlichen Rückgabe, sonst der geplante Zeitraum.
func (b Booking) Blocks(now time.Time) Period {
	switch {
	case b.IsReturned():
		return Period{Start: b.Start, End: *b.ReturnedAt}
	case b.IsOverdue(now):
		return Period{Start: b.Start}
	default:
		return b.Planned()
	}
}

// ChargedPrice ist der Endpreis nach der Rückgabe, sonst der gebuchte Preis.
func (b Booking) ChargedPrice() Price {
	if b.IsReturned() {
		return b.FinalPrice
	}
	return b.Price
}

// Duration ist die tatsächliche Dauer nach der Rückgabe, sonst die geplante.
func (b Booking) Duration() time.Duration {
	if b.IsReturned() {
		return b.ReturnedAt.Sub(b.Start)
	}
	return b.PlannedEnd.Sub(b.Start)
}

// Return schließt die Buchung zum Zeitpunkt at ab und berechnet den Endpreis für
// [Beginn, max(geplante Rückgabe, at)) (Spezifikation 4.6). Ob die Buchung aktiv ist,
// prüft der Aufrufer über Situation, weil das die übrigen Buchungen braucht.
func (b *Booking) Return(at time.Time, quote func(Period) Price) error {
	if b.IsReturned() {
		return ErrAlreadyReturned
	}
	if at.Before(b.Start) {
		return ErrNotStarted
	}
	end := b.PlannedEnd
	if at.After(end) {
		end = at
	}
	returned := at
	b.ReturnedAt = &returned
	b.FinalPrice = quote(Period{Start: b.Start, End: end})
	return nil
}

// Situation ist der Zustand einer Buchung im Zusammenhang mit den übrigen Buchungen ihres Objekts.
type Situation struct {
	State    BookingState
	Overdue  bool // aktiv und geplante Rückgabe vorbei
	Conflict bool // noch nicht ausgegeben, aber durch eine frühere, überfällige Buchung blockiert (4.5a)
}

// Open meldet, ob die Buchung noch nicht abgeschlossen ist.
func (s Situation) Open() bool { return s.State != StateClosed }

// NotHandedOut meldet reservierte und wartende Buchungen; nur sie dürfen storniert oder umgebucht werden.
func (s Situation) NotHandedOut() bool { return s.State == StateReserved || s.State == StateWaiting }

// Evaluate bestimmt den Zustand von b. siblings sind alle Buchungen desselben Objekts (b darf enthalten sein).
func Evaluate(b Booking, siblings []Booking, now time.Time) Situation {
	if b.IsReturned() {
		return Situation{State: StateClosed}
	}
	conflict := blockedByEarlier(b, siblings, now)
	switch {
	case now.Before(b.Start):
		return Situation{State: StateReserved, Conflict: conflict}
	case conflict:
		return Situation{State: StateWaiting, Conflict: true}
	default:
		return Situation{State: StateActive, Overdue: b.IsOverdue(now)}
	}
}

// blockedByEarlier: Eine frühere, offene Buchung desselben Objekts belegt den Zeitraum von b.
// Das passiert nur, wenn sie überfällig ist, weil sich Buchungen beim Anlegen nie überschneiden.
func blockedByEarlier(b Booking, siblings []Booking, now time.Time) bool {
	for _, a := range siblings {
		if a.ID == b.ID || a.IsReturned() || !a.Start.Before(b.Start) {
			continue
		}
		if a.Blocks(now).Overlaps(b.Planned()) {
			return true
		}
	}
	return false
}

// FindConflict liefert die erste Buchung aus bookings, deren Belegung p überschneidet.
// Die Buchung mit der ID except wird übersprungen (z. B. beim Umbuchen die eigene).
func FindConflict(p Period, bookings []Booking, now time.Time, except int) (Booking, bool) {
	for _, b := range bookings {
		if b.ID != except && b.Blocks(now).Overlaps(p) {
			return b, true
		}
	}
	return Booking{}, false
}

// Copy liefert eine Kopie mit eigenem Rückgabezeitpunkt.
func (b Booking) Copy() Booking {
	if b.ReturnedAt != nil {
		t := *b.ReturnedAt
		b.ReturnedAt = &t
	}
	return b
}
