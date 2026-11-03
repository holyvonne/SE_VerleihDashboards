package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"skiverleih/internal/domain"
	"skiverleih/internal/service"
	"skiverleih/internal/store"
)

// ---------- Test-Hilfen: echte App mit In-Memory-Store ----------

// testNow ist der feste Zeitpunkt der Handler-Tests (Montag, 02.11.2026 14:05 Ortszeit).
var testNow = time.Date(2026, 11, 2, 14, 5, 0, 0, time.Local)

func newApp(t *testing.T) (*Server, *service.Service) {
	s, svc, _ := newAppWithClock(t)
	return s, svc
}

// newAppWithClock liefert zusätzlich die Uhr, die ein Test vorstellen kann.
func newAppWithClock(t *testing.T) (*Server, *service.Service, *time.Time) {
	t.Helper()
	now := testNow
	svc := service.New(store.NewMemoryStore(), func() time.Time { return now }, nil)
	s, err := New(svc)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s, svc, &now
}

// rentNow bucht ab jetzt für einen Tag (aktive Ausleihe).
func rentNow(svc *service.Service, skiID, personID int) (domain.Rental, error) {
	now := svc.Now()
	return svc.Rent(service.RentalInput{SkiID: skiID, PersonID: personID, Start: now, End: now.Add(24 * time.Hour)})
}

