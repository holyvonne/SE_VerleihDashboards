package service

import (
	"time"

	"skiverleih/internal/domain"
)

// SkiInput sind die Formularwerte zum Anlegen und Bearbeiten eines Ski.
type SkiInput struct {
	Brand     string
	Model     string
	LengthCM  int
	Type      domain.SkiType
	Condition domain.Condition
	Status    domain.SkiStatus // Grundstatus, nur beim Bearbeiten ausgewertet (F-2.4)
	Note      string
}

// SkiView ist ein Ski mit dem zum Abfragezeitpunkt abgeleiteten Status (3.1).
type SkiView struct {
	Ski             domain.Ski
	Status          domain.SkiStatus // angezeigter Status
	NextReservation *domain.Rental   // nächste Reservierung, falls vorhanden
}

// Bookable meldet, ob der Ski gebucht werden kann (nicht in Wartung, AK-9).
func (v SkiView) Bookable() bool { return !v.Ski.InMaintenance() }

// InUse meldet eine aktive Ausleihe oder Reservierung; dann ist der Status gesperrt (F-2.4).
func (v SkiView) InUse() bool { return v.Status == domain.SkiRented || v.NextReservation != nil }

func skiView(ski domain.Ski, rentals []domain.Rental, now time.Time) SkiView {
	v := SkiView{Ski: ski, Status: ski.StatusAt(rentals, now)}
	for _, r := range rentals {
		if r.SkiID != ski.ID || !r.IsReserved(now) {
			continue
		}
		if v.NextReservation == nil || r.Start.Before(v.NextReservation.Start) {
			v.NextReservation = &r // seit Go 1.22 hat jede Schleifeniteration ihr eigenes r
		}
	}
	return v
}

// SkiDetail ist alles, was die Detailseite eines Ski braucht (F-2.3).
type SkiDetail struct {
	View         SkiView
	Current      *RentalView  // aktive Ausleihe oder nil
	Reservations []RentalView // nächste zuerst
	History      []RentalView // abgeschlossen, neueste zuerst
}

// HasHistory meldet, ob der Ski Buchungen hat (Löschregel F-2.5).
func (d SkiDetail) HasHistory() bool {
	return d.Current != nil || len(d.Reservations) > 0 || len(d.History) > 0
}

// Skis liefert alle Ski, optional gefiltert nach angezeigtem Status ("" = alle) (F-2.1).
func (s *Service) Skis(status domain.SkiStatus) []SkiView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()
	rentals := s.repo.ListRentals()
	var out []SkiView
	for _, sk := range s.repo.ListSkis() {
		v := skiView(sk, rentals, now)
		if status == "" || v.Status == status {
			out = append(out, v)
		}
	}
	return out
}

// BookableSkis liefert alle Ski, die nicht in Wartung sind (F-4.1).
func (s *Service) BookableSkis() []SkiView {
	var out []SkiView
	for _, v := range s.Skis("") {
		if v.Bookable() {
			out = append(out, v)
		}
	}
	return out
}

// CreateSki legt einen neuen, verfügbaren Ski an (F-2.2).
func (s *Service) CreateSki(in SkiInput) (domain.Ski, error) {
	ski := domain.NewSki(in.Brand, in.Model, in.LengthCM, in.Type, in.Condition, in.Note)
	if err := ski.Validate(); err != nil {
		return domain.Ski{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.repo.CreateSki(ski), nil
}

// SkiDetail liefert Ski, aktuelle Ausleihe, Reservierungen und Historie (F-2.3).
func (s *Service) SkiDetail(id int) (SkiDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ski, err := s.repo.Ski(id)
	if err != nil {
		return SkiDetail{}, err
	}
	rentals := s.rentalsOfSki(id)
	d := SkiDetail{View: skiView(ski, rentals, s.now())}
	active, reserved, closed := splitByState(s.views(rentals))
	if len(active) > 0 {
		d.Current = &active[0] // I-2: höchstens eine
	}
	d.Reservations, d.History = reserved, closed
	return d, nil
}

// UpdateSki ändert Stammdaten und ggf. den Grundstatus verfügbar ↔ wartung (F-2.4).
// Auf Wartung darf nur, wer weder aktiv ausgeliehen noch reserviert ist (AK-4, AK-21, I-6).
func (s *Service) UpdateSki(id int, in SkiInput) (domain.Ski, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ski, err := s.repo.Ski(id)
	if err != nil {
		return domain.Ski{}, err
	}
	// Belegung mit dem unveränderten Ski prüfen – mit Grundstatus Wartung
	// würde StatusAt sonst „Wartung“ statt „ausgeliehen“ liefern.
	blocked := in.Status == domain.SkiMaintenance && !ski.InMaintenance() &&
		skiView(ski, s.rentalsOfSki(id), s.now()).InUse()

	ski.Brand, ski.Model, ski.LengthCM = in.Brand, in.Model, in.LengthCM
	ski.Type, ski.Condition, ski.Note = in.Type, in.Condition, in.Note
	ski.Normalize()

	errs := domain.ValidationErrors{}
	if err := mergeValidation(errs, ski.SetManualStatus(in.Status)); err != nil {
		return domain.Ski{}, err
	}
	if err := mergeValidation(errs, ski.Validate()); err != nil {
		return domain.Ski{}, err
	}
	if err := errOrNil(errs); err != nil {
		return domain.Ski{}, err
	}
	if blocked {
		return domain.Ski{}, domain.ErrSkiInUse
	}
	if err := s.repo.UpdateSki(ski); err != nil {
		return domain.Ski{}, err
	}
	return ski, nil
}

// DeleteSki löscht nur Ski ohne Buchungen (F-2.5).
func (s *Service) DeleteSki(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.repo.Ski(id); err != nil {
		return err
	}
	if len(s.rentalsOfSki(id)) > 0 {
		return domain.ErrHasHistory
	}
	return s.repo.DeleteSki(id)
}
