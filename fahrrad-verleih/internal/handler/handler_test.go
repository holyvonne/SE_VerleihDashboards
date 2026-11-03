package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/service"
	"fahrradverleih/plugins/fahrrad"
)

func TestPagesRender(t *testing.T) {
	a := newApp(t)
	bike, person := a.addBike(fahrrad.Citybike), a.addPerson()
	b := a.book(bike, person, local(12, 0), local(14, 0))
	for _, path := range []string{
		"/", "/fahrraeder", "/fahrraeder/neu", fmt.Sprintf("/fahrraeder/%d", bike), fmt.Sprintf("/fahrraeder/%d/bearbeiten", bike),
		"/personen", "/personen/neu", fmt.Sprintf("/personen/%d", person), "/buchungen", "/buchungen?status=reserviert",
		"/buchungen/neu", fmt.Sprintf("/buchungen/%d/umbuchen", b.ID), "/verfuegbarkeit", "/tarife",
	} {
		if rec := a.get(path); rec.Code != http.StatusOK {
			t.Errorf("GET %s: %d", path, rec.Code)
		}
	}
	assertStatus(t, a.get("/gibtsnicht"), http.StatusNotFound)
	assertStatus(t, a.get("/fahrraeder/999"), http.StatusNotFound)
}

// AK-2, AK-3: Fahrrad anlegen, Feldfehler mit 422.
func TestBikeCreate(t *testing.T) {
	a := newApp(t)
	rec := a.post("/fahrraeder", url.Values{"category": {"mountainbike"}, "attr_marke": {"Cube"}, "attr_modell": {"Aim"}, "attr_rahmengroesse": {"L"}})
	assertRedirect(t, rec, "/fahrraeder/1?msg=bike_created")
	assertContains(t, a.get("/fahrraeder"), "Cube Aim", "8,00 € pro angefangene Stunde", `badge-verfuegbar`)

	rec = a.post("/fahrraeder", url.Values{"category": {"mountainbike"}, "attr_rahmengroesse": {"XXL"}})
	assertStatus(t, rec, http.StatusUnprocessableEntity)
	assertContains(t, rec, "Marke ist ein Pflichtfeld", "Bitte eine gültige Auswahl treffen")

	rec = a.post("/fahrraeder", url.Values{"category": {"ebike"}, "attr_marke": {"X"}, "attr_modell": {"Y"}, "attr_rahmengroesse": {"M"}})
	assertStatus(t, rec, http.StatusUnprocessableEntity)
	assertContains(t, rec, fahrrad.MsgBatteryRequired)
	if n := len(a.svc.Items(service.ItemFilter{})); n != 1 {
		t.Errorf("nur ein Fahrrad darf gespeichert sein, got %d", n)
	}
}

// AK-4: Status gesperrt bei Reservierung, Löschen mit Buchungen → 409.
func TestBikeLockedAndDelete(t *testing.T) {
	a := newApp(t)
	bike, spare, person := a.addBike(fahrrad.Citybike), a.addBike(fahrrad.Citybike), a.addPerson()
	a.book(bike, person, local(12, 0), local(13, 0))
	assertContains(t, a.get(fmt.Sprintf("/fahrraeder/%d/bearbeiten", bike)), "Gesperrt, solange das Fahrrad ausgeliehen oder reserviert ist")

	rec := a.post(fmt.Sprintf("/fahrraeder/%d", bike), url.Values{"category": {"citybike"}, "attr_marke": {"A"}, "attr_modell": {"B"}, "attr_rahmengroesse": {"M"}, "status": {"wartung"}})
	assertStatus(t, rec, http.StatusUnprocessableEntity)
	assertContains(t, rec, "Fahrrad ist ausgeliehen oder reserviert – Status kann nicht geändert werden.")

	rec = a.post(fmt.Sprintf("/fahrraeder/%d/loeschen", bike), nil)
	assertStatus(t, rec, http.StatusConflict)
	assertContains(t, rec, "Fahrrad hat Buchungen und kann nicht gelöscht werden.")
	assertRedirect(t, a.post(fmt.Sprintf("/fahrraeder/%d/loeschen", spare), nil), "/fahrraeder?msg=bike_deleted")
}

func TestBikeListFilters(t *testing.T) {
	a := newApp(t)
	a.addBike(fahrrad.Citybike)
	e := a.addBike(fahrrad.EBike)
	a.book(e, a.addPerson(), local(12, 0), local(13, 0))
	rec := a.get("/fahrraeder?typ=ebike&status=reserviert")
	assertContains(t, rec, "Testrad ebike", "reserviert ab 10.11.2026 12:00")
	if strings.Contains(rec.Body.String(), "Testrad citybike") {
		t.Error("Citybike darf im E-Bike-Filter nicht erscheinen")
	}
}

