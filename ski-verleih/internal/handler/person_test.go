package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Minimale fiktive Testbilder – nur der Dateianfang, an dem der Typ erkannt wird.
var (
	testPNG  = []byte("\x89PNG\r\n\x1a\n" + "fiktives-testbild")
	testJPEG = []byte("\xFF\xD8\xFF\xE0" + "fiktives-testbild")
)

func personFields() map[string]string {
	return map[string]string{
		"first_name": "Max", "last_name": "Muster", "phone": "+43 660 0000002",
		"email": "max@example.com", "id_type": "reisepass", "id_number": "test0002",
	}
}

// F-3.2, AK-7: Registrieren ohne Bild.
func TestPersonCreate_WithoutImage(t *testing.T) {
	s, _ := newApp(t)
	assertRedirect(t, postMultipart(t, s, "/personen", personFields(), nil), "/personen/1?msg=person_created")
	_, body := get(t, s, "/personen/1")
	assertContains(t, body, "Max Muster", "Reisepass", "TEST0002", "Kein Ausweisbild hinterlegt")
}

// F-3.2, Spezifikation 3.4: Bild hochladen und über eigene Route abrufen.
func TestPersonCreate_WithImage(t *testing.T) {
	s, svc := newApp(t)
	assertRedirect(t, postMultipart(t, s, "/personen", personFields(), testPNG), "/personen/1?msg=person_created")

	_, body := get(t, s, "/personen/1")
	assertContains(t, body, `src="/personen/1/ausweisbild"`)

	rec := serve(s, newGet("/personen/1/ausweisbild"))
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), testPNG) {
		t.Fatalf("Bild: Status = %d, %d Bytes", rec.Code, rec.Body.Len())
	}
	for header, want := range map[string]string{
		"Content-Type": "image/png", "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if p, _ := svc.Person(1); !p.HasIDImage() {
		t.Error("Bild nicht gespeichert")
	}
}

// AK-6: PDF, zu großes Bild (Feldfehler) und sehr große Anfrage (413).
func TestPersonCreate_InvalidImage(t *testing.T) {
	s, svc := newApp(t)

	rec := postMultipart(t, s, "/personen", personFields(), []byte("%PDF-1.7 fiktiv"))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("PDF: Status = %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Ausweisbild muss JPEG oder PNG und höchstens 2 MB groß sein", `value="Max"`)

	big := append(bytes.Clone(testPNG), make([]byte, 2<<20)...) // knapp über 2 MB
	rec = postMultipart(t, s, "/personen", personFields(), big)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("2 MB+: Status = %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "höchstens 2 MB")

	huge := append(bytes.Clone(testPNG), make([]byte, 4<<20)...)
	rec = postMultipart(t, s, "/personen", personFields(), huge)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("4 MB: Status = %d, want 413", rec.Code)
	}
	assertContains(t, rec.Body.String(), "höchstens 2 MB")

	if n := len(svc.Persons("")); n != 0 {
		t.Errorf("%d Personen gespeichert, want 0", n)
	}
}

// AK-5: doppelte Ausweisnummer (andere Schreibweise).
func TestPersonCreate_DuplicateID(t *testing.T) {
	s, _ := newApp(t)
	postMultipart(t, s, "/personen", personFields(), nil)
	f := personFields()
	f["id_number"] = "Test0002"
	rec := postMultipart(t, s, "/personen", f, nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("Status = %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Diese Ausweisnummer ist bereits registriert")
}

// F-3.1: Liste mit Suche und maskierter Ausweisnummer.
func TestPersonList(t *testing.T) {
	s, svc := newApp(t)
	_, body := get(t, s, "/personen")
	assertContains(t, body, "Noch keine Personen registriert.")

	addPerson(t, svc, "TEST1234")
	_, body = get(t, s, "/personen")
	assertContains(t, body, "Erika Mustermann", "****1234")
	if strings.Contains(body, "TEST1234") {
		t.Error("vollständige Ausweisnummer in der Liste sichtbar")
	}
	_, body = get(t, s, "/personen?q=erika")
	assertContains(t, body, "Erika Mustermann")
	_, body = get(t, s, "/personen?q=niemand")
	assertContains(t, body, "Keine Person passt zu")
}

// F-3.4: Bearbeiten – Bild bleibt, wird ersetzt oder entfernt.
func TestPersonUpdate_Image(t *testing.T) {
	s, svc := newApp(t)
	postMultipart(t, s, "/personen", personFields(), testPNG)

	_, body := get(t, s, "/personen/1/bearbeiten")
	assertContains(t, body, `value="Max"`, "Gespeichertes Ausweisbild entfernen", `enctype="multipart/form-data"`)

	f := personFields()
	f["last_name"] = "Musterfrau"
	assertRedirect(t, postMultipart(t, s, "/personen/1", f, nil), "/personen/1?msg=person_updated")
	p, _ := svc.Person(1)
	if p.LastName != "Musterfrau" || !p.HasIDImage() {
		t.Errorf("ohne neues Bild: %+v", p)
	}

	postMultipart(t, s, "/personen/1", f, testJPEG)
	if p, _ := svc.Person(1); p.IDImage.ContentType != "image/jpeg" {
		t.Error("Bild nicht ersetzt")
	}

	f["remove_image"] = "1"
	postMultipart(t, s, "/personen/1", f, nil)
	if p, _ := svc.Person(1); p.HasIDImage() {
		t.Error("Bild nicht entfernt")
	}
	if code, _ := get(t, s, "/personen/1/ausweisbild"); code != http.StatusNotFound {
		t.Errorf("Bild-Route nach Entfernen: %d, want 404", code)
	}
}

// F-3.5: Löschen nur ohne Historie.
func TestPersonDelete(t *testing.T) {
	s, svc := newApp(t)
	fresh := addPerson(t, svc, "TEST0001")
	renter := addPerson(t, svc, "TEST0002")
	_, _ = rentNow(svc, addSki(t, svc).ID, renter.ID)

	assertRedirect(t, postForm(s, "/personen/"+itoa(fresh.ID)+"/loeschen", nil), "/personen?msg=person_deleted")

	_, body := get(t, s, "/personen/"+itoa(renter.ID)+"/loeschen")
	assertContains(t, body, "Person hat Ausleih-Historie")
	if rec := postForm(s, "/personen/"+itoa(renter.ID)+"/loeschen", nil); rec.Code != http.StatusConflict {
		t.Errorf("Status = %d, want 409", rec.Code)
	}
}

func TestPersonCreate_NotMultipart(t *testing.T) {
	s, _ := newApp(t)
	if rec := postForm(s, "/personen", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", rec.Code)
	}
}

func newGet(path string) *http.Request { return httptest.NewRequest(http.MethodGet, path, nil) }
