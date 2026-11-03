package handler

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/plugin"
	"fahrradverleih/framework/service"
	"fahrradverleih/framework/store"
	"fahrradverleih/internal/demo"
	"fahrradverleih/plugins/fahrrad"
	"fahrradverleih/plugins/ski"
)

var (
	hrefRe  = regexp.MustCompile(`href="([^"]+)"`)
	formRe  = regexp.MustCompile(`(?s)<form method="post" action="([^"]+)"([^>]*)>(.*?)</form>`)
	inputRe = regexp.MustCompile(`<input type="hidden" name="([^"]+)" value="([^"]*)">`)
)

func newDemoApp(t *testing.T) *app {
	t.Helper()
	a := newApp(t)
	if err := demo.Load(a.svc, testNow); err != nil {
		t.Fatal(err)
	}
	return a
}

// internalLinks liefert alle lokalen Seitenlinks einer Antwort (ohne CSS).
func internalLinks(body string) []string {
	var out []string
	for _, m := range hrefRe.FindAllStringSubmatch(body, -1) {
		link := html.UnescapeString(m[1])
		if strings.HasPrefix(link, "/") && !strings.HasPrefix(link, "//") && !strings.HasPrefix(link, "/static/") {
			out = append(out, link)
		}
	}
	return out
}

// F-9: Jeder Menüpunkt, jeder Link und jeder Button-Link der Anwendung führt zu einer gültigen Seite.
// Der Test startet auf dem Dashboard und folgt mit den Demo-Daten allen internen Links.
func TestCrawlAllLinks(t *testing.T) {
	a := newDemoApp(t)
	seen := map[string]bool{"/": true}
	queue := []string{"/"}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		rec := a.get(path)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: Status %d", path, rec.Code)
			continue
		}
		if !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
			continue // z. B. Ausweisbild
		}
		for _, link := range internalLinks(rec.Body.String()) {
			if !seen[link] {
				seen[link] = true
				queue = append(queue, link)
			}
		}
	}
	for _, must := range []string{"/fahrraeder", "/personen", "/buchungen", "/verfuegbarkeit", "/tarife", "/buchungen/neu",
		"/fahrraeder/neu", "/personen/neu", "/buchungen/2/umbuchen", "/fahrraeder/1/bearbeiten", "/fahrraeder/1/loeschen"} {
		if !seen[must] {
			t.Errorf("Link %s wurde nicht gefunden", must)
		}
	}
	t.Logf("%d Seiten über Links erreicht", len(seen))
	if len(seen) < 40 {
		t.Errorf("nur %d Seiten erreicht – Crawler oder Verlinkung prüfen", len(seen))
	}
}

// F-9: Jeder Button (POST-Formular) auf jeder erreichbaren Seite hat eine Route und antwortet sinnvoll.
// Jedes Formular wird in einer frischen Demo-App mit seinen versteckten Feldern abgeschickt.
func TestEveryFormButtonResponds(t *testing.T) {
	a := newDemoApp(t)
	type form struct {
		action    string
		multipart bool
		values    url.Values
	}
	forms := map[string]form{}
	seen := map[string]bool{"/": true}
	queue := []string{"/"}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		rec := a.get(path)
		if !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
			continue
		}
		body := rec.Body.String()
		for _, m := range formRe.FindAllStringSubmatch(body, -1) {
			v := url.Values{}
			for _, in := range inputRe.FindAllStringSubmatch(m[3], -1) {
				v.Set(in[1], html.UnescapeString(in[2]))
			}
			action := html.UnescapeString(m[1])
			key := action + "?" + v.Encode()
			forms[key] = form{action: action, multipart: strings.Contains(m[2], "multipart"), values: v}
		}
		for _, link := range internalLinks(body) {
			if !seen[link] {
				seen[link] = true
				queue = append(queue, link)
			}
		}
	}
	t.Logf("%d verschiedene POST-Formulare gefunden", len(forms))
	if len(forms) < 10 {
		t.Fatalf("nur %d Formulare gefunden", len(forms))
	}
	allowed := map[int]bool{http.StatusOK: true, http.StatusSeeOther: true, http.StatusConflict: true, http.StatusUnprocessableEntity: true}
	for key, f := range forms {
		fresh := newDemoApp(t)
		var code int
		if f.multipart {
			fields := map[string]string{}
			for k := range f.values {
				fields[k] = f.values.Get(k)
			}
			code = fresh.postMultipart(f.action, fields, nil).Code
		} else {
			code = fresh.post(f.action, f.values).Code
		}
		if !allowed[code] {
			t.Errorf("POST %s: Status %d", key, code)
		}
	}
}

