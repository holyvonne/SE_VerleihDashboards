package handler

import (
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"skiverleih/internal/service"
)

var (
	hrefRe  = regexp.MustCompile(`href="([^"]+)"`)
	formRe  = regexp.MustCompile(`(?s)<form method="post" action="([^"]+)"([^>]*)>(.*?)</form>`)
	inputRe = regexp.MustCompile(`<input type="hidden" name="([^"]+)" value="([^"]*)">`)
	kpiRe   = regexp.MustCompile(`data-kpi="(\w+)">(\d+)<`)
)

func newDemoApp(t *testing.T) (*Server, *service.Service, *time.Time) {
	t.Helper()
	s, svc, now := newAppWithClock(t)
	if err := service.LoadDemo(svc); err != nil {
		t.Fatal(err)
	}
	return s, svc, now
}

type crawledForm struct {
	action    string
	multipart bool
	values    url.Values
}

// crawl folgt ab „/“ allen internen Links und sammelt alle POST-Formulare (AK-23).
func crawl(t *testing.T, s *Server) (pages map[string]int, forms map[string]crawledForm) {
	t.Helper()
	pages = map[string]int{}
	forms = map[string]crawledForm{}
	queue := []string{"/"}
	pages["/"] = 0
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		rec := serve(s, httptest.NewRequest(http.MethodGet, path, nil))
		pages[path] = rec.Code
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
			forms[action+"?"+v.Encode()] = crawledForm{action: action, multipart: strings.Contains(m[2], "multipart"), values: v}
		}
		for _, m := range hrefRe.FindAllStringSubmatch(body, -1) {
			link := html.UnescapeString(m[1])
			if !strings.HasPrefix(link, "/") || strings.HasPrefix(link, "//") || strings.HasPrefix(link, "/static/") {
				continue
			}
			if _, seen := pages[link]; !seen {
				pages[link] = 0
				queue = append(queue, link)
			}
		}
	}
	return pages, forms
}

// AK-23: Jeder Menüpunkt und jeder Link führt zu einer gültigen Seite – mit und ohne Demo-Daten.
func TestCrawlAllLinks(t *testing.T) {
	for _, tc := range []struct {
		name     string
		app      func(*testing.T) *Server
		minPages int
	}{
		{"mit Demo-Daten", func(t *testing.T) *Server { s, _, _ := newDemoApp(t); return s }, 30},
		{"ohne Daten", func(t *testing.T) *Server { s, _ := newApp(t); return s }, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pages, _ := crawl(t, tc.app(t))
			for path, code := range pages {
				if code != http.StatusOK {
					t.Errorf("GET %s: Status %d", path, code)
				}
			}
			for _, must := range []string{"/ski", "/personen", "/ausleihen", "/ausleihen/neu", "/ski/neu", "/personen/neu"} {
				if _, ok := pages[must]; !ok {
					t.Errorf("Menüpunkt/Link %s fehlt", must)
				}
			}
			t.Logf("%d Seiten erreicht", len(pages))
			if len(pages) < tc.minPages {
				t.Errorf("nur %d Seiten erreicht", len(pages))
			}
		})
	}
}

// AK-23: Jeder Button (POST-Formular) hat eine Route und antwortet sinnvoll.
// Jedes Formular wird mit seinen versteckten Feldern in einer frischen Demo-App abgeschickt.
func TestEveryFormButtonResponds(t *testing.T) {
	s, _, _ := newDemoApp(t)
	_, forms := crawl(t, s)
	t.Logf("%d verschiedene POST-Formulare gefunden", len(forms))
	if len(forms) < 10 {
		t.Fatalf("nur %d Formulare gefunden", len(forms))
	}
	allowed := map[int]bool{http.StatusOK: true, http.StatusSeeOther: true, http.StatusConflict: true,
		http.StatusUnprocessableEntity: true, http.StatusRequestEntityTooLarge: true}
	for key, f := range forms {
		fresh, _, _ := newDemoApp(t)
		var rec *httptest.ResponseRecorder
		if f.multipart {
			fields := map[string]string{}
			for k := range f.values {
				fields[k] = f.values.Get(k)
			}
			rec = postMultipart(t, fresh, f.action, fields, nil)
		} else {
			rec = postForm(fresh, f.action, f.values)
		}
		if !allowed[rec.Code] {
			t.Errorf("POST %s: Status %d", key, rec.Code)
		}
	}
}

func kpis(t *testing.T, s *Server) map[string]string {
	t.Helper()
	_, body := get(t, s, "/")
	out := map[string]string{}
	for _, m := range kpiRe.FindAllStringSubmatch(body, -1) {
		out[m[1]] = m[2]
	}
	return out
}

