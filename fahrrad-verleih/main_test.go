package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// AK-18: ungültige Tarife verhindern den Start und nennen den Fehler.
func TestSetupRejectsInvalidRates(t *testing.T) {
	for spec, want := range map[string]string{
		"ebike=0":      "ebike",
		"roller=5":     "roller",
		"citybike=abc": "abc",
	} {
		if _, err := setup(spec, false, time.Now); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("setup(%q): %v, erwartet Fehler mit %q", spec, err, want)
		}
	}
}

func TestSetupWithDemoAndRates(t *testing.T) {
	srv, err := setup("citybike=6", true, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tarife", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `data-rate="citybike">6,00 €`) {
		t.Errorf("Tarifseite: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(rec.Body.String(), `data-kpi="total">9<`) {
		t.Error("Demo-Daten fehlen auf dem Dashboard")
	}
}
