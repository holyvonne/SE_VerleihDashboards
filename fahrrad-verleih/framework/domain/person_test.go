package domain

import (
	"bytes"
	"testing"
)

// PNGHeader reicht http.DetectContentType, um ein PNG zu erkennen.
var pngHeader = []byte("\x89PNG\r\n\x1a\n0000IHDR")

func validPerson() Person {
	return Person{FirstName: "Erika", LastName: "Mustermann", Email: "erika@example.com", IDType: IDCard, IDNumber: "test0001"}
}

func TestPersonValidate(t *testing.T) {
	p := validPerson()
	p.Normalize()
	if errs := p.Validate(); len(errs) != 0 {
		t.Fatalf("gültige Person: %v", errs)
	}
	if p.IDNumber != "TEST0001" || p.MaskedIDNumber() != "****0001" {
		t.Errorf("Normalisierung/Maskierung: %q %q", p.IDNumber, p.MaskedIDNumber())
	}

	bad := Person{Phone: "abc", Email: "keine-mail", IDType: "x", IDNumber: "A-1"}
	errs := bad.Validate()
	for _, field := range []string{"first_name", "last_name", "phone", "email", "id_type", "id_number"} {
		if errs[field] == "" {
			t.Errorf("Feldfehler %q fehlt: %v", field, errs)
		}
	}
}

// AK-26: Bild optional, nur JPEG/PNG bis 2 MB.
func TestNewIDImage(t *testing.T) {
	img, err := NewIDImage(pngHeader)
	if err != nil || img.ContentType != "image/png" {
		t.Fatalf("PNG: %v %v", img, err)
	}
	if _, err := NewIDImage([]byte("nur Text")); err == nil {
		t.Error("Text darf kein Ausweisbild sein")
	}
	big := append(append([]byte(nil), pngHeader...), bytes.Repeat([]byte{0}, MaxIDImageBytes)...)
	if _, err := NewIDImage(big); err == nil {
		t.Error("über 2 MB muss abgelehnt werden")
	}
}

func TestPersonCopy(t *testing.T) {
	p := validPerson()
	p.IDImage = &IDImage{Data: []byte{1, 2}, ContentType: "image/png"}
	c := p.Copy()
	c.IDImage.Data[0] = 9
	if p.IDImage.Data[0] != 1 {
		t.Error("Person.Copy teilt die Bilddaten")
	}
}
