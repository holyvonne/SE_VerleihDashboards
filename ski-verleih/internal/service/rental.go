package service

import (
	"errors"
	"time"

	"skiverleih/internal/domain"
)

// RentalFilter ist der Filter der Buchungsübersicht (F-5.2).
type RentalFilter string

const (
	FilterActive   RentalFilter = "aktiv"
	FilterReserved RentalFilter = "reserviert"
	FilterClosed   RentalFilter = "abgeschlossen"
	FilterAll      RentalFilter = "alle"
)

// ParseRentalFilter liest den Filter aus der URL; Standard ist „aktiv“.
func ParseRentalFilter(s string) RentalFilter {
	switch f := RentalFilter(s); f {
	case FilterReserved, FilterClosed, FilterAll:
		return f
	default:
		return FilterActive
	}
}

// Rentals liefert die Buchungsübersicht (F-5). Neuester Beginn zuerst,
// Reservierungen mit dem nächsten Beginn zuerst (F-5.3).
func (s *Service) Rentals(filter RentalFilter) []RentalView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var views []RentalView
	for _, v := range s.views(s.repo.ListRentals()) {
		keep := true
		switch filter {
		case FilterActive:
			keep = v.State == domain.RentalActive
		case FilterReserved: // inklusive wartender Buchungen (F-5.2)
			keep = v.State == domain.RentalReserved || v.State == domain.RentalWaiting
		case FilterClosed:
			keep = v.State == domain.RentalClosed
		}
		if keep {
			views = append(views, v)
		}
	}
	if filter == FilterReserved {
		return soonestFirst(views)
	}
	return views
}

// RentalInput sind die Formularwerte einer neuen Buchung (F-4.1).
// Ein Nullwert bei Start oder End steht für ein fehlendes oder ungültiges Datum.
type RentalInput struct {
	SkiID    int
	PersonID int
	Start    time.Time
	End      time.Time
}

func (in RentalInput) period() domain.Period { return domain.Period{Start: in.Start, End: in.End} }

// Quote ist die geprüfte Preisvorschau einer Buchung (F-4.2).
type Quote struct {
	Ski           domain.Ski
	Person        domain.Person
	Period        domain.Period
	Price         domain.Money
	Explanation   string // z. B. „3 angefangene Kalendertage × 20,00 €“
	IsReservation bool   // Beginn liegt in der Zukunft
}

func (q Quote) Duration() time.Duration { return q.Period.Duration() }

// PricingRule beschreibt die Preisregel für das Formular (F-4.1).
func (s *Service) PricingRule() string { return s.pricing.Rule() }

// Quote prüft eine Buchung vollständig und berechnet den Preis, ohne zu speichern (F-4.2, AK-13).
func (s *Service) Quote(in RentalInput) (Quote, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.quote(in, s.now())
}

// quote prüft Ski, Person, Zeitraum (3.3), Wartung (AK-9) und Belegung (3.6, AK-16).
func (s *Service) quote(in RentalInput, now time.Time) (Quote, error) {
	errs := domain.ValidationErrors{}
	ski, err := s.repo.Ski(in.SkiID)
	if errors.Is(err, domain.ErrNotFound) {
		errs["ski_id"] = "Bitte einen Ski auswählen"
	} else if err != nil {
		return Quote{}, err
	}
	person, err := s.repo.Person(in.PersonID)
	if errors.Is(err, domain.ErrNotFound) {
		errs["person_id"] = "Bitte eine Person auswählen"
	} else if err != nil {
		return Quote{}, err
	}
	p := in.period()
	if err := mergeValidation(errs, domain.ValidateBooking(p, now)); err != nil {
		return Quote{}, err
	}
	if err := errOrNil(errs); err != nil {
		return Quote{}, err
	}

	if ski.InMaintenance() {
		return Quote{}, domain.ErrSkiInMaintenance
	}
	if _, conflict := domain.FindConflict(p, s.rentalsOfSki(ski.ID), now); conflict {
		return Quote{}, domain.ErrSkiBooked
	}
	return Quote{
		Ski: ski, Person: person, Period: p,
		Price:         s.pricing.Price(p),
		Explanation:   s.pricing.Explain(p),
		IsReservation: now.Before(p.Start),
	}, nil
}

// Rent legt die Buchung an (F-4.3). Prüfen und Anlegen passieren unter derselben
// Schreib-Sperre, damit parallele Anfragen keine Überschneidung erzeugen (AK-11, AK-20).
// Der Preis wird hier neu berechnet – gespeichert wird nie ein Wert aus dem Formular.
func (s *Service) Rent(in RentalInput) (domain.Rental, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rentAt(in, s.now())
}

func (s *Service) rentAt(in RentalInput, now time.Time) (domain.Rental, error) {
	q, err := s.quote(in, now)
	if err != nil {
		return domain.Rental{}, err
	}
	return s.repo.CreateRental(domain.NewRental(q.Ski.ID, q.Person.ID, q.Period, now, q.Price)), nil
}

// Return schließt eine aktive Ausleihe ab und berechnet den Endpreis (F-4.4, AK-17).
// Eine zweite Rückgabe (AK-10) und die Rückgabe einer Reservierung (AK-18) werden abgelehnt.
func (s *Service) Return(rentalID int) (domain.Rental, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.returnAt(rentalID, s.now())
}

func (s *Service) returnAt(rentalID int, at time.Time) (domain.Rental, error) {
	rental, err := s.repo.Rental(rentalID)
	if err != nil {
		return domain.Rental{}, err
	}
	// Eine wartende Buchung wurde nie ausgegeben und kann nicht zurückgegeben werden (3.6a, AK-22).
	if s.situation(rental, at).State == domain.RentalWaiting {
		return domain.Rental{}, domain.ErrNotStarted
	}
	if err := rental.Return(at, s.pricing); err != nil {
		return domain.Rental{}, err
	}
	if err := s.repo.UpdateRental(rental); err != nil {
		return domain.Rental{}, err
	}
	return rental, nil
}

// Cancel storniert eine Reservierung vor ihrem Beginn und löscht sie (F-4.5, AK-18).
func (s *Service) Cancel(rentalID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rental, err := s.repo.Rental(rentalID)
	if err != nil {
		return err
	}
	if err := s.situation(rental, s.now()).CheckCancellable(); err != nil {
		return err
	}
	return s.repo.DeleteRental(rentalID)
}
