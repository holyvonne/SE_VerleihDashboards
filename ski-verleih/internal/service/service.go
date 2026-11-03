package service

import (
	"errors"
	"sort"
	"sync"
	"time"

	"skiverleih/internal/domain"
)

// Service setzt die Anwendungsfälle um.
//
// Nebenläufigkeit: Schreibende Anwendungsfälle (z. B. Buchen) lesen alle Buchungen
// eines Ski und legen danach eine neue an. Sie laufen deshalb unter der Schreib-Sperre.
// Abfragen über mehrere Tabellen nehmen die Lese-Sperre, damit sie nie einen halb
// fertigen Zustand sehen. So kann ein Ski auch bei parallelen Anfragen nicht doppelt
// gebucht werden (I-2, AK-11, AK-20).
// Methoden mit kleinem Anfangsbuchstaben setzen voraus, dass die Sperre bereits gehalten wird.
type Service struct {
	mu      sync.RWMutex
	repo    Repository
	now     func() time.Time
	pricing domain.PricingPolicy
}

// New erstellt den Service. now liefert die aktuelle Zeit (Tests übergeben eine feste Uhr),
// pricing die Preisregel (3.7). nil wählt jeweils den Standard.
func New(repo Repository, now func() time.Time, pricing domain.PricingPolicy) *Service {
	if now == nil {
		now = time.Now
	}
	if pricing == nil {
		pricing = domain.StandardPricing()
	}
	return &Service{repo: repo, now: now, pricing: pricing}
}

// Now liefert die Uhrzeit des Service, z. B. für Vorbelegungen im Formular.
func (s *Service) Now() time.Time { return s.now() }

// RentalView verbindet eine Buchung mit Ski, Person und den zum Zeitpunkt der
// Abfrage abgeleiteten Werten für die Anzeige.
type RentalView struct {
	Rental   domain.Rental
	Ski      domain.Ski
	Person   domain.Person
	State    domain.RentalState
	Overdue  bool
	Conflict bool          // von einer überfälligen Ausleihe blockiert (3.6a)
	Duration time.Duration // geplant bzw. tatsächlich (F-5.1)
	Price    domain.Money  // gebuchter Preis bzw. Endpreis
}

// views reichert Buchungen an und sortiert den neuesten Beginn zuerst (F-5.3).
func (s *Service) views(rentals []domain.Rental) []RentalView {
	skis := map[int]domain.Ski{}
	for _, sk := range s.repo.ListSkis() {
		skis[sk.ID] = sk
	}
	persons := map[int]domain.Person{}
	for _, p := range s.repo.ListPersons() {
		persons[p.ID] = p
	}
	now := s.now()
	bySki := map[int][]domain.Rental{}
	for _, r := range s.repo.ListRentals() {
		bySki[r.SkiID] = append(bySki[r.SkiID], r)
	}
	out := make([]RentalView, len(rentals))
	for i, r := range rentals {
		sit := domain.Evaluate(r, bySki[r.SkiID], now)
		out[i] = RentalView{
			Rental: r, Ski: skis[r.SkiID], Person: persons[r.PersonID],
			State: sit.State, Overdue: sit.Overdue, Conflict: sit.Conflict,
			Duration: r.Duration(), Price: r.ChargedPrice(),
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Rental, out[j].Rental
		if !a.Start.Equal(b.Start) {
			return a.Start.After(b.Start)
		}
		return a.ID > b.ID
	})
	return out
}

// soonestFirst dreht die Sortierung um: nächster Beginn zuerst (für Reservierungen).
func soonestFirst(v []RentalView) []RentalView {
	for i, j := 0, len(v)-1; i < j; i, j = i+1, j-1 {
		v[i], v[j] = v[j], v[i]
	}
	return v
}

// rentalsWhere liefert alle Buchungen, für die keep true ist.
func (s *Service) rentalsWhere(keep func(domain.Rental) bool) []domain.Rental {
	var out []domain.Rental
	for _, r := range s.repo.ListRentals() {
		if keep(r) {
			out = append(out, r)
		}
	}
	return out
}

func (s *Service) rentalsOfSki(skiID int) []domain.Rental {
	return s.rentalsWhere(func(r domain.Rental) bool { return r.SkiID == skiID })
}

// situation bestimmt den Zustand einer Buchung im Zusammenhang mit ihrem Ski (3.6a).
func (s *Service) situation(r domain.Rental, now time.Time) domain.Situation {
	return domain.Evaluate(r, s.rentalsOfSki(r.SkiID), now)
}

// splitByState teilt angereicherte Buchungen in aktiv, noch nicht ausgegeben (reserviert oder
// wartend, nächste zuerst) und abgeschlossen.
func splitByState(views []RentalView) (active, reserved, closed []RentalView) {
	for _, v := range views {
		switch v.State {
		case domain.RentalActive:
			active = append(active, v)
		case domain.RentalReserved, domain.RentalWaiting:
			reserved = append(reserved, v)
		default:
			closed = append(closed, v)
		}
	}
	return active, soonestFirst(reserved), closed
}

// mergeValidation übernimmt Feldfehler aus err in dst.
// Andere Fehler werden unverändert zurückgegeben.
func mergeValidation(dst domain.ValidationErrors, err error) error {
	if err == nil {
		return nil
	}
	var verr domain.ValidationErrors
	if !errors.As(err, &verr) {
		return err
	}
	for field, msg := range verr {
		if _, exists := dst[field]; !exists {
			dst[field] = msg
		}
	}
	return nil
}

// errOrNil vermeidet die typed-nil-Falle bei leeren ValidationErrors.
func errOrNil(errs domain.ValidationErrors) error {
	if len(errs) == 0 {
		return nil
	}
	return errs
}