// AK-1: Kennzahlen folgen jeder Buchung, Rückgabe, Stornierung und Umbuchung.
func TestDashboardFollowsActions(t *testing.T) {
	a := newDemoApp(t)
	kpis := func() map[string]string {
		body := a.get("/").Body.String()
		out := map[string]string{}
		for _, m := range regexp.MustCompile(`data-kpi="(\w+)">([^<]+)<`).FindAllStringSubmatch(body, -1) {
			out[m[1]] = strings.TrimSpace(m[2])
		}
		out["konflikt"] = fmt.Sprint(strings.Contains(body, "durch verspätete Rückgaben gefährdet"))
		return out
	}
	expect := func(step string, want map[string]string) {
		t.Helper()
		got := kpis()
		for k, v := range want {
			if got[k] != v {
				t.Errorf("%s: %s = %q, want %q (alle: %v)", step, k, got[k], v, got)
			}
		}
	}
	expect("Demo", map[string]string{"available": "6", "rented": "2", "overdue": "davon 1 überfällig", "reserved": "2", "total": "9", "persons": "4", "konflikt": "true"})

	// Neue Reservierung auf einem freien Rad (Pegasus Solero, #3) für morgen.
	v := bookingValues(3, 4, local(10, 0).AddDate(0, 0, 1), local(12, 0).AddDate(0, 0, 1))
	assertRedirect(t, a.confirm(v), "/buchungen?msg=reserved")
	expect("nach Reservierung", map[string]string{"available": "6", "reserved": "3"})

	// Sofort ausleihen: Kalkhoff Agattu (#2).
	v = bookingValues(2, 4, testNow, local(11, 0))
	v.Set("start_mode", startNow)
	assertRedirect(t, a.confirm(v), "/buchungen?msg=rented")
	expect("nach Ausleihe", map[string]string{"available": "5", "rented": "3"})

	// Überfällige Ausleihe zurückgeben (Buchung 1, Gazelle Esprit) → Konflikt aufgelöst.
	assertRedirect(t, a.post("/buchungen/1/rueckgabe", url.Values{"zurueck": {"/"}}), "/?msg=returned")
	expect("nach Rückgabe", map[string]string{"available": "6", "rented": "2", "konflikt": "false"})
	if strings.Contains(a.get("/").Body.String(), `data-kpi="overdue"`) {
		t.Error("nach der Rückgabe darf nichts mehr überfällig sein")
	}

	// Reservierung stornieren (Buchung 2, Lena).
	assertRedirect(t, a.post("/buchungen/2/stornieren", url.Values{"zurueck": {"/"}}), "/?msg=cancelled")
	expect("nach Storno", map[string]string{"reserved": "2"})
}

// F-2.2: Fahrrad über das Formular bearbeiten, inklusive Typwechsel und Wartung.
func TestBikeEditHTTP(t *testing.T) {
	a := newApp(t)
	bike := a.addBike(fahrrad.EBike)
	edit := url.Values{"category": {"ebike"}, "attr_marke": {"Stevens"}, "attr_modell": {"E-Courier"}, "attr_rahmengroesse": {"L"},
		"attr_akku_wh": {"625"}, "status": {"verfuegbar"}, "note": {"Neuer Akku"}}
	assertRedirect(t, a.post(fmt.Sprintf("/fahrraeder/%d", bike), edit), fmt.Sprintf("/fahrraeder/%d?msg=bike_updated", bike))
	assertContains(t, a.get(fmt.Sprintf("/fahrraeder/%d", bike)), "Stevens E-Courier", "625 Wh", "Neuer Akku", "12,00 € pro angefangene Stunde")

	edit.Set("category", "citybike") // Akku passt nicht mehr zum Typ
	rec := a.post(fmt.Sprintf("/fahrraeder/%d", bike), edit)
	assertStatus(t, rec, http.StatusUnprocessableEntity)
	assertContains(t, rec, fahrrad.MsgBatteryForbidden)

	edit.Del("attr_akku_wh")
	edit.Set("status", "wartung")
	assertRedirect(t, a.post(fmt.Sprintf("/fahrraeder/%d", bike), edit), fmt.Sprintf("/fahrraeder/%d", bike))
	v, _ := a.svc.Item(bike)
	if v.Item.Category != fahrrad.Citybike || v.Status != domain.StatusMaintenance {
		t.Errorf("gespeichert: %s %s", v.Item.Category, v.Status)
	}
	assertContains(t, a.get("/buchungen/neu"), "Es gibt kein buchbares Fahrrad.")
	assertStatus(t, a.post("/fahrraeder/999", edit), http.StatusNotFound)
}

