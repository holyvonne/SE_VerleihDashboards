package service

import (
	"fmt"
	"time"

	"skiverleih/internal/domain"
)

// LoadDemo füllt den Service mit Beispieldaten für Präsentationen (NF-7).
// Alle Daten sind frei erfunden (NF-11): Marken, Namen, Ausweisnummern (Präfix TEST)
// und E-Mail-Adressen auf example.com. Ausweisbilder werden nicht mitgeliefert.
func LoadDemo(s *Service) error {
	skis := []SkiInput{
		{Brand: "Gipfelwerk", Model: "Allmountain 80", LengthCM: 170, Type: domain.SkiTypeAllround, Condition: domain.ConditionGood},
		{Brand: "Gipfelwerk", Model: "Allmountain 80", LengthCM: 160, Type: domain.SkiTypeAllround, Condition: domain.ConditionNew},
		{Brand: "Firnflug", Model: "Carve RS", LengthCM: 165, Type: domain.SkiTypeCarving, Condition: domain.ConditionGood},
		{Brand: "Firnflug", Model: "Carve RS", LengthCM: 175, Type: domain.SkiTypeCarving, Condition: domain.ConditionUsed},
		{Brand: "Pulverfee", Model: "Powder 104", LengthCM: 184, Type: domain.SkiTypeFreeride, Condition: domain.ConditionGood},
		{Brand: "Hüttenwirt", Model: "Tour Light", LengthCM: 172, Type: domain.SkiTypeTouring, Condition: domain.ConditionNew},
		{Brand: "Zwergenschanz", Model: "Junior", LengthCM: 110, Type: domain.SkiTypeKids, Condition: domain.ConditionGood},
		{Brand: "Zwergenschanz", Model: "Junior", LengthCM: 120, Type: domain.SkiTypeKids, Condition: domain.ConditionUsed, Note: "Kante vorne links prüfen"},
	}
	persons := []PersonInput{
		{FirstName: "Erika", LastName: "Mustermann", Phone: "+43 660 0000001", Email: "erika@example.com", IDType: domain.IDCard, IDNumber: "TEST0001"},
		{FirstName: "Max", LastName: "Muster", Phone: "+43 660 0000002", Email: "max@example.com", IDType: domain.IDPassport, IDNumber: "TEST0002"},
		{FirstName: "Lena", LastName: "Beispiel", Email: "lena@example.com", IDType: domain.IDDriverLicense, IDNumber: "TEST0003"},
		{FirstName: "Paul", LastName: "Probe", Phone: "0660 0000004", IDType: domain.IDCard, IDNumber: "TEST0004"},
	}

	var skiIDs, personIDs []int
	for _, in := range skis {
		sk, err := s.CreateSki(in)
		if err != nil {
			return fmt.Errorf("demo ski: %w", err)
		}
		skiIDs = append(skiIDs, sk.ID)
	}
	for _, in := range persons {
		p, err := s.CreatePerson(in)
		if err != nil {
			return fmt.Errorf("demo person: %w", err)
		}
		personIDs = append(personIDs, p.ID)
	}

	// Ein Ski in Wartung (F-1.1 Hinweis).
	maint := skis[7]
	maint.Status = domain.SkiMaintenance
	if _, err := s.UpdateSki(skiIDs[7], maint); err != nil {
		return fmt.Errorf("demo wartung: %w", err)
	}

	// Buchungen mit Beginn in der Vergangenheit werden so angelegt, als wären sie
	// zu ihrem Beginn gebucht worden (rentAt mit „damals“ als Uhrzeit).
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	hour := time.Hour
	bookings := []struct {
		ski, person int
		start, end  time.Duration // relativ zu jetzt
	}{
		{skiIDs[0], personIDs[0], -26 * hour, 46 * hour},  // aktiv
		{skiIDs[6], personIDs[0], -26 * hour, -2 * hour},  // aktiv, überfällig
		{skiIDs[2], personIDs[1], -72 * hour, -24 * hour}, // wird unten zurückgegeben
		{skiIDs[4], personIDs[2], 72 * hour, 120 * hour},  // Reservierung
	}
	var ids []int
	for _, b := range bookings {
		start := now.Add(b.start)
		booked := start
		if b.start > 0 {
			booked = now
		}
		r, err := s.rentAt(RentalInput{SkiID: b.ski, PersonID: b.person, Start: start, End: now.Add(b.end)}, booked)
		if err != nil {
			return fmt.Errorf("demo buchung: %w", err)
		}
		ids = append(ids, r.ID)
	}
	// Vier Stunden zu spät zurückgegeben.
	if _, err := s.returnAt(ids[2], now.Add(-20*hour)); err != nil {
		return fmt.Errorf("demo rückgabe: %w", err)
	}
	return nil
}
