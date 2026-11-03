package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"skiverleih/internal/domain"
)

func skiValues() url.Values {
	return url.Values{
		"brand": {"Firnflug"}, "model": {"Carve RS"}, "length": {"165"},
		"type": {"carving"}, "condition": {"neu"}, "note": {""},
	}
}

// F-2.2: Anlegen → 303 zur Detailseite mit Meldung.
func TestSkiCreate(t *testing.T) {
	s, svc := newApp(t)
	rec := postForm(s, "/ski", skiValues())
	assertRedirect(t, rec, "/ski/1?msg=ski_created")

	_, body := get(t, s, "/ski/1?msg=ski_created")
	assertContains(t, body, "Ski wurde angelegt.", "Firnflug Carve RS", "165 cm", "Carving", "Verfügbar")
	if n := len(svc.Skis("")); n != 1 {
		t.Errorf("%d Ski, want 1", n)
	}
}

// AK-3: ungültige Länge → 422, Formular mit Eingaben und Feldmeldung, nichts gespeichert.
func TestSkiCreate_Invalid(t *testing.T) {
	s, svc := newApp(t)
	v := skiValues()
	v.Set("length", "300")
	v.Set("brand", "")
	rec := postForm(s, "/ski", v)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("Status = %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(),
		"Länge muss zwischen 70 und 220 cm liegen", "Marke ist ein Pflichtfeld",
		`value="Carve RS"`, `value="300"`, "Bitte die markierten Felder prüfen.")
	if n := len(svc.Skis("")); n != 0 {
		t.Errorf("%d Ski gespeichert, want 0", n)
	}
}

// F-2.1: Liste mit Statusfilter und Ausleih-Button.
func TestSkiList(t *testing.T) {
	s, svc := newApp(t)
	_, body := get(t, s, "/ski")
	assertContains(t, body, "Noch keine Ski erfasst.", "Ersten Ski anlegen")

	a, b := addSki(t, svc), addSki(t, svc)
	p := addPerson(t, svc, "TEST0001")
	_, _ = rentNow(svc, a.ID, p.ID)

	_, body = get(t, s, "/ski")
	assertContains(t, body, `href="/ski/1"`, `href="/ski/2"`, "badge-ausgeliehen", `href="/ausleihen/neu?ski=`+itoa(b.ID)+`"`)
	_, body = get(t, s, "/ski?status=verfuegbar")
	if strings.Contains(body, `href="/ski/1"`) || !strings.Contains(body, `href="/ski/2"`) {
		t.Error("Filter verfuegbar falsch")
	}
	_, body = get(t, s, "/ski?status=wartung")
	assertContains(t, body, "Keine Ski mit diesem Status.")
}

// F-2.4: Bearbeiten und Status Wartung.
func TestSkiUpdate(t *testing.T) {
	s, svc := newApp(t)
	sk := addSki(t, svc)

	code, body := get(t, s, "/ski/1/bearbeiten")
	if code != http.StatusOK {
		t.Fatalf("Status = %d", code)
	}
	assertContains(t, body, `value="Gipfelwerk"`, `<option value="wartung"`)

	v := skiValues()
	v.Set("status", "wartung")
	assertRedirect(t, postForm(s, "/ski/1", v), "/ski/1?msg=ski_updated")
	got, _ := svc.SkiDetail(sk.ID)
	if got.View.Ski.Brand != "Firnflug" || got.View.Ski.Status != domain.SkiMaintenance {
		t.Errorf("Ski = %+v", got.View.Ski)
	}
}

// AK-4: manipulierter POST setzt ausgeliehenen Ski nicht auf Wartung.
func TestSkiUpdate_RentedCannotGoToMaintenance(t *testing.T) {
	s, svc := newApp(t)
	sk := addSki(t, svc)
	p := addPerson(t, svc, "TEST0001")
	_, _ = rentNow(svc, sk.ID, p.ID)

	_, body := get(t, s, "/ski/1/bearbeiten")
	assertContains(t, body, `name="status" value="verfuegbar"`, "disabled")

	v := skiValues()
	v.Set("status", "wartung")
	rec := postForm(s, "/ski/1", v)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("Status = %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Ski ist ausgeliehen oder reserviert – Status kann nicht geändert werden.")
	if d, _ := svc.SkiDetail(sk.ID); d.View.Status != domain.SkiRented {
		t.Errorf("Status = %q, want ausgeliehen", d.View.Status)
	}

	// Stammdaten ändern bleibt möglich; das gesperrte Formular sendet den Grundstatus.
	v.Set("status", "verfuegbar")
	assertRedirect(t, postForm(s, "/ski/1", v), "/ski/1?msg=ski_updated")
}

// F-2.5, F-6.3: Bestätigungsseite, Löschen, Sperre bei Historie.
func TestSkiDelete(t *testing.T) {
	s, svc := newApp(t)
	fresh := addSki(t, svc)
	used := addSki(t, svc)
	p := addPerson(t, svc, "TEST0001")
	_, _ = rentNow(svc, used.ID, p.ID)

	_, body := get(t, s, "/ski/"+itoa(fresh.ID)+"/loeschen")
	assertContains(t, body, "Wirklich", "Endgültig löschen", `action="/ski/`+itoa(fresh.ID)+`/loeschen"`)
	assertRedirect(t, postForm(s, "/ski/"+itoa(fresh.ID)+"/loeschen", nil), "/ski?msg=ski_deleted")
	if code, _ := get(t, s, "/ski/"+itoa(fresh.ID)); code != http.StatusNotFound {
		t.Errorf("gelöschter Ski: Status = %d, want 404", code)
	}

	_, body = get(t, s, "/ski/"+itoa(used.ID)+"/loeschen")
	assertContains(t, body, "Ski hat Ausleih-Historie")
	if strings.Contains(body, "Endgültig löschen") {
		t.Error("Lösch-Button trotz Historie angezeigt")
	}
	rec := postForm(s, "/ski/"+itoa(used.ID)+"/loeschen", nil)
	if rec.Code != http.StatusConflict {
		t.Errorf("Status = %d, want 409", rec.Code)
	}
	if _, err := svc.SkiDetail(used.ID); err != nil {
		t.Error("Ski mit Historie wurde gelöscht")
	}
}