// AK-8: Vorschau zeigt den Preis und speichert nichts.
func TestBookingPreview(t *testing.T) {
	a := newApp(t)
	bike, person := a.addBike(fahrrad.Mountainbike), a.addPerson()
	v := bookingValues(bike, person, local(9, 0), local(11, 30))
	v.Set("aktion", "vorschau")
	rec := a.post("/buchungen", v)
	assertStatus(t, rec, http.StatusOK)
	assertContains(t, rec, "Zusammenfassung", "Reservierung", "2 Std 30 Min", "3 angefangene Stunden × 8,00 €", `data-price>24,00 €`, "Verbindlich buchen")
	if n := len(a.svc.Bookings(service.FilterAll)); n != 0 {
		t.Fatalf("Vorschau hat %d Buchungen angelegt", n)
	}
}

// F-5.3, AK-10, AK-15: Bestätigen legt Reservierung bzw. Ausleihe an.
func TestBookingConfirm(t *testing.T) {
	a := newApp(t)
	bike, other, person := a.addBike(fahrrad.Citybike), a.addBike(fahrrad.Citybike), a.addPerson()
	assertRedirect(t, a.confirm(bookingValues(bike, person, local(12, 0), local(14, 0))), "/buchungen?msg=reserved&status=reserviert")

	a.clock.set(testNow.Add(25 * time.Second)) // mitten in der Minute: „Sofort“ darf trotzdem nicht scheitern
	v := bookingValues(other, person, time.Time{}, local(10, 0))
	v.Set("start_mode", startNow)
	assertRedirect(t, a.confirm(v), "/buchungen?msg=rented&status=aktiv")
	if it, _ := a.svc.Item(other); it.Status != domain.StatusRented {
		t.Errorf("sofort gebucht: Status %s", it.Status)
	}
	assertContains(t, a.get("/buchungen?status=aktiv&msg=rented"), "Fahrrad wurde ausgeliehen.")
}

// AK-9: Geänderte Angaben nach der Vorschau buchen nicht.
func TestBookingChangedAfterPreview(t *testing.T) {
	a := newApp(t)
	bike, person := a.addBike(fahrrad.Citybike), a.addPerson()
	v := bookingValues(bike, person, local(10, 0), local(11, 0))
	v.Set("aktion", "vorschau")
	m := tokenRe.FindStringSubmatch(a.post("/buchungen", v).Body.String())
	v.Set("end_time", "13:00")
	v.Set("aktion", "bestaetigen")
	v.Set("geprueft", htmlUnescape(m[1]))
	rec := a.post("/buchungen", v)
	assertStatus(t, rec, http.StatusOK)
	assertContains(t, rec, msgQuoteChanged, "data-price>15,00 €")
	if n := len(a.svc.Bookings(service.FilterAll)); n != 0 {
		t.Fatalf("es darf keine Buchung entstehen, got %d", n)
	}
}

// AK-11, AK-15, AK-16: Fehler im Formular.
func TestBookingErrors(t *testing.T) {
	a := newApp(t)
	bike, person := a.addBike(fahrrad.Citybike), a.addPerson()
	a.book(bike, person, local(10, 0), local(12, 0))

	v := bookingValues(bike, person, local(11, 0), local(13, 0))
	v.Set("aktion", "vorschau")
	rec := a.post("/buchungen", v)
	assertStatus(t, rec, http.StatusUnprocessableEntity)
	assertContains(t, rec, "Fahrrad ist im gewählten Zeitraum bereits ausgeliehen oder reserviert.")

	for _, c := range []struct {
		start, end time.Time
		msg        string
	}{
		{local(7, 0), local(9, 0), domain.MsgStartPast},
		{local(13, 0), local(13, 30), "Verleihdauer muss mindestens 1 Stunde betragen"},
		{local(13, 0), local(13, 0).AddDate(0, 0, 31), "Verleihdauer darf höchstens 30 Tage betragen"},
		{local(14, 0), local(13, 0), domain.MsgEndBefore},
	} {
		v := bookingValues(bike, person, c.start, c.end)
		v.Set("aktion", "vorschau")
		rec := a.post("/buchungen", v)
		assertStatus(t, rec, http.StatusUnprocessableEntity)
		assertContains(t, rec, c.msg)
	}
}

