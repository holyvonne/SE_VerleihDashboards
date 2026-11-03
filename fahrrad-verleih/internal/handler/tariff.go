package handler

import (
	"net/http"
	"time"

	"fahrradverleih/framework/domain"
	"fahrradverleih/plugins/fahrrad"
)

type tariffRow struct {
	Category domain.Category
	Rate     domain.Money
	Rule     string
}

type tariffExample struct {
	Category string
	Label    string // z. B. „10:00 – 12:15“
	Duration time.Duration
	Price    domain.Price
}

// Rechenbeispiele aus Spezifikation 4.6, berechnet mit den konfigurierten Sätzen (F-7).
var exampleSpecs = []struct {
	category string
	startH   int
	startM   int
	duration time.Duration
}{
	{fahrrad.Citybike, 10, 0, time.Hour},
	{fahrrad.Citybike, 10, 0, time.Hour + time.Minute},
	{fahrrad.Citybike, 10, 0, 2*time.Hour + 15*time.Minute},
	{fahrrad.Mountainbike, 9, 0, 3*time.Hour + 30*time.Minute},
	{fahrrad.EBike, 9, 0, 8 * time.Hour},
	{fahrrad.EBike, 18, 0, 16 * time.Hour},
}

// F-7, AK-18
func (s *Server) handleTariffs(w http.ResponseWriter, r *http.Request) {
	var rows []tariffRow
	for _, c := range s.bikes.Categories() {
		rows = append(rows, tariffRow{Category: c, Rate: s.bikes.Rate(c.Key), Rule: s.bikes.Pricing().Describe(c.Key)})
	}
	var examples []tariffExample
	for _, e := range exampleSpecs {
		c, ok := s.category(e.category)
		if !ok {
			continue
		}
		start := time.Date(2026, 11, 2, e.startH, e.startM, 0, 0, time.Local)
		end := start.Add(e.duration)
		label := start.Format("15:04") + " – " + end.Format("15:04")
		if end.Day() != start.Day() {
			label = start.Format("02.01. 15:04") + " – " + end.Format("02.01. 15:04")
		}
		price := s.bikes.Pricing().Quote(domain.Item{Kind: kind, Category: c.Key}, domain.Period{Start: start, End: end})
		examples = append(examples, tariffExample{Category: c.Label, Label: label, Duration: e.duration, Price: price})
	}
	limits := s.bikes.Limits()
	s.render(w, r, http.StatusOK, "tariffs.html", page{
		Title:    "Tarife",
		Subtitle: "Stundenpreise je Fahrradtyp",
		Active:   "tarife",
		Content: struct {
			Rows     []tariffRow
			Examples []tariffExample
			Limits   domain.BookingLimits
		}{rows, examples, limits},
	})
}
