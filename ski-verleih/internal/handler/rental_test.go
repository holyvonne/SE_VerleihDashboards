package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"skiverleih/internal/domain"
	"skiverleih/internal/service"
)

// ---------- Hilfen für das zweistufige Buchungsformular ----------

// bookingValues füllt das Buchungsformular wie der Browser.
func bookingValues(skiID, personID int, start, end time.Time) url.Values {
	return url.Values{
		"ski_id": {itoa(skiID)}, "person_id": {itoa(personID)},
		"start_date": {start.Format(dateLayout)}, "start_time": {start.Format(timeLayout)},
		"end_date": {end.Format(dateLayout)}, "end_time": {end.Format(timeLayout)},
	}
}

var tokenRe = regexp.MustCompile(`name="geprueft" value="([^"]+)"`)

// preview führt „Preis berechnen“ aus und liefert Seite und geprüfte Angaben.
func preview(t *testing.T, s *Server, v url.Values) (body, token string) {
	t.Helper()
	rec := postForm(s, "/ausleihen", with(v, "aktion", "vorschau"))
	if rec.Code != http.StatusOK {
		t.Fatalf("Vorschau: Status = %d; %.400s", rec.Code, rec.Body.String())
	}
	m := tokenRe.FindStringSubmatch(rec.Body.String())
	if m == nil {
		t.Fatal("Vorschau enthält kein Feld „geprueft“")
	}
	return rec.Body.String(), m[1]
}

// confirm führt „Verbindlich buchen“ mit den geprüften Angaben aus.
func confirm(s *Server, v url.Values, token string) *httptest.ResponseRecorder {
	return postForm(s, "/ausleihen", with(with(v, "aktion", "bestaetigen"), "geprueft", token))
}

// with liefert eine Kopie von v mit gesetztem Feld.
func with(v url.Values, key, value string) url.Values {
	out := url.Values{}
	for k, vs := range v {
		out[k] = append([]string(nil), vs...)
	}
	out.Set(key, value)
	return out
}

// ---------- F-4.1 Formular ----------

func TestRentalForm(t *testing.T) {
	s, svc := newApp(t)
	_, body := get(t, s, "/ausleihen/neu")
	assertContains(t, body, "Es gibt keinen buchbaren Ski.")

	rented, free, maint := addSki(t, svc), addSki(t, svc), addSki(t, svc)
	_, body = get(t, s, "/ausleihen/neu")
	assertContains(t, body, "Es ist noch keine Person registriert.")

	p := addPerson(t, svc, "TEST0001")
	_, _ = rentNow(svc, rented.ID, p.ID)
	_, _ = svc.UpdateSki(maint.ID, service.SkiInput{Brand: "Gipfelwerk", Model: "Pro", LengthCM: 170,
		Type: domain.SkiTypeAllround, Condition: domain.ConditionGood, Status: domain.SkiMaintenance})

	_, body = get(t, s, "/ausleihen/neu?ski="+itoa(free.ID)+"&person="+itoa(p.ID))
	assertContains(t, body,
		`<option value="`+itoa(free.ID)+`" selected>`, `<option value="`+itoa(p.ID)+`" selected>`,
		"#"+itoa(rented.ID)+" · Gipfelwerk Pro (170 cm) · Ausgeliehen", // für später reservierbar
		`name="start_date" type="date" value="2026-11-02"`, `name="start_time" type="time" value="14:05"`,
		`name="end_date" type="date" value="2026-11-02"`, `name="end_time" type="time" value="17:00"`,
		"20,00 € pro angefangenem Kalendertag, mindestens 20,00 €",
		`value="vorschau"`)
	if strings.Contains(body, "#"+itoa(maint.ID)+" · ") {
		t.Error("Ski in Wartung wird angeboten")
	}
	if strings.Contains(body, `value="bestaetigen"`) {
		t.Error("Buchen ohne Vorschau möglich")
	}
}

