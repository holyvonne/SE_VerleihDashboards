package service_test

import (
	"testing"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/service"
	"fahrradverleih/plugins/fahrrad"
)

// AK-2, AK-3, AK-23: Prüfung in drei Stufen.
func TestCreateItemValidation(t *testing.T) {
	s, _ := newService(t)
	it, err := s.CreateItem(service.ItemInput{Kind: fahrrad.Key, Category: fahrrad.Mountainbike, Attributes: bikeAttrs("Cube")})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := s.Item(it.ID)
	if v.Status != domain.StatusAvailable || v.Rule != "8,00 € pro angefangene Stunde, mindestens 1 Stunde" || v.CategoryLabel != "Mountainbike" {
		t.Errorf("angelegt: %+v", v)
	}

	_, err = s.CreateItem(service.ItemInput{Kind: fahrrad.Key, Category: fahrrad.Citybike, Attributes: map[string]string{fahrrad.AttrFrame: "XXL"}})
	assertField(t, err, "attr_marke", "Marke ist ein Pflichtfeld")
	assertField(t, err, "attr_rahmengroesse", "Bitte eine gültige Auswahl treffen")

	_, err = s.CreateItem(service.ItemInput{Kind: fahrrad.Key, Category: fahrrad.EBike, Attributes: bikeAttrs("Rad")})
	assertField(t, err, "attr_akku_wh", fahrrad.MsgBatteryRequired)

	attrs := bikeAttrs("Rad")
	attrs[fahrrad.AttrBattery] = "400"
	_, err = s.CreateItem(service.ItemInput{Kind: fahrrad.Key, Category: fahrrad.Citybike, Attributes: attrs})
	assertField(t, err, "attr_akku_wh", fahrrad.MsgBatteryForbidden)

	attrs[fahrrad.AttrBattery] = "150"
	_, err = s.CreateItem(service.ItemInput{Kind: fahrrad.Key, Category: fahrrad.EBike, Attributes: attrs})
	assertField(t, err, "attr_akku_wh", "Wert muss zwischen 200 und 1000 liegen")

	_, err = s.CreateItem(service.ItemInput{Kind: fahrrad.Key, Category: "roller", Attributes: bikeAttrs("Rad")})
	assertField(t, err, "category", "Bitte eine gültige Kategorie wählen")

	_, err = s.CreateItem(service.ItemInput{Kind: "boot", Category: "x"})
	assertErr(t, err, domain.ErrUnknownKind)

	if n := len(s.Items(service.ItemFilter{})); n != 1 {
		t.Errorf("nur das gültige Rad darf gespeichert sein, got %d", n)
	}
}

// AK-4: Statuswechsel gesperrt bei offenen Buchungen, Löschen nur ohne Buchungen.
func TestUpdateAndDeleteItem(t *testing.T) {
	s, _ := newService(t)
	bike, spare, person := addBike(t, s, fahrrad.Citybike), addBike(t, s, fahrrad.Citybike), addPerson(t, s)
	mustBook(t, s, in(bike, person, at(10, 0), at(11, 0)))

	maintenance := service.ItemInput{Category: fahrrad.Citybike, Attributes: bikeAttrs("Rad"), Status: domain.StatusMaintenance}
	_, err := s.UpdateItem(bike, maintenance)
	assertErr(t, err, domain.ErrItemInUse)

	renamed := service.ItemInput{Category: fahrrad.Citybike, Attributes: bikeAttrs("Neu"), Note: "  frisch geölt "}
	if _, err := s.UpdateItem(bike, renamed); err != nil {
		t.Fatalf("Merkmale ändern muss trotz Buchung gehen: %v", err)
	}
	v, _ := s.Item(bike)
	if v.Name != "Neu Test" || v.Item.Note != "frisch geölt" {
		t.Errorf("gespeichert: %q %q", v.Name, v.Item.Note)
	}

	assertErr(t, s.DeleteItem(bike), domain.ErrHasHistory)
	if err := s.DeleteItem(spare); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Item(spare); err != domain.ErrNotFound {
		t.Errorf("gelöscht: %v", err)
	}
}

func TestItemFilters(t *testing.T) {
	s, c := newService(t)
	city, mtb, e, person := addBike(t, s, fahrrad.Citybike), addBike(t, s, fahrrad.Mountainbike), addBike(t, s, fahrrad.EBike), addPerson(t, s)
	mustBook(t, s, in(city, person, at(8, 0), at(10, 0)))
	mustBook(t, s, in(e, person, at(12, 0), at(13, 0)))
	c.set(at(9, 0))
	count := func(f service.ItemFilter) int { return len(s.Items(f)) }
	if count(service.ItemFilter{Status: "ausgeliehen"}) != 1 || count(service.ItemFilter{Status: "verfuegbar"}) != 2 {
		t.Error("Statusfilter")
	}
	if got := s.Items(service.ItemFilter{Status: service.FilterReservedItems}); len(got) != 1 || got[0].Item.ID != e {
		t.Errorf("reserviert: %v", got)
	}
	if got := s.Items(service.ItemFilter{Category: fahrrad.Mountainbike}); len(got) != 1 || got[0].Item.ID != mtb {
		t.Errorf("Typfilter: %v", got)
	}
	if count(service.ItemFilter{Category: fahrrad.EBike, Status: "ausgeliehen"}) != 0 {
		t.Error("kombinierter Filter")
	}
}

