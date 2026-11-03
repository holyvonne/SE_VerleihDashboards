// Package demo lädt fiktive Beispieldaten für den Fahrradverleih (NF-7, NF-11).
package demo

import (
	"fmt"
	"time"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/service"
	"fahrradverleih/plugins/fahrrad"
)

type bike struct {
	category, brand, model, frame, battery, note string
	maintenance                                  bool
}

var bikes = []bike{
	{category: fahrrad.Citybike, brand: "Gazelle", model: "Esprit", frame: "M"},
	{category: fahrrad.Citybike, brand: "Kalkhoff", model: "Agattu", frame: "L"},
	{category: fahrrad.Citybike, brand: "Pegasus", model: "Solero", frame: "S"},
	{category: fahrrad.Mountainbike, brand: "Cube", model: "Aim", frame: "L"},
	{category: fahrrad.Mountainbike, brand: "Canyon", model: "Grand Canyon", frame: "M"},
	{category: fahrrad.Mountainbike, brand: "Trek", model: "Marlin", frame: "XL", note: "Bremsen werden geprüft", maintenance: true},
	{category: fahrrad.EBike, brand: "Riese & Müller", model: "Charger4", frame: "M", battery: "625"},
	{category: fahrrad.EBike, brand: "Cube", model: "Kathmandu Hybrid", frame: "L", battery: "750"},
	{category: fahrrad.EBike, brand: "Haibike", model: "Trekking 6", frame: "M", battery: "500"},
}

var persons = []service.PersonInput{
	{FirstName: "Erika", LastName: "Mustermann", Phone: "+49 30 1234567", Email: "erika@example.com", IDType: domain.IDCard, IDNumber: "TEST0001"},
	{FirstName: "Max", LastName: "Mustermann", Email: "max@example.com", IDType: domain.IDPassport, IDNumber: "TEST0002"},
	{FirstName: "Lena", LastName: "Beispiel", Phone: "0151 2345678", IDType: domain.IDDriverLicense, IDNumber: "TEST0003"},
	{FirstName: "Jonas", LastName: "Test", Email: "jonas@example.com", IDType: domain.IDCard, IDNumber: "TEST0004"},
}

// Load legt 9 Fahrräder, 4 Personen und 5 Buchungen relativ zu now an:
// eine überfällige Ausleihe mit nachfolgender Reservierung (Konflikt), eine aktive Ausleihe,
// eine Reservierung für morgen und eine abgeschlossene Ausleihe.
func Load(svc *service.Service, now time.Time) error {
	ids := make([]int, len(bikes))
	for i, b := range bikes {
		attrs := map[string]string{fahrrad.AttrBrand: b.brand, fahrrad.AttrModel: b.model, fahrrad.AttrFrame: b.frame, fahrrad.AttrBattery: b.battery}
		status := domain.StatusAvailable
		if b.maintenance {
			status = domain.StatusMaintenance
		}
		it, err := svc.CreateItem(service.ItemInput{Kind: fahrrad.Key, Category: b.category, Attributes: attrs, Status: status, Note: b.note})
		if err != nil {
			return fmt.Errorf("Fahrrad %s %s: %w", b.brand, b.model, err)
		}
		ids[i] = it.ID
	}
	pids := make([]int, len(persons))
	for i, p := range persons {
		created, err := svc.CreatePerson(p)
		if err != nil {
			return fmt.Errorf("Person %s: %w", p.LastName, err)
		}
		pids[i] = created.ID
	}

	base := now.Truncate(15 * time.Minute)
	h := func(n float64) time.Time { return base.Add(time.Duration(n * float64(time.Hour))) }
	local := now.In(time.Local)
	tomorrow9 := time.Date(local.Year(), local.Month(), local.Day()+1, 9, 0, 0, 0, time.Local)

	bookings := []struct {
		bike, person int
		start, end   time.Time
	}{
		{0, 0, h(-5), h(-1)},                            // Erika, Citybike: überfällig
		{0, 2, h(2), h(5)},                              // Lena, gleiches Citybike: Konflikt
		{6, 1, h(-1), h(3)},                             // Max, E-Bike: aktiv
		{3, 3, h(-28), h(-24)},                          // Jonas, Mountainbike: wird zurückgegeben
		{7, 0, tomorrow9, tomorrow9.Add(4 * time.Hour)}, // Erika, E-Bike: Reservierung morgen
	}
	created := make([]domain.Booking, len(bookings))
	for i, b := range bookings {
		bookedAt := b.start
		if bookedAt.After(now) {
			bookedAt = h(-5) // Reservierungen wurden „früher“ angelegt
		}
		bk, err := svc.BookAsOf(service.BookingInput{ItemID: ids[b.bike], PersonID: pids[b.person], Start: b.start, End: b.end}, bookedAt)
		if err != nil {
			return fmt.Errorf("Buchung %d: %w", i+1, err)
		}
		created[i] = bk
	}
	_, err := svc.ReturnAsOf(created[3].ID, h(-23.5)) // eine halbe Stunde zu spät
	return err
}