// ---------- F-4.2/F-4.3 Vorschau und Bestätigung ----------

// AK-13: Vorschau zeigt Zusammenfassung und Preis, speichert nichts.
func TestRentalPreview(t *testing.T) {
	s, svc := newApp(t)
	sk := addSki(t, svc)
	p := addPerson(t, svc, "TEST0001")
	end := time.Date(2026, 11, 4, 10, 0, 0, 0, time.Local) // 02.11. 14:05 – 04.11. 10:00

	body, token := preview(t, s, bookingValues(sk.ID, p.ID, testNow, end))
	assertContains(t, body,
		"Zusammenfassung", "Gipfelwerk Pro (170 cm)", "Erika Mustermann",
		"02.11.2026 14:05 – 04.11.2026 10:00", "Ausleihe ab sofort", "1 T 19 Std",
		"3 angefangene Kalendertage × 20,00 €", `data-price>60,00 €<`,
		`value="bestaetigen"`, "Verbindlich buchen")
	if token == "" {
		t.Error("kein Token")
	}
	if n := len(svc.Rentals(service.FilterAll)); n != 0 {
		t.Errorf("Vorschau hat %d Buchungen angelegt", n)
	}
}

// AK-8: Bestätigen legt die Ausleihe an; Dashboard ändert sich.
func TestRentalConfirm_Now(t *testing.T) {
	s, svc := newApp(t)
	sk := addSki(t, svc)
	p := addPerson(t, svc, "TEST0001")
	v := bookingValues(sk.ID, p.ID, testNow, testNow.Add(3*time.Hour))

	_, token := preview(t, s, v)
	assertRedirect(t, confirm(s, v, token), "/ausleihen?msg=rented&status=aktiv")

	rentals := svc.Rentals(service.FilterActive)
	if len(rentals) != 1 || rentals[0].Price != domain.Euro(20) {
		t.Fatalf("Buchungen = %+v", rentals)
	}
	_, body := get(t, s, "/")
	assertContains(t, body, `data-kpi="available">0<`, `data-kpi="rented">1<`)
}

// AK-15: Beginn in der Zukunft → Reservierung, Ski bleibt verfügbar.
func TestRentalConfirm_Reservation(t *testing.T) {
	s, svc := newApp(t)
	sk := addSki(t, svc)
	p := addPerson(t, svc, "TEST0001")
	start := time.Date(2026, 11, 7, 9, 0, 0, 0, time.Local)
	v := bookingValues(sk.ID, p.ID, start, start.Add(32*time.Hour)) // Sa 09:00 – So 17:00

	body, token := preview(t, s, v)
	assertContains(t, body, "badge-reserviert", "Reservierung", "40,00 €")
	assertRedirect(t, confirm(s, v, token), "/ausleihen?msg=reserved&status=reserviert")

	_, body = get(t, s, "/ausleihen?msg=reserved&status=reserviert")
	assertContains(t, body, "Reservierung wurde angelegt.", "07.11.2026 09:00", "Stornieren", "badge-reserviert")
	_, body = get(t, s, "/")
	assertContains(t, body, `data-kpi="available">1<`, `data-kpi="rented">0<`, `data-kpi="reserved">1<`)
	_, body = get(t, s, "/ski")
	assertContains(t, body, "Verfügbar", "reserviert ab 07.11.2026")
}