// F-3, AK-26: Person bearbeiten, Bild ersetzen und entfernen über das Formular.
func TestPersonEditImageHTTP(t *testing.T) {
	a := newApp(t)
	fields := map[string]string{"first_name": "Lena", "last_name": "Beispiel", "id_type": "reisepass", "id_number": "TEST0950"}
	png := []byte("\x89PNG\r\n\x1a\n0000IHDR")
	jpeg := []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00")
	assertRedirect(t, a.postMultipart("/personen", fields, png), "/personen/1")

	fields["phone"] = "+49 30 123"
	assertRedirect(t, a.postMultipart("/personen/1", fields, nil), "/personen/1?msg=person_updated")
	p, _ := a.svc.Person(1)
	if !p.HasIDImage() || p.Phone != "+49 30 123" {
		t.Errorf("ohne neue Auswahl muss das Bild bleiben: %v %q", p.HasIDImage(), p.Phone)
	}
	assertContains(t, a.get("/personen/1"), `src="/personen/1/ausweisbild"`)

	assertRedirect(t, a.postMultipart("/personen/1", fields, jpeg), "/personen/1")
	if img := a.get("/personen/1/ausweisbild"); img.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("Bild ersetzt: %s", img.Header().Get("Content-Type"))
	}

	fields["remove_image"] = "1"
	assertRedirect(t, a.postMultipart("/personen/1", fields, nil), "/personen/1")
	assertStatus(t, a.get("/personen/1/ausweisbild"), http.StatusNotFound)
	assertContains(t, a.get("/personen/1"), "Kein Ausweisbild hinterlegt")

	delete(fields, "remove_image")
	fields["id_type"] = ""
	rec := a.postMultipart("/personen/1", fields, nil)
	assertStatus(t, rec, http.StatusUnprocessableEntity)
	assertContains(t, rec, "Bitte einen gültigen Ausweistyp wählen")

	assertRedirect(t, a.post("/personen/1/loeschen", nil), "/personen?msg=person_deleted")
}

// AK-21, F-8.2: Wird zusätzlich das Ski-Plugin registriert, läuft die Fahrrad-App unverändert weiter
// und zeigt nur Fahrräder. Ski-Objekte sind über die Fahrrad-Routen nicht erreichbar.
func TestBikeAppWithSkiPluginRegistered(t *testing.T) {
	bikes, _ := fahrrad.New(fahrrad.DefaultConfig())
	reg := plugin.NewRegistry()
	if err := reg.Register(bikes); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(ski.New(domain.Euro(20), nil)); err != nil {
		t.Fatalf("Ski-Plugin muss ohne Framework-Änderung registrierbar sein: %v", err)
	}
	c := &clock{t: testNow}
	svc := service.New(store.NewMemoryStore(), reg, c.now)
	srv, err := New(svc, bikes)
	if err != nil {
		t.Fatal(err)
	}
	a := &app{t: t, srv: srv, svc: svc, clock: c}
	skiItem, err := svc.CreateItem(service.ItemInput{Kind: ski.Key, Category: "carving", Attributes: map[string]string{
		ski.AttrBrand: "Atomic", ski.AttrModel: "Redster", ski.AttrLength: "170"}})
	if err != nil {
		t.Fatal(err)
	}
	bike := a.addBike(fahrrad.Citybike)

	rec := a.get("/fahrraeder")
	assertContains(t, rec, "Testrad citybike")
	if strings.Contains(rec.Body.String(), "Atomic") {
		t.Error("Ski darf in der Fahrradliste nicht erscheinen")
	}
	assertContains(t, a.get("/"), `data-kpi="total">1<`)
	assertStatus(t, a.get(fmt.Sprintf("/fahrraeder/%d", skiItem.ID)), http.StatusNotFound)
	if strings.Contains(a.get("/buchungen/neu").Body.String(), "Atomic") {
		t.Error("Ski darf im Fahrrad-Buchungsformular nicht wählbar sein")
	}
	assertStatus(t, a.get(fmt.Sprintf("/fahrraeder/%d", bike)), http.StatusOK)
}
