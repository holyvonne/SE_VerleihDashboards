package service

import (
	"sort"
	"strings"

	"fahrradverleih/framework/domain"
)

// PersonInput sind die Formularwerte einer Person. Image == nil bedeutet „kein neues Bild“.
type PersonInput struct {
	FirstName, LastName, Phone, Email string
	IDType                            domain.IDType
	IDNumber                          string
	Image                             []byte
	RemoveImage                       bool
}

// PersonRow ist eine Zeile der Personenliste (F-3).
type PersonRow struct {
	Person       domain.Person
	Active       int
	Reservations int
}

// Persons liefert alle Personen, gefiltert nach Name, Ausweisnummer oder E-Mail.
func (s *Service) Persons(q string) []PersonRow {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := s.load(s.now())
	q = strings.ToLower(strings.TrimSpace(q))
	active, reserved := map[int]int{}, map[int]int{}
	for _, b := range snap.bookings {
		switch sit := snap.situation(b); {
		case sit.State == domain.StateActive:
			active[b.PersonID]++
		case sit.NotHandedOut():
			reserved[b.PersonID]++
		}
	}
	var out []PersonRow
	for _, p := range s.repo.ListPersons() {
		hay := strings.ToLower(p.FullName() + " " + p.LastName + " " + p.FirstName + " " + p.IDNumber + " " + p.Email)
		if q == "" || strings.Contains(hay, q) {
			out = append(out, PersonRow{Person: p, Active: active[p.ID], Reservations: reserved[p.ID]})
		}
	}
	return out
}

// AllPersons liefert alle Personen nach Nachname sortiert (Auswahl im Buchungsformular).
func (s *Service) AllPersons() []domain.Person {
	persons := s.repo.ListPersons()
	sortPersons(persons)
	return persons
}

func (s *Service) Person(id int) (domain.Person, error) { return s.repo.Person(id) }

// PersonDetail ist die Detailansicht einer Person.
type PersonDetail struct {
	Person   domain.Person
	Active   []BookingView
	Upcoming []BookingView
	History  []BookingView
}

func (d PersonDetail) HasHistory() bool { return len(d.Active)+len(d.Upcoming)+len(d.History) > 0 }

func (s *Service) PersonDetail(id int) (PersonDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := s.load(s.now())
	p, ok := snap.persons[id]
	if !ok {
		return PersonDetail{}, domain.ErrNotFound
	}
	active, upcoming, closed := split(snap.views(func(b domain.Booking, _ domain.Situation) bool { return b.PersonID == id }))
	return PersonDetail{Person: p, Active: active, Upcoming: upcoming, History: closed}, nil
}

// CreatePerson registriert eine Person (F-3, AK-5, AK-26).
func (s *Service) CreatePerson(in PersonInput) (domain.Person, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := domain.Person{CreatedAt: s.now()}
	if err := s.applyPersonInput(&p, in); err != nil {
		return domain.Person{}, err
	}
	return s.repo.CreatePerson(p), nil
}

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
	return p, s.repo.UpdatePerson(p)
}

// applyPersonInput übernimmt die Eingaben, prüft sie und sammelt alle Feldfehler.
func (s *Service) applyPersonInput(p *domain.Person, in PersonInput) error {
	p.FirstName, p.LastName, p.Phone, p.Email = in.FirstName, in.LastName, in.Phone, in.Email
	p.IDType, p.IDNumber = in.IDType, in.IDNumber
	p.Normalize()
	errs := p.Validate()
	if s.idTaken(*p) {
		errs.Add("id_number", "Diese Ausweisnummer ist bereits registriert")
	}
	switch {
	case in.Image != nil:
		img, err := domain.NewIDImage(in.Image)
		if v, ok := domain.AsValidation(err); ok {
			errs.Merge(v)
		} else {
			p.IDImage = img
		}
	case in.RemoveImage:
		p.IDImage = nil
	}
	return errs.ErrOrNil()
}

// idTaken prüft I-5 gegen alle anderen Personen.
func (s *Service) idTaken(p domain.Person) bool {
	for _, other := range s.repo.ListPersons() {
		if other.ID != p.ID && other.IDKey() == p.IDKey() {
			return true
		}
	}
	return false
}

// DeletePerson löscht eine Person ohne Buchungen.
func (s *Service) DeletePerson(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.repo.Person(id); err != nil {
		return err
	}
	for _, b := range s.repo.ListBookings() {
		if b.PersonID == id {
			return domain.ErrHasHistory
		}
	}
	return s.repo.DeletePerson(id)
}

func sortPersons(ps []domain.Person) {
	sort.SliceStable(ps, func(i, j int) bool {
		a, b := ps[i], ps[j]
		if !strings.EqualFold(a.LastName, b.LastName) {
			return strings.ToLower(a.LastName) < strings.ToLower(b.LastName)
		}
		return strings.ToLower(a.FirstName) < strings.ToLower(b.FirstName)
	})
}
