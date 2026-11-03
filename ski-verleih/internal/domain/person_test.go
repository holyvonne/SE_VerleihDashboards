package domain

import (
	"bytes"
	"strings"
	"testing"
)

// Nur fiktive Testdaten (NF-11).
func validPerson() Person {
	p := Person{
		FirstName: "Erika",
		LastName:  "Mustermann",
		Phone:     "+43 660 000000",
		Email:     "erika@example.com",
		IDType:    IDPassport,
		IDNumber:  "TEST1234",
	}
	p.Normalize()
	return p
}

func TestPersonNormalize(t *testing.T) {
	p := Person{FirstName: " Max ", LastName: " Muster ", IDNumber: "  c01x00t47 "}
	p.Normalize()
	if p.FirstName != "Max" || p.LastName != "Muster" {
		t.Errorf("Namen nicht getrimmt: %+v", p)
	}
	if p.IDNumber != "C01X00T47" {
		t.Errorf("IDNumber = %q, want C01X00T47", p.IDNumber)
	}
}

func TestPersonValidate(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Person)
		wantField string
	}{
		{"gültig", func(*Person) {}, ""},
		{"ohne optionale Felder (AK-7)", func(p *Person) { p.Phone, p.Email = "", "" }, ""},
		{"Vorname leer", func(p *Person) { p.FirstName = "" }, "first_name"},
		{"Nachname 51 Zeichen", func(p *Person) { p.LastName = strings.Repeat("b", 51) }, "last_name"},
		{"Telefon mit Buchstaben", func(p *Person) { p.Phone = "0660-ABC" }, "phone"},
		{"Telefon erlaubte Zeichen", func(p *Person) { p.Phone = "+43 (0) 660/123-45" }, ""},
		{"Telefon 31 Zeichen", func(p *Person) { p.Phone = strings.Repeat("1", 31) }, "phone"},
		{"E-Mail ohne @", func(p *Person) { p.Email = "erika.example.com" }, "email"},
		{"Ausweistyp unbekannt", func(p *Person) { p.IDType = "bibliotheksausweis" }, "id_type"},
		{"Ausweisnummer 3 Zeichen", func(p *Person) { p.IDNumber = "ABC" }, "id_number"},
		{"Ausweisnummer 4 Zeichen", func(p *Person) { p.IDNumber = "AB12" }, ""},
		{"Ausweisnummer 30 Zeichen", func(p *Person) { p.IDNumber = strings.Repeat("A", 30) }, ""},
		{"Ausweisnummer 31 Zeichen", func(p *Person) { p.IDNumber = strings.Repeat("A", 31) }, "id_number"},
		{"Ausweisnummer mit Bindestrich", func(p *Person) { p.IDNumber = "TEST-1234" }, "id_number"},
		{"Ausweisnummer leer", func(p *Person) { p.IDNumber = "" }, "id_number"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validPerson()
			tt.mutate(&p)
			assertFieldError(t, p.Validate(), tt.wantField)
		})
	}
}

// AK-5: Groß-/Kleinschreibung darf die Eindeutigkeit nicht umgehen.
func TestPersonIDKey_CaseInsensitive(t *testing.T) {
	a := Person{IDType: IDPassport, IDNumber: "C01X00T47"}
	b := Person{IDType: IDPassport, IDNumber: "c01x00t47"}
	a.Normalize()
	b.Normalize()
	if a.IDKey() != b.IDKey() {
		t.Errorf("IDKey unterschiedlich: %q vs %q", a.IDKey(), b.IDKey())
	}
	c := Person{IDType: IDCard, IDNumber: "C01X00T47"}
	if a.IDKey() == c.IDKey() {
		t.Error("gleiche Nummer bei anderem Ausweistyp darf nicht kollidieren")
	}
}

// F-3.1: Ausweisnummer in Listen maskiert.
func TestPersonMaskedIDNumber(t *testing.T) {
	tests := map[string]string{
		"TEST1234":  "****1234",
		"C01X00T47": "****0T47",
		"AB12":      "****",
		"":          "****",
	}
	for in, want := range tests {
		p := Person{IDNumber: in}
		if got := p.MaskedIDNumber(); got != want {
			t.Errorf("MaskedIDNumber(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPersonFullNameAndImage(t *testing.T) {
	p := validPerson()
	if p.FullName() != "Erika Mustermann" {
		t.Errorf("FullName() = %q", p.FullName())
	}
	if p.HasIDImage() {
		t.Error("HasIDImage() = true ohne Bild (AK-7)")
	}
}

// Minimale Dateianfänge, an denen http.DetectContentType den Typ erkennt.
var (
	pngHeader  = []byte("\x89PNG\r\n\x1a\n" + "rest")
	jpegHeader = []byte("\xFF\xD8\xFF\xE0" + "rest")
	pdfHeader  = []byte("%PDF-1.7\n" + "rest")
)

// AK-6: nur JPEG/PNG bis 2 MB.
func TestNewIDImage(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		wantCT string // "" = Fehler erwartet
	}{
		{"PNG", pngHeader, "image/png"},
		{"JPEG", jpegHeader, "image/jpeg"},
		{"PDF abgelehnt", pdfHeader, ""},
		{"Text abgelehnt", []byte("hallo"), ""},
		{"leer abgelehnt", nil, ""},
		{"genau 2 MB", padTo(pngHeader, MaxIDImageBytes), "image/png"},
		{"über 2 MB", padTo(pngHeader, MaxIDImageBytes+1), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := NewIDImage(tt.data)
			if tt.wantCT == "" {
				assertFieldError(t, err, "id_image")
				if img != nil {
					t.Error("Bild trotz Fehler zurückgegeben")
				}
				return
			}
			if err != nil {
				t.Fatalf("unerwarteter Fehler: %v", err)
			}
			if img.ContentType != tt.wantCT {
				t.Errorf("ContentType = %q, want %q", img.ContentType, tt.wantCT)
			}
		})
	}
}

func padTo(prefix []byte, size int) []byte {
	return append(bytes.Clone(prefix), make([]byte, size-len(prefix))...)
}