// AK-5, AK-26: Personen mit eindeutiger Ausweisnummer und optionalem Bild.
func TestPersons(t *testing.T) {
	s, _ := newService(t)
	in1 := service.PersonInput{FirstName: "Erika", LastName: "Mustermann", IDType: domain.IDCard, IDNumber: "TEST0001"}
	p, err := s.CreatePerson(in1)
	if err != nil {
		t.Fatal(err)
	}
	if p.HasIDImage() {
		t.Error("ohne Bild angelegt")
	}

	dup := in1
	dup.IDNumber = "test0001"
	_, err = s.CreatePerson(dup)
	assertField(t, err, "id_number", "Diese Ausweisnummer ist bereits registriert")

	withImage := service.PersonInput{FirstName: "Max", LastName: "Mustermann", IDType: domain.IDPassport, IDNumber: "TEST0002",
		Image: []byte("\x89PNG\r\n\x1a\n0000IHDR")}
	p2, err := s.CreatePerson(withImage)
	if err != nil || !p2.HasIDImage() || p2.IDImage.ContentType != "image/png" {
		t.Fatalf("mit Bild: %v %v", p2.IDImage, err)
	}
	bad := withImage
	bad.IDNumber, bad.Image = "TEST0003", []byte("kein Bild")
	_, err = s.CreatePerson(bad)
	assertField(t, err, "id_image", domain.MsgInvalidImage)

	upd := withImage
	upd.Image, upd.RemoveImage = nil, false
	if p2, _ = s.UpdatePerson(p2.ID, upd); !p2.HasIDImage() {
		t.Error("ohne neue Auswahl bleibt das Bild erhalten")
	}
	upd.RemoveImage = true
	if p2, _ = s.UpdatePerson(p2.ID, upd); p2.HasIDImage() {
		t.Error("Bild entfernen")
	}

	if rows := s.Persons("muster"); len(rows) != 2 {
		t.Errorf("Suche: %d", len(rows))
	}
	if rows := s.Persons("0002"); len(rows) != 1 || rows[0].Person.FirstName != "Max" {
		t.Errorf("Suche nach Nummer: %v", rows)
	}

	bike := addBike(t, s, fahrrad.Citybike)
	mustBook(t, s, in(bike, p.ID, at(10, 0), at(11, 0)))
	assertErr(t, s.DeletePerson(p.ID), domain.ErrHasHistory)
	if err := s.DeletePerson(p2.ID); err != nil {
		t.Fatal(err)
	}
	d, _ := s.PersonDetail(p.ID)
	if len(d.Upcoming) != 1 || !d.HasHistory() {
		t.Errorf("Detail: %+v", d)
	}
}

// AK-1 (Service): Kennzahlen nach Typ.
func TestStats(t *testing.T) {
	s, c := newService(t)
	city, e1, e2, person := addBike(t, s, fahrrad.Citybike), addBike(t, s, fahrrad.EBike), addBike(t, s, fahrrad.EBike), addPerson(t, s)
	mustBook(t, s, in(city, person, at(8, 0), at(9, 0)))
	mustBook(t, s, in(e1, person, at(12, 0), at(13, 0)))
	if _, err := s.UpdateItem(e2, service.ItemInput{Category: fahrrad.EBike, Attributes: map[string]string{
		fahrrad.AttrBrand: "Rad", fahrrad.AttrModel: "Test", fahrrad.AttrFrame: "M", fahrrad.AttrBattery: "500",
	}, Status: domain.StatusMaintenance}); err != nil {
		t.Fatal(err)
	}
	c.set(at(9, 30)) // Citybike überfällig
	st, err := s.Stats(fahrrad.Key)
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != 3 || st.Rented != 1 || st.Overdue != 1 || st.Available != 1 || st.Maintenance != 1 || st.Reserved != 1 || st.Persons != 1 {
		t.Errorf("Kennzahlen: %+v", st)
	}
	var ebike service.CategoryStats
	for _, cs := range st.Categories {
		if cs.Category.Key == fahrrad.EBike {
			ebike = cs
		}
	}
	if ebike.Total != 2 || ebike.Available != 1 || ebike.Maintenance != 1 || ebike.Rule == "" {
		t.Errorf("E-Bikes: %+v", ebike)
	}
	if len(st.Active) != 1 || !st.Active[0].Overdue || len(st.Upcoming) != 1 {
		t.Errorf("Listen: %d aktiv, %d anstehend", len(st.Active), len(st.Upcoming))
	}
	_, err = s.Stats("boot")
	assertErr(t, err, domain.ErrUnknownKind)
}