func serve(s *Server, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func get(t *testing.T, s *Server, path string) (int, string) {
	t.Helper()
	rec := serve(s, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Code, rec.Body.String()
}

func postForm(s *Server, path string, vals url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return serve(s, req)
}

// postMultipart sendet ein Personenformular; image == nil bedeutet „keine Datei gewählt“.
func postMultipart(t *testing.T, s *Server, path string, fields map[string]string, image []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	if image != nil {
		fw, err := mw.CreateFormFile("id_image", "testbild.png")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fw.Write(image)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return serve(s, req)
}

func assertContains(t *testing.T, body string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(body, w) {
			t.Errorf("Antwort enthält %q nicht", w)
		}
	}
}

func assertRedirect(t *testing.T, rec *httptest.ResponseRecorder, wantLocation string) {
	t.Helper()
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("Status = %d, want 303; Body: %.300s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != wantLocation {
		t.Errorf("Location = %q, want %q", loc, wantLocation)
	}
}

// Fiktive Testdaten (NF-11).
func addSki(t *testing.T, svc *service.Service) domain.Ski {
	t.Helper()
	sk, err := svc.CreateSki(service.SkiInput{Brand: "Gipfelwerk", Model: "Pro", LengthCM: 170, Type: domain.SkiTypeAllround, Condition: domain.ConditionGood})
	if err != nil {
		t.Fatal(err)
	}
	return sk
}

func addPerson(t *testing.T, svc *service.Service, idNumber string) domain.Person {
	t.Helper()
	p, err := svc.CreatePerson(service.PersonInput{FirstName: "Erika", LastName: "Mustermann", IDType: domain.IDPassport, IDNumber: idNumber})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// ---------- Dashboard (F-1) ----------

// AK-1: Kennzahlen aus echten Daten inkl. Wartungshinweis.
func TestDashboard_ShowsStats(t *testing.T) {
	s, svc := newApp(t)
	var skis []domain.Ski
	for i := 0; i < 5; i++ {
		skis = append(skis, addSki(t, svc))
	}
	p1, p2 := addPerson(t, svc, "TEST0001"), addPerson(t, svc, "TEST0002")
	addPerson(t, svc, "TEST0003")
	_, _ = rentNow(svc, skis[0].ID, p1.ID)
	_, _ = rentNow(svc, skis[1].ID, p2.ID)
	_, _ = svc.UpdateSki(skis[2].ID, service.SkiInput{Brand: "Gipfelwerk", Model: "Pro", LengthCM: 170, Type: domain.SkiTypeAllround, Condition: domain.ConditionGood, Status: domain.SkiMaintenance})

	code, body := get(t, s, "/")
	if code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", code)
	}
	assertContains(t, body,
		`data-kpi="available">2<`, `data-kpi="rented">2<`, `data-kpi="total">5<`, `data-kpi="persons">3<`,
		"davon 1 in Wartung",
	)
}

// AK-2: leerer Bestand.
func TestDashboard_Empty(t *testing.T) {
	s, _ := newApp(t)
	code, body := get(t, s, "/")
	if code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", code)
	}
	assertContains(t, body, `data-kpi="available">0<`, `data-kpi="total">0<`, `data-kpi="reserved">0<`,
		"Keine aktiven Ausleihen", "Keine anstehenden Reservierungen")
	if strings.Contains(body, "in Wartung") || strings.Contains(body, "überfällig") {
		t.Error("Hinweis bei 0 Ski angezeigt")
	}
}

// AK-15, F-1.3: Reservierung zählt getrennt; zur Startzeit wird sie zur Ausleihe.
func TestDashboard_ReservationsSeparate(t *testing.T) {
	s, svc, clock := newAppWithClock(t)
	sk := addSki(t, svc)
	p := addPerson(t, svc, "TEST0001")
	start := testNow.Add(48 * time.Hour)
	r, err := svc.Rent(service.RentalInput{SkiID: sk.ID, PersonID: p.ID, Start: start, End: start.Add(24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}

	_, body := get(t, s, "/")
	assertContains(t, body,
		`data-kpi="available">1<`, `data-kpi="rented">0<`, `data-kpi="reserved">1<`,
		"Keine aktiven Ausleihen", "04.11.2026 14:05", "40,00 €",
		`action="/ausleihen/`+itoa(r.ID)+`/stornieren"`)

	*clock = start.Add(time.Hour) // Beginn erreicht
	_, body = get(t, s, "/")
	assertContains(t, body, `data-kpi="available">0<`, `data-kpi="rented">1<`, `data-kpi="reserved">0<`,
		"Keine anstehenden Reservierungen", `action="/ausleihen/`+itoa(r.ID)+`/rueckgabe"`)

	*clock = start.Add(48 * time.Hour) // geplante Rückgabe vorbei
	_, body = get(t, s, "/")
	assertContains(t, body, "davon 1 überfällig", "badge-overdue")
}

// F-1.2, F-1.3, F-6.1: Schnellzugriffe, Rückgabe-Button, Navigation.
func TestDashboard_QuickLinksAndRecentRentals(t *testing.T) {
	s, svc := newApp(t)
	sk := addSki(t, svc)
	p := addPerson(t, svc, "TEST0001")
	r, _ := rentNow(svc, sk.ID, p.ID)

	_, body := get(t, s, "/")
	assertContains(t, body,
		`href="/ausleihen/neu"`, `href="/ausleihen?status=aktiv"`, `href="/ski/neu"`, `href="/personen/neu"`,
		"Gipfelwerk Pro (170 cm)", "Erika Mustermann", "03.11.2026 14:05", // geplante Rückgabe
		`action="/ausleihen/`+itoa(r.ID)+`/rueckgabe"`,
		`class="active" aria-current="page"`,
	)
	if strings.Contains(body, "Keine aktiven Ausleihen") {
		t.Error("Leertext trotz aktiver Ausleihe angezeigt")
	}
}

func TestStaticCSS(t *testing.T) {
	s, _ := newApp(t)
	code, body := get(t, s, "/static/style.css")
	if code != http.StatusOK || !strings.Contains(body, ".kpi") {
		t.Errorf("Status = %d, CSS nicht ausgeliefert", code)
	}
}

func TestNotFound(t *testing.T) {
	s, _ := newApp(t)
	for _, path := range []string{"/gibt-es-nicht", "/ski/999", "/ski/abc", "/personen/999", "/personen/0/bearbeiten"} {
		code, body := get(t, s, path)
		if code != http.StatusNotFound {
			t.Errorf("%s: Status = %d, want 404", path, code)
		}
		assertContains(t, body, "Zum Dashboard")
	}
}

func TestDashboard_PostNotAllowed(t *testing.T) {
	s, _ := newApp(t)
	if rec := serve(s, httptest.NewRequest(http.MethodPost, "/", nil)); rec.Code == http.StatusOK {
		t.Errorf("POST / lieferte 200")
	}
}

// F-6.2: nur bekannte Meldungs-Schlüssel, kein freier Text aus der URL.
func TestFlashOnlyKnownKeys(t *testing.T) {
	s, _ := newApp(t)
	_, body := get(t, s, "/?msg=rented")
	assertContains(t, body, "Ski wurde ausgeliehen.")
	_, body = get(t, s, "/?msg=%3Cscript%3Ealert(1)%3C/script%3E")
	if strings.Contains(body, "flash") && strings.Contains(body, "alert(1)") {
		t.Error("unbekannter Schlüssel wurde angezeigt")
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[time.Duration]string{
		0:                             "0 Min",
		59 * time.Minute:              "59 Min",
		3 * time.Hour:                 "3 Std",
		3*time.Hour + 30*time.Minute:  "3 Std 30 Min",
		51 * time.Hour:                "2 T 3 Std",
		48*time.Hour + 20*time.Minute: "2 T 0 Std",
	}
	for d, want := range cases {
		if got := formatDuration(d); got != want {
			t.Errorf("formatDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

func itoa(n int) string { return strconv.Itoa(n) }
