# CLAUDE.md – Arbeitsregeln für dieses Projekt

Fahrradverleih auf Basis eines wiederverwendbaren Verleih-Frameworks mit Plugin-Architektur
(Studienaufgabe Software Engineering, Aufgabe 1c). Vorgehen: **Specification-Driven Development (SDD)**.

## Grundregeln

1. **Die Spezifikation gilt.** [docs/SPEZIFIKATION.md](docs/SPEZIFIKATION.md) ist die einzige verbindliche Quelle.
   Bevor sich Verhalten, Routen, Felder, Regeln oder Paketgrenzen ändern: zuerst die Spezifikation anpassen,
   die Änderung nennen, Bestätigung abwarten, dann implementieren.
2. **Nichts erfinden.** Fehlt etwas oder ist etwas unklar: in Abschnitt 11 eintragen und nachfragen.
3. **Rückverfolgbarkeit.** Handler, Service-Methoden und Tests nennen die Anforderung im Kommentar (`// F-5.3, AK-9`).
4. **Kein Git.** Keine Repositories anlegen, keine git-Befehle ausführen.
5. **Keine externen Dependencies.** Nur die Go-Standardbibliothek. `go.mod` hat keine `require`-Einträge.
6. **Eigenständig.** Nichts aus `../ski-verleih` importieren, kopieren per Verweis oder dort ändern.
   Das Projekt muss allein als ZIP lauffähig sein.
7. **Schrittweise** nach Abschnitt 10 der Spezifikation. Nach jedem Schritt laufen die Tests grün.

## Architekturregeln (Spezifikation 3.1)

```
main.go ──► internal/handler ──► framework/service ──► framework/plugin ──► framework/domain
   │               │                     └──────────► framework/store ─────────┘
   └──► plugins/fahrrad ──► framework/plugin, framework/domain
```

- `framework/…` kennt **keine** Verleihart: keine Importe von `plugins/`, `internal/`, `web/`, keine Wörter wie
  `fahrrad`, `bike`, `ski` im Nicht-Test-Code. Der Architekturtest (`framework/architecture_test.go`) prüft das.
- Fachliche Unterschiede zwischen Verleiharten gehören **ins Plugin** (Kategorien, Merkmale, Validierung über
  mehrere Felder, Preisstrategie, Grenzen), nie als `if kind == "…"` ins Framework.
- Plugins importieren nur `framework/domain` und `framework/plugin`.
- `framework/domain`: reine Typen und Regeln, zeitabhängige Regeln bekommen `now` als Parameter.
- `framework/store`: In-Memory, `sync.RWMutex`, gibt Kopien zurück, keine Geschäftsregeln außer ID-Vergabe.
- `framework/service`: Anwendungsfälle, definiert die Repository-Interfaces, schreibende Fälle unter einem Mutex,
  Zeit über `now func() time.Time`, Plugins über die Registry.
- `internal/handler`: HTTP parsen, Service aufrufen, rendern. Keine Geschäftslogik. Darf das Fahrrad-Plugin für
  Texte und Tarife kennen.
- Plugins werden nur in `main.go` (und in Tests) registriert.

## Neue Verleihart hinzufügen

1. Neues Paket unter `plugins/<art>/`, das `plugin.Plugin` implementiert (Kategorien, Merkmale, `Validate`, `DisplayName`, `Pricing`, `Limits`). Nur `framework/domain` und `framework/plugin` importieren.
2. Eine eigene `PricingStrategy` schreiben, falls keine passende existiert.
3. In `main.go` (oder im Test) `reg.Register(...)` aufrufen. **Kein** Framework-Code wird geändert.
4. Tests: Registrierung, Preisbeispiele und ein Service-Test mit der neuen Art (Vorlage: `framework/service/plugin_test.go`).

## Tests

- Domain- und Service-Tests laufen mit fester Uhr (`clock`). `checkInvariants` nach jeder Änderung aufrufen.
- Handler-Tests laufen über `httptest` gegen den echten Service. `crawl_test.go` folgt allen Links und schickt alle Formulare ab (AK-27) – neue Seiten und Buttons sind damit automatisch abgedeckt.
- Gefundene Fehler bekommen einen Regressionstest (`internal/handler/regression_test.go`).

## Konventionen

- Bezeichner englisch (`Item`, `Booking`, `PricingStrategy`), UI-Texte deutsch, Enum-/Schlüsselwerte deutsch und
  URL-tauglich (`verfuegbar`, `ebike`, `rahmengroesse`).
- Fehler als benannte `var Err…` in `framework/domain/errors.go`, neutral formuliert. Die UI-Texte mit „Fahrrad“
  entstehen nur in `internal/handler/messages.go`. Feldmeldungen (`domain.ValidationErrors`) stehen auf Deutsch im Code.
- „ausgeliehen“, „reserviert“, „überfällig“ werden immer aus der Zeit abgeleitet, nie gespeichert.
- Geld nur als `domain.Money` (Cent, `int64`). Preise berechnet immer der Server über die `PricingStrategy` des Plugins.
- Nur fiktive Testdaten: erfundene Namen, Ausweisnummern mit Präfix `TEST`, E-Mails auf `example.com`.
- Post/Redirect/Get, ungültige Formulare mit 422, Flash-Meldungen nur über feste Schlüssel.
- Kleine Funktionen, `gofmt`, Kommentare auf Deutsch, wo sie das *Warum* erklären.

## Befehle

```sh
go run .                                   # Server auf :8081
go run . -demo                             # mit Beispieldaten
go run . -tarife "citybike=6,ebike=14"     # Stundensätze überschreiben
go run . -addr :9001                       # anderer Port
go test ./...                              # alle Tests
go test -race ./...                        # mit Race-Detector (CGO_ENABLED=1, gcc nötig)
go vet ./...
gofmt -l .
```

## Definition of Done (pro Schritt)

- [ ] Verhalten entspricht der Spezifikation, Abweichungen sind dort nachgetragen
- [ ] Zugehörige Akzeptanzkriterien durch Tests abgedeckt, `go test ./...` und `go test -race ./...` grün
- [ ] Architekturtest grün, `go vet` ohne Befunde, `gofmt -l .` leer
- [ ] App startet mit `go run .`, Funktion im Browser ohne JavaScript bedienbar
- [ ] README aktualisiert, falls sich Bedienung oder Start ändern