// AK-1, F-1: Kennzahlen der Demo-Daten und nach jeder Aktion.
func TestDashboardFollowsActions(t *testing.T) {
	s, svc, _ := newDemoApp(t)
	expect := func(step string, want map[string]string) {
		t.Helper()
		got := kpis(t, s)
		for k, v := range want {
			if got[k] != v {
				t.Errorf("%s: %s = %s, want %s (alle: %v)", step, k, got[k], v, got)
			}
		}
	}
	expect("Demo", map[string]string{"available": "5", "rented": "2", "reserved": "1", "total": "8", "persons": "4"})
	_, body := get(t, s, "/")
	assertContains(t, body, "davon 1 überfällig", "davon 1 in Wartung")

	// Ski 2 ab jetzt ausleihen.
	if _, err := rentNow(svc, 2, 4); err != nil {
		t.Fatal(err)
	}
	expect("nach Ausleihe", map[string]string{"available": "4", "rented": "3"})

	// Neue Reservierung für Ski 4 in drei Tagen.
	start := testNow.Add(72 * time.Hour)
	if _, err := svc.Rent(service.RentalInput{SkiID: 4, PersonID: 4, Start: start, End: start.Add(24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	expect("nach Reservierung", map[string]string{"available": "4", "reserved": "2"})

	// Überfällige Ausleihe (Ski 7) zurückgeben.
	var overdueID int
	for _, v := range svc.Rentals(service.FilterActive) {
		if v.Overdue {
			overdueID = v.Rental.ID
		}
	}
	assertRedirect(t, postForm(s, "/ausleihen/"+itoa(overdueID)+"/rueckgabe", url.Values{"zurueck": {"/"}}), "/?msg=returned")
	expect("nach Rückgabe", map[string]string{"available": "5", "rented": "2"})
	if _, body := get(t, s, "/"); strings.Contains(body, "überfällig") {
		t.Error("nach der Rückgabe darf nichts mehr überfällig sein")
	}

	// Reservierung stornieren.
	reserved := svc.Rentals(service.FilterReserved)
	assertRedirect(t, postForm(s, "/ausleihen/"+itoa(reserved[0].Rental.ID)+"/stornieren", url.Values{"zurueck": {"/"}}), "/?msg=cancelled")
	expect("nach Storno", map[string]string{"reserved": "1"})
}

// AK-22 über HTTP: Konfliktkarte, Badge „Wartet auf Rückgabe“, Rückgabe gesperrt, Storno möglich.
func TestOverdueConflictHTTP(t *testing.T) {
	s, svc, now := newAppWithClock(t)
	ski := addSki(t, svc)
	p1, p2 := addPerson(t, svc, "TEST0001"), addPerson(t, svc, "TEST0002")
	if _, err := svc.Rent(service.RentalInput{SkiID: ski.ID, PersonID: p1.ID, Start: testNow, End: testNow.Add(2 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	b, err := svc.Rent(service.RentalInput{SkiID: ski.ID, PersonID: p2.ID, Start: testNow.Add(3 * time.Hour), End: testNow.Add(5 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}

	*now = testNow.Add(150 * time.Minute) // erste Ausleihe überfällig
	_, body := get(t, s, "/")
	assertContains(t, body, "1 Buchung ist durch verspätete Rückgaben gefährdet", "badge-conflict", "davon 1 überfällig")

	*now = testNow.Add(210 * time.Minute) // Beginn von b erreicht
	_, body = get(t, s, "/ausleihen?status=reserviert")
	assertContains(t, body, "Wartet auf Rückgabe", "Konflikt", "/ausleihen/"+itoa(b.ID)+"/stornieren")
	if _, active := get(t, s, "/ausleihen?status=aktiv"); strings.Count(active, "badge-overdue\">Überfällig") != 1 {
		t.Error("unter „Aktiv“ muss genau die überfällige Ausleihe stehen")
	}
	rec := postForm(s, "/ausleihen/"+itoa(b.ID)+"/rueckgabe", url.Values{"zurueck": {"/"}})
	if rec.Code != http.StatusConflict {
		t.Errorf("Rückgabe einer wartenden Buchung: Status %d, want 409", rec.Code)
	}
	assertRedirect(t, postForm(s, "/ausleihen/"+itoa(b.ID)+"/stornieren", url.Values{"zurueck": {"/"}}), "/?msg=cancelled")
	if _, body := get(t, s, "/"); strings.Contains(body, "gefährdet") {
		t.Error("nach dem Storno darf keine Konfliktkarte mehr erscheinen")
	}
}
