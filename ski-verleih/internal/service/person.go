package service

import (
	"strings"

	"skiverleih/internal/domain"
)

const msgDuplicateID = "Diese Ausweisnummer ist bereits registriert"

// PersonInput sind die Formularwerte zum Anlegen und Bearbeiten einer Person.
type PersonInput struct {
	FirstName   string
	LastName    string
	Phone       string
	Email       string
	IDType      domain.IDType
	IDNumber    string
	Image       []byte // neu hochgeladenes Ausweisbild; nil = keines gewählt
	RemoveImage bool   // „Ausweisbild entfernen“ (F-3.4); ein neues Bild hat Vorrang
}

// PersonRow ist eine Zeile der Personenliste (F-3.1).
type PersonRow struct {
	Person        domain.Person
	ActiveRentals int
	Reservations  int
}

// PersonDetail ist alles, was die Detailseite einer Person braucht (F-3.3).
type PersonDetail struct {
	Person       domain.Person
	Active       []RentalView
	Reservations []RentalView // nächste zuerst
	History      []RentalView
}

// HasHistory meldet, ob die Person Buchungen hat (Löschregel F-3.5).
func (d PersonDetail) HasHistory() bool {
	return len(d.Active) > 0 || len(d.Reservations) > 0 || len(d.History) > 0
}

// Persons liefert die Personenliste, optional gefiltert nach Suchbegriff (F-3.1).
// Gesucht wird in Vorname, Nachname und Ausweisnummer, ohne Groß-/Kleinschreibung.
func (s *Service) Persons(query string) []PersonRow {
	s.mu.RLock()
	defer s.mu.RUnlock()
	q := strings.ToLower(strings.TrimSpace(query))
	now := s.now()
	active, reserved := map[int]int{}, map[int]int{}
	for _, r := range s.repo.ListRentals() {
		switch s.situation(r, now).State {
		case domain.RentalActive:
			active[r.PersonID]++
		case domain.RentalReserved, domain.RentalWaiting:
			reserved[r.PersonID]++
		}
	}
	var out []PersonRow
	for _, p := range s.repo.ListPersons() {
		if q != "" && !matches(p, q) {
			continue
		}
		out = append(out, PersonRow{Person: p, ActiveRentals: active[p.ID], Reservations: reserved[p.ID]})
	}
	return out
}

func matches(p domain.Person, q string) bool {
	for _, field := range []string{p.FirstName, p.LastName, p.IDNumber} {
		if strings.Contains(strings.ToLower(field), q) {
			return true
		}
	}
	return false
}

// AllPersons liefert alle Personen, z. B. für die Auswahlliste beim Ausleihen.
func (s *Service) AllPersons() []domain.Person { return s.repo.ListPersons() }

// Person liefert eine einzelne Person (z. B. für das Ausweisbild).
func (s *Service) Person(id int) (domain.Person, error) { return s.repo.Person(id) }

// CreatePerson registriert eine Person (F-3.2, AK-5, AK-6, AK-7).
func (s *Service) CreatePerson(in PersonInput) (domain.Person, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := domain.Person{CreatedAt: s.now()}
	if err := s.applyPersonInput(&p, in); err != nil {
		return domain.Person{}, err
	}
	return s.repo.CreatePerson(p), nil
}

// UpdatePerson ändert eine Person; ohne neues Bild bleibt das alte erhalten (F-3.4).
func (s *Service) UpdatePerson(id int, in PersonInput) (domain.Person, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.repo.Person(id)
	if err != nil {
		return domain.Person{}, err
	}
	if err := s.applyPersonInput(&p, in); err != nil {
		return domain.Person{}, err
	}
	if err := s.repo.UpdatePerson(p); err != nil {
		return domain.Person{}, err
	}
	return p, nil
}

// applyPersonInput überträgt und prüft die Eingaben inkl. Bild und Eindeutigkeit (I-5).
func (s *Service) applyPersonInput(p *domain.Person, in PersonInput) error {
	p.FirstName, p.LastName = in.FirstName, in.LastName
	p.Phone, p.Email = in.Phone, in.Email
	p.IDType, p.IDNumber = in.IDType, in.IDNumber
	p.Normalize()

	errs := domain.ValidationErrors{}
	if err := mergeValidation(errs, p.Validate()); err != nil {
		return err
	}
	switch {
	case in.Image != nil:
		img, err := domain.NewIDImage(in.Image)
		if err := mergeValidation(errs, err); err != nil {
			return err
		}
		if img != nil {
			p.IDImage = img
		}
	case in.RemoveImage:
		p.IDImage = nil
	}
	if _, invalid := errs["id_number"]; !invalid && s.idTaken(*p) {
		errs["id_number"] = msgDuplicateID
	}
	return errOrNil(errs)
}

// idTaken prüft, ob eine andere Person dieselbe Ausweis-Kombination hat.
func (s *Service) idTaken(p domain.Person) bool {
	for _, other := range s.repo.ListPersons() {
		if other.ID != p.ID && other.IDKey() == p.IDKey() {
			return true
		}
	}
	return false
}

// PersonDetail liefert Person, aktive Ausleihen und Historie (F-3.3).
func (s *Service) PersonDetail(id int) (PersonDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, err := s.repo.Person(id)
	if err != nil {
		return PersonDetail{}, err
	}
	d := PersonDetail{Person: p}
	d.Active, d.Reservations, d.History = splitByState(
		s.views(s.rentalsWhere(func(r domain.Rental) bool { return r.PersonID == id })))
	return d, nil
}

// DeletePerson löscht nur Personen ohne Ausleih-Historie (F-3.5).
func (s *Service) DeletePerson(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.repo.Person(id); err != nil {
		return err
	}
	if len(s.rentalsWhere(func(r domain.Rental) bool { return r.PersonID == id })) > 0 {
		return domain.ErrHasHistory
	}
	return s.repo.DeletePerson(id)
}
