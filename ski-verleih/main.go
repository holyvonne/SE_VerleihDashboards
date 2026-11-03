// Ski-Verleih – Prototyp für Aufgaben 1a und 1b. Spezifikation: docs/SPEZIFIKATION.md
package main

import (
	"flag"
	"log"
	"net/http"
	"strings"
	"time"

	"skiverleih/internal/domain"
	"skiverleih/internal/handler"
	"skiverleih/internal/service"
	"skiverleih/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "Adresse, auf der der Server lauscht (NF-6)")
	demo := flag.Bool("demo", false, "fiktive Beispieldaten laden (NF-7)")
	flag.Parse()

	// Preisregel aus 1b (Spezifikation 3.7); hier gegen eine andere Regel austauschbar.
	pricing := domain.StandardPricing()
	svc := service.New(store.NewMemoryStore(), time.Now, pricing)
	if *demo {
		if err := service.LoadDemo(svc); err != nil {
			log.Fatal(err)
		}
		log.Print("Demo-Daten geladen (fiktiv)")
	}

	srv, err := handler.New(svc)
	if err != nil {
		log.Fatal(err)
	}

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           srv,
		ReadHeaderTimeout: 5 * time.Second,
	}
	host := *addr
	if strings.HasPrefix(host, ":") {
		host = "localhost" + host
	}
	log.Printf("Ski-Verleih läuft auf http://%s", host)
	log.Fatal(httpServer.ListenAndServe())
}
