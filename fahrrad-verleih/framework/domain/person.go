package domain

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// IDType ist die Art des Ausweisdokuments (Spezifikation 4.1).
type IDType string

const (
	IDCard          IDType = "personalausweis"
	IDPassport      IDType = "reisepass"
	IDDriverLicense IDType = "fuehrerschein"
)

var IDTypes = []IDType{IDCard, IDPassport, IDDriverLicense}

var idTypeLabels = map[IDType]string{
	IDCard:          "Personalausweis",
	IDPassport:      "Reisepass",
	IDDriverLicense: "Führerschein",
}

func (t IDType) Valid() bool { _, ok := idTypeLabels[t]; return ok }

func (t IDType) Label() string {
	if l, ok := idTypeLabels[t]; ok {
		return l
	}
	return string(t)
}

// MaxIDImageBytes ist die maximale Größe eines Ausweisbildes (2 MB).
const MaxIDImageBytes = 2 << 20

// MsgInvalidImage steht am Bildfeld und auf der 413-Fehlerseite (AK-26).
const MsgInvalidImage = "Ausweisbild muss JPEG oder PNG und höchstens 2 MB groß sein"

// IDImage ist ein optionales Ausweisbild im Arbeitsspeicher (Spezifikation 4.1a).
type IDImage struct {
	Data        []byte
	ContentType string
}

// NewIDImage prüft Größe und Inhalt. Der Typ wird aus den Bytes bestimmt,
// nicht aus Dateiname oder Browser-Angabe.
func NewIDImage(data []byte) (*IDImage, error) {
	if len(data) == 0 || len(data) > MaxIDImageBytes {
		return nil, ValidationErrors{"id_image": MsgInvalidImage}
	}
	ct := http.DetectContentType(data)
	if ct != "image/jpeg" && ct != "image/png" {
		return nil, ValidationErrors{"id_image": MsgInvalidImage}
	}
	return &IDImage{Data: data, ContentType: ct}, nil
}

// Person ist eine registrierte Kundin oder ein registrierter Kunde.
type Person struct {
	ID        int
	FirstName string
	LastName  string
	Phone     string
	Email     string
	IDType    IDType
	IDNumber  string
	IDImage   *IDImage // optional
	CreatedAt time.Time
}

// Normalize trimmt Textfelder und speichert die Ausweisnummer in Großbuchstaben.
func (p *Person) Normalize() {
	p.FirstName = strings.TrimSpace(p.FirstName)
	p.LastName = strings.TrimSpace(p.LastName)
	p.Phone = strings.TrimSpace(p.Phone)
	p.Email = strings.TrimSpace(p.Email)
	p.IDNumber = strings.ToUpper(strings.TrimSpace(p.IDNumber))
}

// Validate prüft die Feldregeln aus Spezifikation 4.1.
// Die Eindeutigkeit der Ausweisnummer (I-5) prüft der Service, weil sie alle Personen betrifft.
func (p Person) Validate() ValidationErrors {
	errs := ValidationErrors{}
	requireText(errs, "first_name", "Vorname", p.FirstName, 50)
	requireText(errs, "last_name", "Nachname", p.LastName, 50)

	optionalText(errs, "phone", "Telefon", p.Phone, 30)
	if !validPhone(p.Phone) {
		errs.Add("phone", "Telefon darf nur Ziffern, Leerzeichen und + - / ( ) enthalten")
	}
	optionalText(errs, "email", "E-Mail", p.Email, 100)
	if p.Email != "" && !strings.Contains(p.Email, "@") {
		errs.Add("email", "Bitte eine gültige E-Mail-Adresse angeben")
	}

	if !p.IDType.Valid() {
		errs.Add("id_type", "Bitte einen gültigen Ausweistyp wählen")
	}
	if n := utf8.RuneCountInString(p.IDNumber); n < 4 || n > 30 || !alphanumeric(p.IDNumber) {
		errs.Add("id_number", "Ausweisnummer muss 4–30 Zeichen lang sein und darf nur Buchstaben und Ziffern enthalten")
	}
	return errs
}

// FullName liefert „Vorname Nachname“.
func (p Person) FullName() string { return p.FirstName + " " + p.LastName }

// HasIDImage meldet, ob ein Ausweisbild hinterlegt ist.
func (p Person) HasIDImage() bool { return p.IDImage != nil }

// MaskedIDNumber zeigt nur die letzten vier Zeichen, z. B. „****0001“.
func (p Person) MaskedIDNumber() string {
	if len(p.IDNumber) <= 4 {
		return "****"
	}
	return "****" + p.IDNumber[len(p.IDNumber)-4:]
}

// IDKey ist der Schlüssel für die Eindeutigkeit von Ausweistyp + Ausweisnummer (I-5).
func (p Person) IDKey() string { return string(p.IDType) + ":" + p.IDNumber }

// Copy liefert eine tiefe Kopie, damit Aufrufer das gespeicherte Bild nicht verändern.
func (p Person) Copy() Person {
	if p.IDImage != nil {
		img := *p.IDImage
		img.Data = append([]byte(nil), p.IDImage.Data...)
		p.IDImage = &img
	}
	return p
}

// alphanumeric erlaubt nur A–Z und 0–9; Normalize hat bereits in Großbuchstaben gewandelt.
func alphanumeric(s string) bool {
	for _, r := range s {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func validPhone(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && !strings.ContainsRune(" +-/()", r) {
			return false
		}
	}
	return true
}