// AK-14: geänderte Angaben nach der Vorschau → keine Buchung, neue Vorschau mit Hinweis.
func TestRentalConfirm_ChangedAfterPreview(t *testing.T) {
	s, svc := newApp(t)
	sk := addSki(t, svc)
	p := addPerson(t, svc, "TEST0001")
	oneDay := bookingValues(sk.ID, p.ID, testNow, time.Date(2026, 11, 2, 17, 0, 0, 0, time.Local))
	body, token := preview(t, s, oneDay)
	assertContains(t, body, `data-price>20,00 €<`)

	threeDays := with(oneDay, "end_date", "2026-11-04")
	rec := confirm(s, threeDays, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Die Angaben haben sich geändert", `data-price>60,00 €<`)
	if n := len(svc.Rentals(service.FilterAll)); n != 0 {
		t.Fatalf("%d Buchungen trotz geänderter Angaben", n)
	}

	// Ohne Vorschau (kein Token) wird ebenfalls nicht gebucht.
	confirm(s, oneDay, "")
	if n := len(svc.Rentals(service.FilterAll)); n != 0 {
		t.Fatalf("%d Buchungen ohne Vorschau", n)
	}

	// Mit dem neuen Token klappt es.
	m := tokenRe.FindStringSubmatch(rec.Body.String())
	assertRedirect(t, confirm(s, threeDays, m[1]), "/ausleihen?msg=rented&status=aktiv")
	if r := svc.Rentals(service.FilterAll); len(r) != 1 || r[0].Price != domain.Euro(60) {
		t.Errorf("Buchung = %+v", r)
	}
}

// AK-9, AK-16: Überschneidung und Wartung werden schon in der Vorschau abgelehnt.
func TestRentalPreview_Conflicts(t *testing.T) {
	s, svc := newApp(t)
	sk := addSki(t, svc)
	maint := addSki(t, svc)
	p := addPerson(t, svc, "TEST0001")
	start := testNow.Add(24 * time.Hour)
	if _, err := svc.Rent(service.RentalInput{SkiID: sk.ID, PersonID: p.ID, Start: start, End: start.Add(48 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	_, _ = svc.UpdateSki(maint.ID, service.SkiInput{Brand: "Gipfelwerk", Model: "Pro", LengthCM: 170,
		Type: domain.SkiTypeAllround, Condition: domain.ConditionGood, Status: domain.SkiMaintenance})

	rec := postForm(s, "/ausleihen", with(bookingValues(sk.ID, p.ID, start.Add(24*time.Hour), start.Add(72*time.Hour)), "aktion", "vorschau"))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("Überschneidung: Status = %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Ski ist im gewählten Zeitraum bereits ausgeliehen oder reserviert.")
	if strings.Contains(rec.Body.String(), "Verbindlich buchen") {
		t.Error("Buchen trotz Konflikt angeboten")
	}

	// angrenzend danach ist erlaubt
	preview(t, s, bookingValues(sk.ID, p.ID, start.Add(48*time.Hour), start.Add(72*time.Hour)))

	// auch ein manipuliertes „bestaetigen“ ohne Vorschau bucht nicht
	rec = postForm(s, "/ausleihen", with(bookingValues(maint.ID, p.ID, start, start.Add(time.Hour)), "aktion", "bestaetigen"))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("Wartung: Status = %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Ski ist in Wartung und kann nicht gebucht werden.")
	if n := len(svc.Rentals(service.FilterAll)); n != 1 {
		t.Errorf("%d Buchungen, want 1", n)
	}
}

// AK-19: Eingabefehler am Feld.
func TestRentalPreview_InvalidDates(t *testing.T) {
	s, svc := newApp(t)
	sk := addSki(t, svc)
	p := addPerson(t, svc, "TEST0001")

	cases := []struct {
		name string
		v    url.Values
		want string
	}{
		{"Rückgabe vor Beginn", bookingValues(sk.ID, p.ID, testNow, testNow.Add(-time.Hour)), "Rückgabe muss nach dem Beginn liegen"},
		{"Beginn gestern", bookingValues(sk.ID, p.ID, testNow.Add(-24*time.Hour), testNow.Add(time.Hour)), "Beginn darf nicht in der Vergangenheit liegen"},
		{"Datum fehlt", with(bookingValues(sk.ID, p.ID, testNow, testNow.Add(time.Hour)), "end_date", ""), "Bitte Datum und Uhrzeit angeben"},
		{"Uhrzeit ungültig", with(bookingValues(sk.ID, p.ID, testNow, testNow.Add(time.Hour)), "start_time", "25:99"), "Bitte Datum und Uhrzeit angeben"},
		{"61 Tage", bookingValues(sk.ID, p.ID, testNow, testNow.Add(61*24*time.Hour)), "Verleihdauer darf höchstens 60 Tage betragen"},
		{"keine Auswahl", with(with(bookingValues(sk.ID, p.ID, testNow, testNow.Add(time.Hour)), "ski_id", ""), "person_id", "x"), "Bitte einen Ski auswählen"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := postForm(s, "/ausleihen", with(c.v, "aktion", "vorschau"))
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("Status = %d, want 422", rec.Code)
			}
			assertContains(t, rec.Body.String(), c.want, "Bitte die markierten Felder prüfen.")
		})
	}
}

// AK-11, AK-20 über HTTP: parallele Bestätigungen für denselben Ski – genau eine bucht.
func TestRentalConfirm_ConcurrentHTTP(t *testing.T) {
	s, svc := newApp(t)
	sk := addSki(t, svc)
	type req struct {
		v     url.Values
		token string
	}
	var reqs []req
	for i := 0; i < 30; i++ {
		p := addPerson(t, svc, "TEST00"+itoa(10+i))
		start := testNow.Add(time.Duration(48+i) * time.Hour) // alle überlappen sich
		v := bookingValues(sk.ID, p.ID, start, start.Add(72*time.Hour))
		_, token := preview(t, s, v)
		reqs = append(reqs, req{v, token})
	}

	codes := make([]int, len(reqs))
	var wg sync.WaitGroup
	for i, r := range reqs {
		wg.Add(1)
		go func(i int, r req) {
			defer wg.Done()
			codes[i] = confirm(s, r.v, r.token).Code
		}(i, r)
	}
	wg.Wait()

	ok := 0
	for _, c := range codes {
		if c == http.StatusSeeOther {
			ok++
		} else if c != http.StatusUnprocessableEntity {
			t.Errorf("unerwarteter Status %d", c)
		}
	}
	if ok != 1 {
		t.Errorf("%d erfolgreiche Buchungen, want genau 1", ok)
	}
}

// ---------- F-4.4/F-4.5 Rückgabe und Stornieren ----------

// AK-10, AK-17: Rückgabe, Rücksprung, Endpreis, zweite Rückgabe → 409.
func TestRentalReturn(t *testing.T) {
	s, svc, clock := newAppWithClock(t)
	sk := addSki(t, svc)
	p := addPerson(t, svc, "TEST0001")
	r, _ := rentNow(svc, sk.ID, p.ID) // 02.11. 14:05 – 03.11. 14:05 = 40 €
	path := "/ausleihen/" + itoa(r.ID) + "/rueckgabe"

	*clock = testNow.Add(48 * time.Hour) // einen Kalendertag zu spät
	assertRedirect(t, postForm(s, path, url.Values{"zurueck": {"/personen/" + itoa(p.ID)}}),
		"/personen/"+itoa(p.ID)+"?msg=returned")

	d, _ := svc.SkiDetail(sk.ID)
	if d.View.Status != domain.SkiAvailable || d.Current != nil || len(d.History) != 1 || d.History[0].Price != domain.Euro(60) {
		t.Errorf("nach Rückgabe: %+v", d)
	}
	_, body := get(t, s, "/ausleihen?status=abgeschlossen")
	assertContains(t, body, "60,00 €", "Zurückgegeben", "04.11.2026 14:05")

	rec := postForm(s, path, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("zweite Rückgabe: Status = %d, want 409", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Ausleihe wurde bereits zurückgegeben.")

	if rec := postForm(s, "/ausleihen/999/rueckgabe", nil); rec.Code != http.StatusNotFound {
		t.Errorf("unbekannte Ausleihe: Status = %d, want 404", rec.Code)
	}
}

// AK-18: Stornieren nur für Reservierungen; Reservierung nicht zurückgebbar.
func TestRentalCancel(t *testing.T) {
	s, svc := newApp(t)
	p := addPerson(t, svc, "TEST0001")
	start := testNow.Add(72 * time.Hour)
	res, _ := svc.Rent(service.RentalInput{SkiID: addSki(t, svc).ID, PersonID: p.ID, Start: start, End: start.Add(time.Hour)})
	active, _ := rentNow(svc, addSki(t, svc).ID, p.ID)

	rec := postForm(s, "/ausleihen/"+itoa(res.ID)+"/rueckgabe", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("Rückgabe einer Reservierung: Status = %d", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Die Reservierung hat noch nicht begonnen. Storniere sie stattdessen.")

	rec = postForm(s, "/ausleihen/"+itoa(active.ID)+"/stornieren", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("Stornieren einer Ausleihe: Status = %d", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Nur Reservierungen, die noch nicht begonnen haben, können storniert werden.")

	assertRedirect(t, postForm(s, "/ausleihen/"+itoa(res.ID)+"/stornieren", url.Values{"zurueck": {"/"}}), "/?msg=cancelled")
	if n := len(svc.Rentals(service.FilterReserved)); n != 0 {
		t.Errorf("%d Reservierungen nach Stornieren", n)
	}
	_, body := get(t, s, "/?msg=cancelled")
	assertContains(t, body, "Reservierung wurde storniert.")
}

// F-4.4: Rücksprung nur auf lokale Pfade (kein Open Redirect).
func TestSafeBack(t *testing.T) {
	cases := map[string]string{
		"/personen/3":            "/personen/3",
		"/ausleihen?status=alle": "/ausleihen?status=alle",
		"/":                      "/",
		"":                       "/ausleihen",
		"https://example.com/":   "/ausleihen",
		"//example.com":          "/ausleihen",
		"/\\example.com":         "/ausleihen",
		"javascript:alert(1)":    "/ausleihen",
		"personen/3":             "/ausleihen",
	}
	for in, want := range cases {
		if got := safeBack(in); got != want {
			t.Errorf("safeBack(%q) = %q, want %q", in, got, want)
		}
	}
}

// F-5: Übersicht mit vier Filtern, Preis und passender Aktion.
func TestRentalList_Filters(t *testing.T) {
	s, svc := newApp(t)
	p := addPerson(t, svc, "TEST0001")
	closed, _ := rentNow(svc, addSki(t, svc).ID, p.ID)
	_, _ = rentNow(svc, addSki(t, svc).ID, p.ID)
	start := testNow.Add(72 * time.Hour)
	_, _ = svc.Rent(service.RentalInput{SkiID: addSki(t, svc).ID, PersonID: p.ID, Start: start, End: start.Add(time.Hour)})
	_, _ = svc.Return(closed.ID)

	_, body := get(t, s, "/ausleihen")
	if strings.Count(body, `class="badge badge-ausgeliehen"`) != 1 || strings.Contains(body, "badge-closed") || strings.Contains(body, "badge-reserviert") {
		t.Error("Standardfilter soll nur aktive Ausleihen zeigen")
	}
	assertContains(t, body, "/rueckgabe", "Reserviert", "Abgeschlossen")

	_, body = get(t, s, "/ausleihen?status=reserviert")
	assertContains(t, body, "badge-reserviert", "/stornieren", `name="zurueck" value="/ausleihen?status=reserviert"`)
	if strings.Contains(body, "/rueckgabe") {
		t.Error("Reservierung bietet Rückgabe an")
	}

	_, body = get(t, s, "/ausleihen?status=abgeschlossen")
	assertContains(t, body, "badge-closed", "40,00 €")

	_, body = get(t, s, "/ausleihen?status=alle")
	assertContains(t, body, "badge-closed", "badge-reserviert", "badge-ausgeliehen")
}
