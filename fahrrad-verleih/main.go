// Velo-Verleih: Fahrradverleih auf Basis des Verleih-Frameworks (Aufgabe 1c).
//
//	go run .                                 Server auf :8081
//	go run . -demo                           mit Beispieldaten
//	go run . -tarife "citybike=6,ebike=14"   Stundensätze überschreiben
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"fahrradverleih/framework/plugin"
	"fahrradverleih/framework/service"
	"fahrradverleih/framework/store"
	"fahrradverleih/internal/demo"
	"fahrradverleih/internal/handler"
	"fahrradverleih/plugins/fahrrad"
)

func main() {
	addr := flag.String("addr", ":8081", "Adresse des Servers (NF-6)")
	withDemo := flag.Bool("demo", false, "fiktive Beispieldaten laden (NF-7)")
	rates := flag.String("tarife", "", `Stundensätze, z. B. "citybike=6,mountainbike=9.50,ebike=14" (4.7)`)
	flag.Parse()

	srv, err := setup(*rates, *withDemo, time.Now)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Start abgebrochen:", err)
		os.Exit(2)
	}
	url := *addr
	if strings.HasPrefix(url, ":") {
		url = "localhost" + url
	}
	log.Printf("Velo-Verleih läuft auf http://%s", url)
	log.Fatal(http.ListenAndServe(*addr, srv))
}

// setup verdrahtet Registry, Fahrrad-Plugin, Service und Handler (F-8.2).
// Hier – und nur hier – werden Verleiharten registriert.
func setup(rateSpec string, withDemo bool, now func() time.Time) (http.Handler, error) {
	rates, err := fahrrad.ParseRates(rateSpec)
	if err != nil {
		return nil, fmt.Errorf("-tarife: %w", err)
	}
	bikes, err := fahrrad.New(fahrrad.DefaultConfig().WithRates(rates))
	if err != nil {
		return nil, err
	}
	reg := plugin.NewRegistry()
	if err := reg.Register(bikes); err != nil {
		return nil, err
	}
	svc := service.New(store.NewMemoryStore(), reg, now)
	if withDemo {
		if err := demo.Load(svc, now()); err != nil {
			return nil, fmt.Errorf("Demo-Daten: %w", err)
		}
	}
	return handler.New(svc, bikes)
}
