# CLAUDE.md – Arbeitsregeln für dieses Projekt

Webbasiertes Ski-Verleihsystem (Studienaufgabe Software Engineering, Aufgaben 1a und 1b).
Vorgehen: **Specification-Driven Development (SDD)**.

## Grundregeln

1. **Die Spezifikation gilt.** [docs/SPEZIFIKATION.md](docs/SPEZIFIKATION.md) ist die einzige verbindliche Quelle.
   Bevor sich Verhalten, Routen, Felder oder Regeln ändern: zuerst die Spezifikation anpassen,
   die Änderung nennen, Bestätigung abwarten, dann implementieren.
2. **Nichts erfinden.** Was nicht in der Spezifikation steht, wird nicht gebaut. Fehlt etwas
   oder ist etwas unklar: in Abschnitt 11 („Offene Fragen“) eintragen und nachfragen.
3. **Rückverfolgbarkeit.** Handler, Service-Methoden und Tests nennen im Kommentar die
   Anforderung, die sie umsetzen (z. B. `// F-4.2, AK-8`).
4. **Kein Git.** Keine Repositories anlegen, keine git-Befehle ausführen.
5. **Keine externen Dependencies.** Nur die Go-Standardbibliothek. `go.mod` hat keine `require`-Einträge.
6. **Schrittweise vorgehen** nach Abschnitt 10 der Spezifikation. Nach jedem Schritt laufen die Tests
   grün und die App startet.

## Technik

- Go ≥ 1.22 (`ServeMux` mit Methoden- und Platzhalter-Mustern), `html/template`, `embed`.
- Frontend: serverseitig gerendertes HTML + eine CSS-Datei. **Kein JavaScript**, außer die
  Spezifikation erlaubt es ausdrücklich.
- Daten: In-Memory (Maps), threadsicher mit `sync.RWMutex`. Keine Datenbank, keine Dateien.

## Schichten und Abhängigkeiten

```
handler → service → store
   └────────┴────────┴──→ domain
```

- `internal/domain`: Structs, Enums, `Validate()`, benannte Fehler, `Period`, `Money`, `PricingPolicy`.
  Importiert keine Projektpakete. Zeitabhängige Regeln bekommen `now` als Parameter und lesen nie selbst die Uhr.
- `internal/store`: `MemoryStore`. Kennt kein HTTP und keine Geschäftsregeln außer der ID-Vergabe. Gibt Kopien zurück.
- `internal/service`: Anwendungsfälle (Ausleihen, Rückgabe, Löschregeln, Statistik). Definiert die Repository-Interfaces.
  Schreibende Anwendungsfälle laufen unter einem Mutex. Die Zeit kommt über `now func() time.Time`.
- `internal/handler`: HTTP parsen, Formularwerte prüfen, Service aufrufen, Template rendern bzw. 303-Redirect.
  Keine Geschäftslogik.
- `web/`: Templates und CSS, eingebettet per `//go:embed`.

## Konventionen

- Bezeichner im Code englisch (`Ski`, `Person`, `Rental`, `RentalService`), alle UI-Texte deutsch.
- Enum-Werte deutsch und URL-tauglich (`verfuegbar`, `ausgeliehen`, `wartung`).
- Fehler: benannte `var Err… = errors.New(...)` in `domain/errors.go`. Die Übersetzung in UI-Texte
  geschieht nur in `handler/messages.go`. Ausnahme: Feldmeldungen der Validierung
  (`domain.ValidationErrors`) stehen direkt auf Deutsch im Domain-Code, weil sie fachliche Regeln beschreiben.
- „Ausgeliehen“, „reserviert“, „wartet auf Rückgabe“ und „überfällig“ werden immer aus der Zeit abgeleitet (`Ski.StatusAt`, `domain.Evaluate`).
  Für Anzeigen und Regeln (Rückgabe, Storno, Filter, Kennzahlen) gilt `domain.Evaluate`, weil er die übrigen Buchungen des Ski berücksichtigt (3.6a).
  `Rental.State` allein kennt nur die eigenen Zeitpunkte. Zustände werden nie gespeichert. Gespeichert wird nur der Grundstatus verfügbar/Wartung, und zwar über `Ski.SetManualStatus`.
- Geld nur als `domain.Money` (Cent, `int64`), nie `float64`. Preise berechnet immer der Server über die `PricingPolicy`.
- Nur fiktive Testdaten (NF-11): `Erika Mustermann`, Ausweisnummern mit Präfix `TEST`, E-Mails auf `example.com`.
- Formulare: Post/Redirect/Get. Ungültiges Formular → erneut rendern mit Status 422, Eingaben und Feldfehlern.
- Flash-Meldungen nur über feste Schlüssel (`?msg=rented`), nie freier Text aus der URL.
- Kleine Funktionen und kurze Dateien, `gofmt`. Kommentare auf Deutsch, wo sie das *Warum* erklären.
- Keine vorweggenommene Abstraktion für 1c (siehe Spezifikation Abschnitt 9). Interfaces erst, wenn sie gebraucht werden.

## Tests

- Service-Tests laufen mit fester Uhr und rufen `checkInvariants` auf (I-1 bis I-6).
- `internal/handler/crawl_test.go` folgt allen Links und schickt alle Formulare ab (AK-23). Neue Seiten und Buttons sind damit automatisch abgedeckt.

## Befehle

```sh
go run .                 # Server auf :8080
go run . -demo           # mit Beispieldaten
go run . -addr :9000     # anderer Port
go test ./...            # alle Tests
go test -race ./...      # mit Race-Detector (CGO_ENABLED=1, gcc nötig)
go vet ./...             # statische Prüfung
gofmt -l .               # Formatierung prüfen
```

## Definition of Done (pro Schritt)

- [ ] Verhalten entspricht der Spezifikation. Abweichungen sind dort nachgetragen.
- [ ] Zugehörige Akzeptanzkriterien durch Tests abgedeckt, `go test ./...` grün
- [ ] `go vet ./...` ohne Befunde, Code mit `gofmt` formatiert
- [ ] App startet mit `go run .`, die Funktion ist im Browser ohne JavaScript bedienbar
- [ ] README aktualisiert, falls sich Bedienung oder Start ändern
