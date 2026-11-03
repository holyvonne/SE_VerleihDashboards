package handler

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/plugin"
	"fahrradverleih/framework/service"
	"fahrradverleih/framework/store"
	"fahrradverleih/plugins/fahrrad"
)

// testNow: 10.11.2026 08:00 in der Zeitzone des Servers, weil die Formulare lokal geparst werden.
var testNow = time.Date(2026, 11, 10, 8, 0, 0, 0, time.Local)

func local(h, m int) time.Time { return time.Date(2026, 11, 10, h, m, 0, 0, time.Local) }

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time  { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) set(t time.Time) { c.mu.Lock(); defer c.mu.Unlock(); c.t = t }

type app struct {
	t     *testing.T
	srv   *Server
	svc   *service.Service
	clock *clock
}

func newApp(t *testing.T) *app { return newAppWithConfig(t, fahrrad.DefaultConfig()) }

func newAppWithConfig(t *testing.T, cfg fahrrad.Config) *app {
	t.Helper()
	bikes, err := fahrrad.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	reg := plugin.NewRegistry()
	if err := reg.Register(bikes); err != nil {
		t.Fatal(err)
	}
	c := &clock{t: testNow}
	svc := service.New(store.NewMemoryStore(), reg, c.now)
	srv, err := New(svc, bikes)
	if err != nil {
		t.Fatal(err)
	}
	return &app{t: t, srv: srv, svc: svc, clock: c}
}

func (a *app) serve(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	a.srv.ServeHTTP(rec, req)
	return rec
}

func (a *app) get(path string) *httptest.ResponseRecorder {
	return a.serve(httptest.NewRequest(http.MethodGet, path, nil))
}

func (a *app) post(path string, v url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return a.serve(req)
}

func (a *app) postMultipart(path string, fields map[string]string, file []byte) *httptest.ResponseRecorder {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	if file != nil {
		fw, _ := mw.CreateFormFile("id_image", "test.png")
		_, _ = fw.Write(file)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return a.serve(req)
}

func (a *app) addBike(category string) int {
	a.t.Helper()
	attrs := map[string]string{fahrrad.AttrBrand: "Testrad", fahrrad.AttrModel: category, fahrrad.AttrFrame: "M"}
	if category == fahrrad.EBike {
		attrs[fahrrad.AttrBattery] = "500"
	}
	it, err := a.svc.CreateItem(service.ItemInput{Kind: fahrrad.Key, Category: category, Attributes: attrs})
	if err != nil {
		a.t.Fatal(err)
	}
	return it.ID
}

var personNo int
var personMu sync.Mutex

func (a *app) addPerson() int {
	a.t.Helper()
	personMu.Lock()
	personNo++
	n := personNo
	personMu.Unlock()
	p, err := a.svc.CreatePerson(service.PersonInput{FirstName: "Test", LastName: fmt.Sprintf("Kunde%d", n), IDType: domain.IDCard, IDNumber: fmt.Sprintf("TEST%04d", n)})
	if err != nil {
		a.t.Fatal(err)
	}
	return p.ID
}

func (a *app) book(bike, person int, start, end time.Time) domain.Booking {
	a.t.Helper()
	b, err := a.svc.Book(service.BookingInput{ItemID: bike, PersonID: person, Start: start, End: end})
	if err != nil {
		a.t.Fatal(err)
	}
	return b
}

// bookingValues füllt das Buchungsformular mit „Später“ und festem Zeitraum.
func bookingValues(bike, person int, start, end time.Time) url.Values {
	return url.Values{
		"fahrrad": {fmt.Sprint(bike)}, "person": {fmt.Sprint(person)}, "start_mode": {startLater},
		"start_date": {start.Format(dateLayout)}, "start_time": {start.Format(timeLayout)},
		"end_date": {end.Format(dateLayout)}, "end_time": {end.Format(timeLayout)},
	}
}

var tokenRe = regexp.MustCompile(`name="geprueft" value="([^"]+)"`)

// confirm schickt erst die Vorschau und dann „Verbindlich buchen“ mit dem Prüfwert.
func (a *app) confirm(v url.Values) *httptest.ResponseRecorder {
	a.t.Helper()
	v.Set("aktion", "vorschau")
	rec := a.post("/buchungen", v)
	m := tokenRe.FindStringSubmatch(rec.Body.String())
	if m == nil {
		a.t.Fatalf("Vorschau ohne Prüfwert (Status %d): %s", rec.Code, rec.Body.String())
	}
	v.Set("aktion", "bestaetigen")
	v.Set("geprueft", htmlUnescape(m[1]))
	return a.post("/buchungen", v)
}

func htmlUnescape(s string) string { return strings.NewReplacer("&#43;", "+", "&amp;", "&").Replace(s) }

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("Status %d, want %d; Body: %s", rec.Code, want, truncate(rec.Body.String()))
	}
}

func assertContains(t *testing.T, rec *httptest.ResponseRecorder, parts ...string) {
	t.Helper()
	body := rec.Body.String()
	for _, p := range parts {
		if !strings.Contains(body, p) {
			t.Errorf("Antwort enthält %q nicht; Body: %s", p, truncate(body))
		}
	}
}

func assertRedirect(t *testing.T, rec *httptest.ResponseRecorder, prefix string) {
	t.Helper()
	assertStatus(t, rec, http.StatusSeeOther)
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, prefix) {
		t.Fatalf("Weiterleitung nach %q, want Präfix %q", loc, prefix)
	}
}

func truncate(s string) string {
	if len(s) > 1500 {
		return s[:1500] + " …"
	}
	return s
}
