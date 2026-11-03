package demo

import (
	"testing"
	"time"

	"fahrradverleih/framework/plugin"
	"fahrradverleih/framework/service"
	"fahrradverleih/framework/store"
	"fahrradverleih/plugins/fahrrad"
)

// AK-1: Kennzahlen der Demo-Daten, zu verschiedenen Tageszeiten geladen.
func TestDemoNumbers(t *testing.T) {
	for _, now := range []time.Time{
		time.Date(2026, 11, 2, 14, 5, 0, 0, time.Local),
		time.Date(2026, 11, 2, 0, 10, 0, 0, time.Local),
		time.Date(2026, 11, 2, 23, 50, 0, 0, time.Local),
	} {
		t.Run(now.Format("15:04"), func(t *testing.T) {
			bikes, _ := fahrrad.New(fahrrad.DefaultConfig())
			reg := plugin.NewRegistry()
			_ = reg.Register(bikes)
			svc := service.New(store.NewMemoryStore(), reg, func() time.Time { return now })
			if err := Load(svc, now); err != nil {
				t.Fatal(err)
			}
			st, _ := svc.Stats(fahrrad.Key)
			if st.Available != 6 || st.Rented != 2 || st.Overdue != 1 || st.Reserved != 2 || st.Total != 9 || st.Persons != 4 || len(st.Conflicts) != 1 || st.Maintenance != 1 {
				t.Errorf("Kennzahlen: verfügbar %d, ausgeliehen %d (überfällig %d), reserviert %d, gesamt %d, Personen %d, Konflikte %d, Wartung %d",
					st.Available, st.Rented, st.Overdue, st.Reserved, st.Total, st.Persons, len(st.Conflicts), st.Maintenance)
			}
			closed := svc.Bookings(service.FilterClosed)
			if len(closed) != 1 || closed[0].Price.Amount != 40_00 {
				t.Errorf("abgeschlossene Ausleihe (4,5 Std Mountainbike → 5 × 8 €): %+v", closed)
			}
		})
	}
}
