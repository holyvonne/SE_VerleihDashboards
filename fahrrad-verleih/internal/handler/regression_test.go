package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/plugin"
	"fahrradverleih/framework/service"
	"fahrradverleih/framework/store"
	"fahrradverleih/plugins/fahrrad"
	"fahrradverleih/plugins/ski"
)

// Abschlussprüfung, Befund 1: Das Größenlimit (NF-4) greift auch, wenn der Client „; charset=…“ anhängt.
func TestFormLimitWithCharset(t *testing.T) {
	a := newApp(t)
	bike, person := a.addBike(fahrrad.Citybike), a.addPerson()
	v := bookingValues(bike, person, local(10, 0), local(11, 0))
	v.Set("aktion", "vorschau")
	v.Set("polster", strings.Repeat("x", maxFormBytes+1))
	req := httptest.NewRequest(http.MethodPost, "/buchungen", strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	rec := a.serve(req)
	if rec.Code == http.StatusOK || rec.Code == http.StatusSeeOther {
		t.Fatalf("zu große Anfrage wurde verarbeitet: Status %d", rec.Code)
	}
}

// Abschlussprüfung, Befund 2: Die Fahrrad-App bucht keine Objekte einer anderen Verleihart,
// auch nicht über eine manipulierte ID im Formular (F-8.2).
func TestBookingRejectsOtherKind(t *testing.T) {
	bikes, _ := fahrrad.New(fahrrad.DefaultConfig())
	reg := plugin.NewRegistry()
	_ = reg.Register(bikes)
	_ = reg.Register(ski.New(domain.Euro(20), nil))
	c := &clock{t: testNow}
	svc := service.New(store.NewMemoryStore(), reg, c.now)
	srv, err := New(svc, bikes)
	if err != nil {
		t.Fatal(err)
	}
	a := &app{t: t, srv: srv, svc: svc, clock: c}
	skiItem, err := svc.CreateItem(service.ItemInput{Kind: ski.Key, Category: "allround", Attributes: map[string]string{
		ski.AttrBrand: "Fischer", ski.AttrModel: "RC4", ski.AttrLength: "165"}})
	if err != nil {
		t.Fatal(err)
	}
	a.addBike(fahrrad.Citybike) // damit das Formular und sein Feldfehler erscheinen
	person := a.addPerson()
	v := bookingValues(skiItem.ID, person, local(10, 0), local(12, 0))
	v.Set("aktion", "vorschau")
	rec := a.post("/buchungen", v)
	assertStatus(t, rec, http.StatusUnprocessableEntity)
	assertContains(t, rec, "Bitte eine gültige Auswahl treffen")
	if n := len(svc.Bookings(service.FilterAll)); n != 0 {
		t.Errorf("es darf keine Buchung entstehen, got %d", n)
	}
}

// Abschlussprüfung, Befund 3: Nach dem Umbuchen einer wartenden Buchung führt die Weiterleitung
// zur Liste der aktiven Ausleihen, weil die Buchung sofort aktiv ist.
func TestReassignWaitingRedirectsToActive(t *testing.T) {
	a := newApp(t)
	bike, free, p1, p2 := a.addBike(fahrrad.Citybike), a.addBike(fahrrad.Citybike), a.addPerson(), a.addPerson()
	a.book(bike, p1, local(9, 0), local(10, 0))
	b := a.book(bike, p2, local(11, 0), local(13, 0))
	a.clock.set(local(11, 30)) // erste Ausleihe überfällig, b wartet

	rec := a.get("/buchungen?status=reserviert")
	assertContains(t, rec, "Wartet auf Rückgabe", "badge-conflict", fmt.Sprintf("/buchungen/%d/umbuchen", b.ID))
	assertRedirect(t, a.post(fmt.Sprintf("/buchungen/%d/umbuchen", b.ID), url.Values{"fahrrad": {fmt.Sprint(free)}}),
		"/buchungen?msg=reassigned&status=aktiv")
	assertContains(t, a.get("/buchungen?status=aktiv"), "Testrad citybike", "Zurückgeben")
}
