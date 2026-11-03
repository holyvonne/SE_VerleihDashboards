package service

import (
	"errors"
	"testing"
	"time"

	"skiverleih/internal/domain"
)

var testPNG = []byte("\x89PNG\r\n\x1a\n" + "fiktives-testbild")

// F-2.2, AK-3
func TestCreateSki(t *testing.T) {
	s, _ := newTestService(t)
	sk := mustSki(t, s)
	if sk.ID != 1 || sk.Status != domain.SkiAvailable {
		t.Errorf("Ski = %+v", sk)
	}
	in := skiInput()
	in.LengthCM = 300
	_, err := s.CreateSki(in)
	assertField(t, err, "length")
	if n := len(s.Skis("")); n != 1 {
		t.Errorf("%d Ski gespeichert, want 1", n)
	}
}

// F-2.1: Filter nach angezeigtem Status; BookableSkis ohne Wartung.
func TestSkis_Filter(t *testing.T) {
	s, c := newTestService(t)
	a := mustSki(t, s)
	mustSki(t, s)
	m := mustSki(t, s)
	p := mustPerson(t, s, "TEST0001")
	mustRent(t, s, c, a.ID, p.ID)
	in := skiInput()
	in.Status = domain.SkiMaintenance
	if _, err := s.UpdateSki(m.ID, in); err != nil {
		t.Fatal(err)
	}

	for status, want := range map[domain.SkiStatus]int{"": 3, domain.SkiRented: 1, domain.SkiAvailable: 1, domain.SkiMaintenance: 1} {
		if got := len(s.Skis(status)); got != want {
			t.Errorf("Skis(%q) = %d, want %d", status, got, want)
		}
	}
	if got := len(s.BookableSkis()); got != 2 {
		t.Errorf("BookableSkis = %d, want 2 (ausgeliehen bleibt für später buchbar)", got)
	}
}