// AK-12: parallele Buchungen über HTTP – genau eine gelingt.
func TestBookingConcurrentHTTP(t *testing.T) {
	a := newApp(t)
	bike := a.addBike(fahrrad.EBike)
	persons := make([]int, 20)
	for i := range persons {
		persons[i] = a.addPerson()
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	start := make(chan struct{})
	for _, p := range persons {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			v := bookingValues(bike, p, local(10, 0), local(12, 0))
			v.Set("aktion", "vorschau")
			m := tokenRe.FindStringSubmatch(a.post("/buchungen", v).Body.String())
			if m == nil {
				return
			}
			<-start
			v.Set("aktion", "bestaetigen")
			v.Set("geprueft", htmlUnescape(m[1]))
			if rec := a.post("/buchungen", v); rec.Code == http.StatusSeeOther {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}(p)
	}
	time.Sleep(50 * time.Millisecond)
	close(start)
	wg.Wait()
	if ok != 1 {
		t.Fatalf("%d erfolgreiche Buchungen, want 1", ok)
	}
}

// AK-13, AK-14, F-5.4: Rückgabe und Storno über HTTP, Rücksprung nur auf lokale Pfade.
func TestReturnAndCancel(t *testing.T) {
	a := newApp(t)
	bike, person := a.addBike(fahrrad.EBike), a.addPerson()
	b := a.book(bike, person, local(9, 0), local(12, 0))
	res := a.book(bike, person, local(14, 0), local(15, 0))

	rec := a.post(fmt.Sprintf("/buchungen/%d/rueckgabe", b.ID), url.Values{"zurueck": {"/"}})
	assertStatus(t, rec, http.StatusConflict)
	assertContains(t, rec, "Die Reservierung hat noch nicht begonnen. Storniere sie stattdessen.")

	a.clock.set(local(12, 20))
	assertRedirect(t, a.post(fmt.Sprintf("/buchungen/%d/rueckgabe", b.ID), url.Values{"zurueck": {"https://example.com/"}}), "/buchungen?msg=returned")
	bv, _ := a.svc.Booking(b.ID)
	if bv.Price.Amount != domain.Euro(48) || !bv.Booking.ReturnedAt.Equal(local(12, 20)) {
		t.Errorf("Endpreis %v, Rückgabe %v", bv.Price, bv.Booking.ReturnedAt)
	}
	assertContains(t, a.get("/buchungen?status=abgeschlossen"), "48,00 €", "4 angefangene Stunden × 12,00 €")

	rec = a.post(fmt.Sprintf("/buchungen/%d/rueckgabe", b.ID), nil)
	assertStatus(t, rec, http.StatusConflict)
	assertContains(t, rec, "Ausleihe wurde bereits zurückgegeben.")

	assertRedirect(t, a.post(fmt.Sprintf("/buchungen/%d/stornieren", res.ID), url.Values{"zurueck": {"/fahrraeder/1"}}), "/fahrraeder/1?msg=cancelled")
	if _, err := a.svc.Booking(res.ID); err != domain.ErrNotFound {
		t.Errorf("storniert: %v", err)
	}
}

// AK-24, AK-25, F-1.5: Konflikt auf dem Dashboard und Umbuchen über HTTP.
func TestConflictAndReassign(t *testing.T) {
	a := newApp(t)
	bike, free, p1, p2 := a.addBike(fahrrad.Citybike), a.addBike(fahrrad.Citybike), a.addPerson(), a.addPerson()
	a.book(bike, p1, local(9, 0), local(10, 0))
	b := a.book(bike, p2, local(11, 0), local(13, 0))
	a.clock.set(local(10, 30))

	assertContains(t, a.get("/"), "durch verspätete Rückgaben gefährdet", "badge-conflict", `data-kpi="overdue"`)
	rec := a.get(fmt.Sprintf("/buchungen/%d/umbuchen", b.ID))
	assertContains(t, rec, "Freie Fahrräder im selben Zeitraum", fmt.Sprintf(`name="fahrrad" value="%d"`, free), "10,00 €")

	assertRedirect(t, a.post(fmt.Sprintf("/buchungen/%d/umbuchen", b.ID), url.Values{"fahrrad": {fmt.Sprint(free)}}), "/buchungen?msg=reassigned")
	bv, _ := a.svc.Booking(b.ID)
	if bv.Booking.ItemID != free || bv.Conflict {
		t.Errorf("umgebucht: Rad %d, Konflikt %v", bv.Booking.ItemID, bv.Conflict)
	}
	rec = a.post(fmt.Sprintf("/buchungen/%d/umbuchen", b.ID), url.Values{"fahrrad": {fmt.Sprint(bike)}})
	assertStatus(t, rec, http.StatusConflict)
}

// AK-6: Verfügbarkeitssuche mit Preis und vorbelegtem Buchen-Link.
func TestAvailabilityPage(t *testing.T) {
	a := newApp(t)
	busy, free, person := a.addBike(fahrrad.Citybike), a.addBike(fahrrad.Citybike), a.addPerson()
	a.addBike(fahrrad.EBike)
	a.book(busy, person, local(10, 0), local(12, 0))

	rec := a.get("/verfuegbarkeit?start_date=2026-11-10&start_time=11:00&end_date=2026-11-10&end_time=13:00&typ=citybike")
	assertStatus(t, rec, http.StatusOK)
	assertContains(t, rec, "<strong>1</strong> Fahrrad ist", fmt.Sprintf("/buchungen/neu?fahrrad=%d&amp;ende=", free), "data-offer-price>10,00 €")

	rec = a.get("/verfuegbarkeit?start_date=2026-11-10&start_time=11:00&end_date=2026-11-10&end_time=11:30")
	assertStatus(t, rec, http.StatusUnprocessableEntity)
	assertContains(t, rec, "Verleihdauer muss mindestens 1 Stunde betragen")

	rec = a.get(fmt.Sprintf("/buchungen/neu?fahrrad=%d&start=2026-11-10T11:00&ende=2026-11-10T13:00", free))
	assertContains(t, rec, `value="spaeter" checked`, `value="2026-11-10"`, `value="13:00"`)
}

// AK-18: konfigurierte Sätze erscheinen auf Tarifseite, Liste und Vorschau.
func TestConfiguredRatesShown(t *testing.T) {
	a := newAppWithConfig(t, fahrrad.DefaultConfig().WithRates(map[string]domain.Money{fahrrad.Citybike: domain.Euro(6)}))
	bike, person := a.addBike(fahrrad.Citybike), a.addPerson()
	assertContains(t, a.get("/tarife"), `data-rate="citybike">6,00 €`, "12,00 €")
	assertContains(t, a.get("/fahrraeder"), "6,00 € pro angefangene Stunde")
	v := bookingValues(bike, person, local(10, 0), local(11, 1))
	v.Set("aktion", "vorschau")
	assertContains(t, a.post("/buchungen", v), "data-price>12,00 €")
}

// AK-26: Ausweisbild optional, geprüft, nur über eigene Route.
func TestPersonImage(t *testing.T) {
	a := newApp(t)
	fields := map[string]string{"first_name": "Erika", "last_name": "Mustermann", "id_type": "personalausweis", "id_number": "TEST0901"}
	assertRedirect(t, a.postMultipart("/personen", fields, nil), "/personen/1?msg=person_created")

	fields["id_number"] = "TEST0902"
	png := []byte("\x89PNG\r\n\x1a\n0000IHDR")
	assertRedirect(t, a.postMultipart("/personen", fields, png), "/personen/2")
	img := a.get("/personen/2/ausweisbild")
	assertStatus(t, img, http.StatusOK)
	if img.Header().Get("Cache-Control") != "no-store" || img.Header().Get("Content-Type") != "image/png" {
		t.Errorf("Header: %v", img.Header())
	}
	assertStatus(t, a.get("/personen/1/ausweisbild"), http.StatusNotFound)

	fields["id_number"] = "TEST0903"
	rec := a.postMultipart("/personen", fields, []byte("kein Bild"))
	assertStatus(t, rec, http.StatusUnprocessableEntity)
	assertContains(t, rec, domain.MsgInvalidImage)

	big := append(append([]byte(nil), png...), bytes.Repeat([]byte{0}, 3<<20)...)
	assertStatus(t, a.postMultipart("/personen", fields, big), http.StatusRequestEntityTooLarge)

	fields["id_number"] = "test0901"
	rec = a.postMultipart("/personen", fields, nil)
	assertStatus(t, rec, http.StatusUnprocessableEntity)
	assertContains(t, rec, "Diese Ausweisnummer ist bereits registriert")
}

// F-9: Flash-Meldungen nur über feste Schlüssel.
func TestFlashOnlyKnownKeys(t *testing.T) {
	a := newApp(t)
	assertContains(t, a.get("/?msg=bike_created"), "Fahrrad wurde angelegt.")
	if strings.Contains(a.get("/?msg=%3Cscript%3E").Body.String(), "flash-success") {
		t.Error("unbekannter Schlüssel darf keine Meldung erzeugen")
	}
}

func TestSafeBack(t *testing.T) {
	cases := map[string]string{"/": "/", "/buchungen?status=aktiv": "/buchungen?status=aktiv", "//evil.example": "/buchungen", "https://evil.example": "/buchungen", "/\\evil": "/buchungen", "": "/buchungen"}
	for in, want := range cases {
		if got := safeBack(in); got != want {
			t.Errorf("safeBack(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[time.Duration]string{45 * time.Minute: "45 Min", 3 * time.Hour: "3 Std", 210 * time.Minute: "3 Std 30 Min", 51 * time.Hour: "2 T 3 Std"}
	for d, want := range cases {
		if got := formatDuration(d); got != want {
			t.Errorf("formatDuration(%v) = %q, want %q", d, got, want)
		}
	}
}
