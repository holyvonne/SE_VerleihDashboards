package service

import (
	"testing"
	"time"

	"skiverleih/internal/domain"
)

// AK-22: Eine überfällige Ausleihe trifft eine nachfolgende Reservierung desselben Ski (Spezifikation 3.6a).
// Vor Version 1.1 waren in diesem Fall zwei Buchungen gleichzeitig „aktiv“ (Verstoß gegen I-2a).
func TestOverdueRentalBlocksFollowingReservation(t *testing.T) {
	s, c := newTestService(t)
	ski := mustSki(t, s)
	p1, p2 := mustPerson(t, s, "TEST0001"), mustPerson(t, s, "TEST0002")
	a := mustBook(t, s, booking(c, ski.ID, p1.ID, 0, 2*hour))      // 10–12 Uhr, aktiv
	b := mustBook(t, s, booking(c, ski.ID, p2.ID, 3*hour, 2*hour)) // 13–15 Uhr, Reservierung

	c.advance(150 * time.Minute) // 12:30: A überfällig, B gefährdet
	if vb := findView(t, s, b.ID); vb.State != domain.RentalReserved || !vb.Conflict {
		t.Errorf("12:30: B = %s, Konflikt %v; want reserviert mit Konflikt", vb.State, vb.Conflict)
	}
	st := s.DashboardStats()
	if len(st.Conflicts) != 1 || st.Reserved != 1 || st.Overdue != 1 {
		t.Errorf("Dashboard 12:30: %d Konflikte, %d reserviert, %d überfällig", len(st.Conflicts), st.Reserved, st.Overdue)
	}
	checkInvariants(t, s)

	c.advance(hour) // 13:30: Beginn von B erreicht, Ski noch nicht zurück
	vb := findView(t, s, b.ID)
	if vb.State != domain.RentalWaiting || !vb.Conflict {
		t.Errorf("13:30: B = %s, want wartend", vb.State)
	}
	if active := s.Rentals(FilterActive); len(active) != 1 || active[0].Rental.ID != a.ID {
		t.Errorf("I-2a: genau eine aktive Ausleihe erwartet, got %d", len(active))
	}
	if reserved := s.Rentals(FilterReserved); len(reserved) != 1 || reserved[0].Rental.ID != b.ID {
		t.Errorf("wartende Buchung muss unter „reserviert“ stehen: %v", reserved)
	}
	st = s.DashboardStats()
	if st.Rented != 1 || st.Reserved != 0 || len(st.Conflicts) != 1 || len(st.UpcomingReservations) != 1 {
		t.Errorf("Dashboard 13:30: ausgeliehen %d, reserviert %d, Konflikte %d", st.Rented, st.Reserved, len(st.Conflicts))
	}
	if rows := s.Persons(""); rows[1].Reservations != 1 || rows[1].ActiveRentals != 0 {
		t.Errorf("Personenliste: %+v", rows[1])
	}
	if _, err := s.Return(b.ID); err != domain.ErrNotStarted {
		t.Errorf("B wurde nie ausgegeben und darf nicht zurückgegeben werden: %v", err)
	}
	if _, err := s.Rent(booking(c, ski.ID, p2.ID, hour, hour)); err != domain.ErrSkiBooked {
		t.Errorf("überfällige Ausleihe muss neue Buchungen blockieren: %v", err)
	}
	checkInvariants(t, s)

	c.advance(10 * time.Minute) // 13:40: A kommt zurück → B ist sofort aktiv
	if _, err := s.Return(a.ID); err != nil {
		t.Fatal(err)
	}
	if vb := findView(t, s, b.ID); vb.State != domain.RentalActive || vb.Conflict {
		t.Errorf("nach Rückgabe von A: B = %s, Konflikt %v", vb.State, vb.Conflict)
	}
	if len(s.DashboardStats().Conflicts) != 0 {
		t.Error("Konflikt muss nach der Rückgabe verschwunden sein")
	}
	checkInvariants(t, s)
}

// AK-22: Eine wartende Buchung lässt sich stornieren; danach ist nur noch die überfällige Ausleihe da.
func TestCancelWaitingBooking(t *testing.T) {
	s, c := newTestService(t)
	ski := mustSki(t, s)
	p1, p2 := mustPerson(t, s, "TEST0001"), mustPerson(t, s, "TEST0002")
	mustBook(t, s, booking(c, ski.ID, p1.ID, 0, hour))
	b := mustBook(t, s, booking(c, ski.ID, p2.ID, 2*hour, hour))
	c.advance(150 * time.Minute)
	if err := s.Cancel(b.ID); err != nil {
		t.Fatalf("wartende Buchung muss stornierbar sein: %v", err)
	}
	if _, err := s.repo.Rental(b.ID); err != domain.ErrNotFound {
		t.Errorf("stornierte Buchung muss gelöscht sein: %v", err)
	}
	checkInvariants(t, s)
}

func findView(t *testing.T, s *Service, id int) RentalView {
	t.Helper()
	for _, v := range s.Rentals(FilterAll) {
		if v.Rental.ID == id {
			return v
		}
	}
	t.Fatalf("Buchung %d nicht gefunden", id)
	return RentalView{}
}
