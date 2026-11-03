package handler

import (
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fahrradverleih/framework/domain"
	"fahrradverleih/framework/service"
)

// offerGroup sind die freien Fahrräder eines Typs (F-4).
type offerGroup struct {
	Category domain.Category
	Rule     string
	Offers   []service.Offer
}

type availabilityPage struct {
	StartDate, StartTime, EndDate, EndTime string
	Category                               string
	Categories                             []domain.Category
	Searched                               bool
	Groups                                 []offerGroup
	Count                                  int
	Period                                 domain.Period
	// BookQuery ist „ende=…&start=…“ für den Buchen-Link. Selbst mit url.Values.Encode gebaut,
	// daher als template.URL markiert – sonst würde html/template das & noch einmal maskieren.
	BookQuery template.URL
	Errors    domain.ValidationErrors
}

// F-4, AK-6: Suche per GET, damit das Ergebnis verlinkt und neu geladen werden kann.
func (s *Server) handleAvailability(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	pg := availabilityPage{
		StartDate: strings.TrimSpace(q.Get("start_date")), StartTime: strings.TrimSpace(q.Get("start_time")),
		EndDate: strings.TrimSpace(q.Get("end_date")), EndTime: strings.TrimSpace(q.Get("end_time")),
		Category: q.Get("typ"), Categories: s.bikes.Categories(),
	}
	if _, ok := s.category(pg.Category); !ok {
		pg.Category = ""
	}
	status := http.StatusOK
	var errMsg string
	if pg.StartDate == "" && pg.EndDate == "" {
		start := nextFullHour(s.svc.Now())
		end := start.Add(2 * time.Hour)
		pg.StartDate, pg.StartTime = start.Format(dateLayout), start.Format(timeLayout)
		pg.EndDate, pg.EndTime = end.Format(dateLayout), end.Format(timeLayout)
	} else {
		pg.Searched = true
		pg.Period = domain.Period{Start: parseLocal(pg.StartDate, pg.StartTime), End: parseLocal(pg.EndDate, pg.EndTime)}
		offers, err := s.svc.Available(kind, pg.Category, pg.Period)
		if err != nil {
			fields, msg, isFormError := formProblem(err)
			if !isFormError {
				s.fail(w, r, err)
				return
			}
			pg.Errors, errMsg, pg.Searched, status = fields, msg, false, http.StatusUnprocessableEntity
		}
		pg.Groups = s.groupOffers(offers)
		pg.Count = len(offers)
		v := url.Values{}
		v.Set("start", pg.Period.Start.In(time.Local).Format(queryLayout))
		v.Set("ende", pg.Period.End.In(time.Local).Format(queryLayout))
		pg.BookQuery = template.URL(v.Encode())
	}
	s.render(w, r, status, "availability.html", page{
		Title:    "Verfügbarkeit prüfen",
		Subtitle: "Freie Fahrräder für einen Zeitraum finden",
		Active:   "verfuegbarkeit",
		Flash:    errorFlash(errMsg),
		Content:  pg,
	})
}

func (s *Server) groupOffers(offers []service.Offer) []offerGroup {
	var groups []offerGroup
	for _, c := range s.bikes.Categories() {
		g := offerGroup{Category: c, Rule: s.bikes.Pricing().Describe(c.Key)}
		for _, o := range offers {
			if o.View.Item.Category == c.Key {
				g.Offers = append(g.Offers, o)
			}
		}
		if len(g.Offers) > 0 {
			groups = append(groups, g)
		}
	}
	return groups
}