// F-2.4
func TestUpdateSki(t *testing.T) {
	s, _ := newTestService(t)
	sk := mustSki(t, s)

	in := skiInput()
	in.Model, in.Status = "  Neu  ", domain.SkiMaintenance
	got, err := s.UpdateSki(sk.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "Neu" || got.Status != domain.SkiMaintenance {
		t.Errorf("Ski = %+v", got)
	}

	in.Status = domain.SkiRented // „ausgeliehen“ darf man nicht manuell setzen
	_, err = s.UpdateSki(sk.ID, in)
	assertField(t, err, "status")

	in.Status, in.Brand = domain.SkiAvailable, ""
	_, err = s.UpdateSki(sk.ID, in)
	assertField(t, err, "brand")
	if stored, _ := s.repo.Ski(sk.ID); stored.Brand != "Gipfelwerk" || stored.Status != domain.SkiMaintenance {
		t.Errorf("ungültige Änderung wurde gespeichert: %+v", stored)
	}

	if _, err := s.UpdateSki(999, skiInput()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// AK-4, AK-21: aktiv ausgeliehener oder reservierter Ski kann nicht auf Wartung;
// Stammdaten dürfen aber geändert werden.
func TestUpdateSki_InUse(t *testing.T) {
	s, c := newTestService(t)
	rented := mustSki(t, s)
	reserved := mustSki(t, s)
	p := mustPerson(t, s, "TEST0001")
	mustRent(t, s, c, rented.ID, p.ID)
	mustBook(t, s, booking(c, reserved.ID, p.ID, 3*day, day))

	for _, id := range []int{rented.ID, reserved.ID} {
		in := skiInput()
		in.Status = domain.SkiMaintenance
		if _, err := s.UpdateSki(id, in); !errors.Is(err, domain.ErrSkiInUse) {
			t.Errorf("Ski %d: err = %v, want ErrSkiInUse", id, err)
		}
		in.Status, in.Note = domain.SkiAvailable, "Belag frisch gewachst"
		if got, err := s.UpdateSki(id, in); err != nil || got.Note != "Belag frisch gewachst" {
			t.Errorf("Ski %d: Stammdaten ändern: %+v, %v", id, got, err)
		}
	}
	d, _ := s.SkiDetail(rented.ID)
	if d.View.Status != domain.SkiRented || !d.View.InUse() {
		t.Errorf("Status = %q", d.View.Status)
	}
	checkInvariants(t, s)
}

// F-2.5
func TestDeleteSki(t *testing.T) {
	s, c := newTestService(t)
	fresh := mustSki(t, s)
	used := mustSki(t, s)
	reserved := mustSki(t, s)
	p := mustPerson(t, s, "TEST0001")
	r := mustRent(t, s, c, used.ID, p.ID)
	if _, err := s.Return(r.ID); err != nil {
		t.Fatal(err)
	}
	mustBook(t, s, booking(c, reserved.ID, p.ID, day, day))

	if err := s.DeleteSki(fresh.ID); err != nil {
		t.Errorf("DeleteSki(ohne Buchungen): %v", err)
	}
	for _, id := range []int{used.ID, reserved.ID} {
		if err := s.DeleteSki(id); !errors.Is(err, domain.ErrHasHistory) {
			t.Errorf("DeleteSki(%d): err = %v, want ErrHasHistory", id, err)
		}
	}
	if err := s.DeleteSki(fresh.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("zweites Löschen: err = %v, want ErrNotFound", err)
	}
	d, _ := s.SkiDetail(used.ID)
	if !d.HasHistory() || d.Current != nil || len(d.History) != 1 {
		t.Errorf("SkiDetail = %+v", d)
	}
	checkInvariants(t, s)
}

// F-2.3: Detail trennt aktuelle Ausleihe, Reservierungen und Historie.
func TestSkiDetail(t *testing.T) {
	s, c := newTestService(t)
	sk := mustSki(t, s)
	p := mustPerson(t, s, "TEST0001")
	r := mustRent(t, s, c, sk.ID, p.ID)
	later := mustBook(t, s, booking(c, sk.ID, p.ID, 5*day, day))
	sooner := mustBook(t, s, booking(c, sk.ID, p.ID, 2*day, day))

	d, err := s.SkiDetail(sk.ID)
	if err != nil || d.Current == nil || d.Current.Rental.ID != r.ID || d.Current.Person.ID != p.ID {
		t.Fatalf("SkiDetail = %+v, err = %v", d, err)
	}
	if len(d.Reservations) != 2 || d.Reservations[0].Rental.ID != sooner.ID || d.Reservations[1].Rental.ID != later.ID {
		t.Errorf("Reservierungen = %+v", d.Reservations)
	}
	if d.View.NextReservation == nil || d.View.NextReservation.ID != sooner.ID {
		t.Errorf("NextReservation = %+v", d.View.NextReservation)
	}
	if _, err := s.SkiDetail(999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// F-3.2, AK-7
func TestCreatePerson(t *testing.T) {
	s, c := newTestService(t)
	in := personInput(" test0001 ")
	p, err := s.CreatePerson(in)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != 1 || p.IDNumber != "TEST0001" || p.HasIDImage() || !p.CreatedAt.Equal(c.now()) {
		t.Errorf("Person = %+v", p)
	}
	in.FirstName = ""
	_, err = s.CreatePerson(in)
	assertField(t, err, "first_name")
}

// AK-5, I-5: Ausweistyp + Nummer eindeutig, ohne Groß-/Kleinschreibung.
func TestCreatePerson_DuplicateID(t *testing.T) {
	s, _ := newTestService(t)
	mustPerson(t, s, "C01X00T47")
	_, err := s.CreatePerson(personInput("c01x00t47"))
	assertField(t, err, "id_number")
	var verr domain.ValidationErrors
	errors.As(err, &verr)
	if verr["id_number"] != "Diese Ausweisnummer ist bereits registriert" {
		t.Errorf("Meldung = %q", verr["id_number"])
	}

	other := personInput("C01X00T47")
	other.IDType = domain.IDCard // andere Ausweisart → erlaubt
	if _, err := s.CreatePerson(other); err != nil {
		t.Errorf("anderer Ausweistyp: %v", err)
	}
}

// AK-6: Bild prüfen; ungültiges Bild verhindert das Speichern.
func TestCreatePerson_Image(t *testing.T) {
	s, _ := newTestService(t)
	in := personInput("TEST0001")
	in.Image = testPNG
	p, err := s.CreatePerson(in)
	if err != nil || !p.HasIDImage() || p.IDImage.ContentType != "image/png" {
		t.Fatalf("Person = %+v, err = %v", p, err)
	}

	bad := personInput("TEST0002")
	bad.Image = []byte("%PDF-1.7 fiktiv")
	_, err = s.CreatePerson(bad)
	assertField(t, err, "id_image")
	if n := len(s.Persons("")); n != 1 {
		t.Errorf("%d Personen, want 1", n)
	}
}

// F-3.4: Bild behalten, ersetzen, entfernen; Eindeutigkeit gilt nicht gegen sich selbst.
func TestUpdatePerson(t *testing.T) {
	s, _ := newTestService(t)
	in := personInput("TEST0001")
	in.Image = testPNG
	p, _ := s.CreatePerson(in)
	mustPerson(t, s, "TEST0002")

	keep := personInput("TEST0001")
	keep.LastName = "Musterfrau"
	got, err := s.UpdatePerson(p.ID, keep)
	if err != nil || got.LastName != "Musterfrau" || !got.HasIDImage() {
		t.Fatalf("Bild behalten: %+v, %v", got, err)
	}

	remove := keep
	remove.RemoveImage = true
	got, _ = s.UpdatePerson(p.ID, remove)
	if got.HasIDImage() {
		t.Error("Bild nicht entfernt")
	}

	replace := keep
	replace.Image, replace.RemoveImage = []byte("\xFF\xD8\xFF\xE0fiktiv"), true
	got, _ = s.UpdatePerson(p.ID, replace)
	if !got.HasIDImage() || got.IDImage.ContentType != "image/jpeg" {
		t.Error("neues Bild soll Vorrang vor „entfernen“ haben")
	}

	dup := personInput("TEST0002")
	_, err = s.UpdatePerson(p.ID, dup)
	assertField(t, err, "id_number")

	if _, err := s.UpdatePerson(999, keep); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// F-3.1: Suche in Vorname, Nachname, Ausweisnummer.
func TestPersons_Search(t *testing.T) {
	s, _ := newTestService(t)
	mustPerson(t, s, "TEST0001")
	b := personInput("TEST0002")
	b.FirstName, b.LastName = "Max", "Muster"
	if _, err := s.CreatePerson(b); err != nil {
		t.Fatal(err)
	}
	cases := map[string]int{"": 2, "erika": 1, "MUSTER": 2, "0002": 1, "  max ": 1, "xyz": 0}
	for q, want := range cases {
		if got := len(s.Persons(q)); got != want {
			t.Errorf("Persons(%q) = %d Treffer, want %d", q, got, want)
		}
	}
}

// F-3.3, F-3.5
func TestPersonDetailAndDelete(t *testing.T) {
	s, c := newTestService(t)
	fresh := mustPerson(t, s, "TEST0001")
	renter := mustPerson(t, s, "TEST0002")
	mustRent(t, s, c, mustSki(t, s).ID, renter.ID)
	mustBook(t, s, booking(c, mustSki(t, s).ID, renter.ID, day, day))

	if err := s.DeletePerson(fresh.ID); err != nil {
		t.Errorf("DeletePerson(ohne Buchungen): %v", err)
	}
	if err := s.DeletePerson(renter.ID); !errors.Is(err, domain.ErrHasHistory) {
		t.Errorf("err = %v, want ErrHasHistory", err)
	}
	d, _ := s.PersonDetail(renter.ID)
	if !d.HasHistory() || len(d.Active) != 1 || len(d.Reservations) != 1 || len(d.History) != 0 {
		t.Errorf("PersonDetail = %+v", d)
	}
	checkInvariants(t, s)
}

// AK-1: Kennzahlen inkl. Wartung.
func TestDashboardStats(t *testing.T) {
	s, c := newTestService(t)
	var ids []int
	for i := 0; i < 5; i++ {
		ids = append(ids, mustSki(t, s).ID)
	}
	for _, n := range []string{"TEST0001", "TEST0002", "TEST0003"} {
		mustPerson(t, s, n)
	}
	mustRent(t, s, c, ids[0], 1)
	mustRent(t, s, c, ids[1], 2)
	in := skiInput()
	in.Status = domain.SkiMaintenance
	if _, err := s.UpdateSki(ids[2], in); err != nil {
		t.Fatal(err)
	}

	st := s.DashboardStats()
	if st.Available != 2 || st.Rented != 2 || st.Maintenance != 1 || st.Total != 5 || st.Persons != 3 || st.Reserved != 0 {
		t.Errorf("Stats = %+v", st)
	}
}

// F-1.3: höchstens 5 Einträge je Liste; aktive neueste zuerst, Reservierungen nächste zuerst.
func TestDashboardStats_Lists(t *testing.T) {
	s, c := newTestService(t)
	p := mustPerson(t, s, "TEST0001")
	var lastActive domain.Rental
	for i := 0; i < 7; i++ {
		lastActive = mustRent(t, s, c, mustSki(t, s).ID, p.ID)
		c.advance(1)
	}
	var firstReservation domain.Rental
	for i := 7; i > 0; i-- {
		firstReservation = mustBook(t, s, booking(c, mustSki(t, s).ID, p.ID, time.Duration(i)*day, day))
	}
	st := s.DashboardStats()
	if len(st.ActiveRentals) != 5 || st.ActiveRentals[0].Rental.ID != lastActive.ID {
		t.Errorf("ActiveRentals: %d, erster %d", len(st.ActiveRentals), st.ActiveRentals[0].Rental.ID)
	}
	if len(st.UpcomingReservations) != 5 || st.UpcomingReservations[0].Rental.ID != firstReservation.ID || st.Reserved != 7 {
		t.Errorf("UpcomingReservations: %d, erster %d, Reserved %d", len(st.UpcomingReservations), st.UpcomingReservations[0].Rental.ID, st.Reserved)
	}
}

// NF-7, NF-11: Demo-Daten laden konsistent.
func TestLoadDemo(t *testing.T) {
	s, _ := newTestService(t)
	if err := LoadDemo(s); err != nil {
		t.Fatal(err)
	}
	st := s.DashboardStats()
	if st.Total != 8 || st.Persons != 4 || st.Rented != 2 || st.Overdue != 1 || st.Reserved != 1 || st.Maintenance != 1 || st.Available != 5 {
		t.Errorf("Stats = %+v", st)
	}
	if n := len(s.Rentals(FilterAll)); n != 4 {
		t.Errorf("%d Buchungen, want 4", n)
	}
	if closed := s.Rentals(FilterClosed); len(closed) != 1 || closed[0].Price <= 0 {
		t.Errorf("abgeschlossen = %+v", closed)
	}
	for _, row := range s.Persons("") {
		if row.Person.IDNumber[:4] != "TEST" {
			t.Errorf("nicht-fiktive Ausweisnummer in Demo-Daten: %q", row.Person.IDNumber)
		}
	}
	checkInvariants(t, s)
}
