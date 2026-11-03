# Velo-Verleih: Fahrradverleih auf einem Verleih-Framework (Aufgabe 1c)

Studienaufgabe im Fach Software Engineering. Das Projekt ist ein Fahrradverleih mit **stundenweiser Abrechnung**.
Er baut auf einem **wiederverwendbaren Verleih-Framework** mit **Plugin-Architektur** auf.

- Go, nur Standardbibliothek
- Daten nur im Arbeitsspeicher
- HTML/CSS ohne JavaScript
- eigenständiges Go-Modul, unabhängig vom Projekt `ski-verleih` und allein als ZIP lauffähig

Verbindlich ist [docs/SPEZIFIKATION.md](docs/SPEZIFIKATION.md) (v1.1). Die Arbeitsregeln stehen in [CLAUDE.md](CLAUDE.md).

## Stand

| Bereich | Status |
|---------|--------|
| Spezifikation v1.0 mit Festlegungen A-1 bis A-9 | ✅ |
| Framework (`framework/domain`, `plugin`, `store`, `service`) | ✅ |
| Fahrrad-Plugin (Typen, Merkmale, Stundentarif, Konfiguration) | ✅ |
| Beispiel-Plugin Ski (Tagestarif, ohne Oberfläche) | ✅ |
| Weboberfläche (Dashboard, Fahrräder, Personen, Buchen, Verfügbarkeit, Tarife, Umbuchen) | ✅ |
| `go test ./...` und `go test -race ./...` | ✅ grün (88 Testfunktionen, 124 Fälle inkl. Untertests) |
| Abschlussprüfung (siehe unten) | ✅ 3 Fehler behoben, 4 offene Punkte dokumentiert |
| `go vet ./...`, `gofmt -l .` | ✅ ohne Befunde |

## Starten

Voraussetzung ist Go ab Version 1.22.

```sh
cd fahrrad-verleih
go run . -demo          # mit fiktiven Beispieldaten → http://localhost:8081
go run .                # leer starten
go run . -tarife "citybike=6,mountainbike=9.50,ebike=14"   # Stundensätze überschreiben
go run . -addr :9001    # anderer Port
```

Der Fahrradverleih läuft auf Port **8081**, damit der Ski-Verleih parallel auf 8080 laufen kann.
Nach einem Neustart sind alle Daten weg, weil sie nur im Arbeitsspeicher liegen.

## Testen

```sh
go test ./...                # alle Tests
go test -race ./...          # mit Race-Detector (CGO_ENABLED=1, gcc nötig)
go vet ./...
gofmt -l .                   # keine Ausgabe = korrekt formatiert
```

## Im Browser testen

Starte mit `go run . -demo` und öffne http://localhost:8081. Die Demo-Daten werden **relativ zur Startzeit** angelegt.
Die Uhrzeiten in den Listen hängen also davon ab, wann du startest.

**Demo-Daten:**
- 9 Fahrräder, je 3 pro Typ
- Trek Marlin ist in Wartung
- 4 Personen
- 5 Buchungen:
  - *Gazelle Esprit* ist an Erika ausgeliehen und **überfällig**.
  - Lena hat *dasselbe Rad* ab in 2 Stunden reserviert. Diese Reservierung steht im **Konflikt**.
  - Max hat ein E-Bike aktiv ausgeliehen.
  - Erika hat für morgen 09:00 ein E-Bike reserviert.
  - Jonas hat gestern ein Mountainbike eine halbe Stunde zu spät zurückgegeben.

| # | Was prüfen | So geht's | Erwartung |
|---|------------|-----------|-----------|
| 1 | Dashboard | Startseite | Kacheln: 6 verfügbar · 2 ausgeliehen („davon 1 überfällig“) · 2 reserviert · 9 gesamt · 4 Personen. Oben steht eine rote Karte „1 Buchung ist durch verspätete Rückgaben gefährdet“. Darunter „Bestand nach Typ“ mit Stundenpreisen. |
| 2 | Stundenpreis vorab | **+ Buchung** → Fahrrad *Canyon Grand Canyon* (Mountainbike), Person wählen, **Später**, Beginn morgen 10:00, Rückgabe 12:15 → **Preis berechnen** | Die Zusammenfassung zeigt 2 Std 15 Min, „3 angefangene Stunden × 8,00 €“ und **24,00 €**. Gebucht ist noch nichts. |
| 3 | Buchen | Danach **Verbindlich buchen** | Weiterleitung zu „Reserviert“ mit „Reservierung wurde angelegt.“. Das Rad bleibt „Verfügbar“ mit „reserviert ab …“. |
| 4 | Ausleihe ab sofort | **+ Buchung**, Fahrrad *Kalkhoff Agattu*, **Sofort**, Rückgabe in 3 Stunden → Preis berechnen → Verbindlich buchen | „Fahrrad wurde ausgeliehen.“. Das Dashboard zählt 1 ausgeliehen mehr, das Rad hat den Status „Ausgeliehen“. |
| 5 | Mindestabrechnung und -dauer | Rückgabe nur 30 Minuten nach Beginn | Feldfehler „Verleihdauer muss mindestens 1 Stunde betragen“ |
| 6 | Grenzen | Beginn in der Vergangenheit, Rückgabe vor Beginn oder mehr als 30 Tage | „Beginn darf nicht in der Vergangenheit liegen“, „Rückgabe muss nach dem Beginn liegen“, „Verleihdauer darf höchstens 30 Tage betragen“ |
| 7 | Überschneidung | *Cube Kathmandu Hybrid* für morgen 10:00–12:00 buchen (Erika hat 09:00–13:00) | „Fahrrad ist im gewählten Zeitraum bereits ausgeliehen oder reserviert.“ Morgen 13:00–14:00 wird dagegen angenommen, weil sich die Zeiträume nur berühren. |
| 8 | Geänderte Angaben | Nach „Preis berechnen“ die Rückgabe ändern und direkt **Verbindlich buchen** | Es wird nicht gebucht. Die neue Vorschau zeigt den neuen Preis und den Hinweis „Die Angaben haben sich geändert …“. |
| 9 | Verfügbarkeit | Menü **Verfügbarkeit**, Zeitraum und optional einen Typ wählen → Suchen | Freie Räder nach Typ gruppiert, jeweils mit Preis für den Zeitraum. **Buchen** öffnet das Formular mit Rad und Zeitraum. |
| 10 | Konflikt und Umbuchen | Dashboard → in der roten Karte **Umbuchen** | Es erscheinen alle freien Räder mit neuem Preis. Nach „Auf dieses Fahrrad umbuchen“ kommt „Buchung wurde umgebucht.“, und die rote Karte ist weg. |
| 11 | Verspätete Rückgabe | Dashboard → *Gazelle Esprit* (überfällig) → **Zurückgeben** | Unter **Buchungen → Abgeschlossen** steht der Endpreis mit nachberechneten Stunden. Jonas' Buchung zeigt 40,00 € (4,5 Std → 5 × 8 €). |
| 12 | Stornieren | Bei einer Reservierung **Stornieren** | „Reservierung wurde storniert.“. Der Zeitraum ist wieder frei. |
| 13 | Tarife | Menü **Tarife** | Stundensätze, Rechenregeln und Beispiele. Mit `-tarife "citybike=6"` stehen dort und überall sonst 6,00 €. |
| 14 | Fahrradformular | **Fahrräder → Neues Fahrrad**: E-Bike ohne Akku oder Citybike mit Akku speichern | „Für E-Bikes bitte die Akkukapazität angeben“ bzw. „Akkukapazität nur bei E-Bikes angeben“ |
| 15 | Wartung gesperrt | *Cube Kathmandu Hybrid* (reserviert) → Bearbeiten | Das Statusfeld ist gesperrt. |
| 16 | Filter | **Fahrräder**: Reiter „Reserviert“ und Typ „E-Bike“ kombinieren | Nur E-Bikes mit anstehender Reservierung |
| 17 | Person mit Ausweisbild | **Personen → Neue Person**, optional ein fiktives PNG/JPEG hochladen | Ohne Bild geht es auch. Mit Bild erscheint es in der Detailansicht. Eine Textdatei ergibt einen Feldfehler. |
| 18 | Handy | Browserfenster auf ca. 375 px verkleinern | Die Navigation steht oben, Tabellen scrollen in ihrer Karte, die Seite selbst nicht. |

## Architektur

> Das **Framework** weiß, *wie* verliehen wird. Ein **Plugin** sagt, *was* verliehen wird und *wie* abgerechnet wird.

```
framework/domain    Person, Item, Booking, Period, Money, Field, Fehler, Validierung
framework/plugin    Interfaces Plugin und PricingStrategy, Registry
framework/store     In-Memory-Speicher (threadsicher, gibt Kopien zurück)
framework/service   Anwendungsfälle: Buchen, Vorschau, Rückgabe, Storno, Umbuchen, Verfügbarkeit, Statistik
plugins/fahrrad     Fahrrad-Plugin (in der Web-App registriert)
plugins/ski         Beispiel-Plugin mit Tagestarif (nur in Tests)
internal/demo       fiktive Beispieldaten
internal/handler    HTTP-Schicht der Fahrrad-App
web/                Templates und CSS (per embed in der Binary)
main.go             einzige Stelle, an der Plugins registriert werden
```

| Framework (`framework/`) | Fahrrad-Plugin (`plugins/fahrrad`) |
|--------------------------|-------------------------------------|
| Personen, Objekte, Buchungen | Verleihart „Fahrrad“ |
| Reservierung, Ausleihe, Zustände aus der Uhrzeit | Typen Citybike, Mountainbike, E-Bike |
| Merkmalsprüfung anhand der Beschreibung (`Field`) | Merkmale Marke, Modell, Rahmengröße, Akku |
| ruft `Plugin.Validate` für Regeln über mehrere Felder auf | Regel „Akku nur bei E-Bikes, dort Pflicht“ |
| Verfügbarkeit, Überschneidungsschutz unter Sperre | – |
| Konflikte durch verspätete Rückgaben, Umbuchen | – |
| ruft `PricingStrategy.Quote` auf | `HourlyTariff`: angefangene Stunden × Satz je Typ |
| prüft Zeiträume gegen `Plugin.Limits()` | 1 Stunde bis 30 Tage |

**Nachweis der Austauschbarkeit:**
- `framework/service/plugin_test.go` registriert eine Verleihart `testgeraet`, die nur im Test existiert, und eine Pauschal-Preisstrategie. Alle Anwendungsfälle laufen damit (AK-20).
- Derselbe Test registriert Fahrrad und Ski **gleichzeitig**. Für 48 Stunden kostet ein Citybike 240 €, ein Ski 60 € (3 Kalendertage) (AK-21).
- `framework/architecture_test.go` liest den Quellcode mit `go/parser` und schlägt fehl, wenn eine dieser Regeln verletzt ist (AK-22):
  - Das Framework importiert keine Plugins und keinen Anwendungscode.
  - Im Framework-Code steht kein fahrrad- oder ski-spezifischer Begriff.
  - Plugins importieren nur `framework/domain` und `framework/plugin`.

**Eine neue Verleihart hinzufügen** (ohne Änderung am Framework):

```go
package kanu

type Plugin struct{}

func (Plugin) Kind() plugin.Kind                      { return plugin.Kind{Key: "kanu", Singular: "Kanu", Plural: "Kanus"} }
func (Plugin) Categories() []domain.Category          { return []domain.Category{{Key: "einer", Label: "Einer"}} }
func (Plugin) Fields() []domain.Field                 { return []domain.Field{{Key: "name", Label: "Name", Type: domain.FieldText, Required: true}} }
func (Plugin) Validate(domain.Item) domain.ValidationErrors { return nil }
func (Plugin) DisplayName(it domain.Item) string      { return it.Attr("name") }
func (Plugin) Pricing() plugin.PricingStrategy        { return halfDayRate{} } // eigene Strategie
func (Plugin) Limits() domain.BookingLimits           { return domain.BookingLimits{MaxDuration: 7 * 24 * time.Hour} }

// in main.go:  reg.Register(kanu.Plugin{})
```

## Preisregel Fahrrad

**Preis = angefangene Stunden × Stundensatz des Fahrradtyps, mindestens 1 Stunde.**

| Typ | Satz | Beispiel |
|-----|------|----------|
| Citybike | 5,00 € | 10:00–11:00 → 1 Std → 5,00 €; 10:00–11:01 → 2 Std → 10,00 €; 10:00–12:15 → 3 Std → 15,00 € |
| Mountainbike | 8,00 € | 09:00–12:30 → 4 Std → 32,00 € |
| E-Bike | 12,00 € | 09:00–17:00 → 8 Std → 96,00 €; 18:00–10:00 → 16 Std → 192,00 € |

- **Rückgabe:** Der tatsächliche Zeitpunkt wird gespeichert. Berechnet wird `[Beginn, max(geplant, tatsächlich))`.
  - Vorzeitige Rückgabe: Es gilt der gebuchte Preis.
  - Verspätete Rückgabe: Weitere angefangene Stunden kommen hinzu, z. B. E-Bike 09:00–12:00, zurück um 12:20 → 4 Std → 48,00 €.
- **Konfiguration:** über `fahrrad.Config` im Plugin (Sätze, Mindest- und Höchstdauer) oder beim Start über `-tarife`.
  Ungültige Angaben verhindern den Start, z. B. `ebike=0` oder `roller=5`.

## Akzeptanzkriterien → Tests

| AK | Inhalt | Test |
|----|--------|------|
| AK-1 | Dashboard-Zahlen | `internal/demo` `TestDemoNumbers`, `service` `TestStats`, `main_test.go` |
| AK-2, AK-3 | Fahrrad anlegen, Akku-Regel | `service` `TestCreateItemValidation`, `fahrrad` `TestValidateBattery`, `handler` `TestBikeCreate` |
| AK-4 | Status gesperrt, Löschen | `service` `TestUpdateAndDeleteItem`, `handler` `TestBikeLockedAndDelete` |
| AK-5, AK-26 | Personen, Ausweisbild | `service` `TestPersons`, `domain` `TestNewIDImage`, `handler` `TestPersonImage` |
| AK-6 | Verfügbarkeit | `service` `TestAvailable`, `handler` `TestAvailabilityPage` |
| AK-7 | Preisbeispiele, Konfiguration | `fahrrad` `TestHourlyTariffExamples`, `TestConfiguredRates`, `handler` `TestConfiguredRatesShown` |
| AK-8, AK-9 | Vorschau, geänderte Angaben | `service` `TestQuoteDoesNotStore`, `handler` `TestBookingPreview`, `TestBookingChangedAfterPreview` |
| AK-10 | Reservierung wird aktiv | `service` `TestBookNowAndReservation`, `handler` `TestBookingConfirm` |
| AK-11 | Überschneidungen | `domain` `TestPeriodOverlaps`, `service` `TestBookRejectsOverlaps`, `handler` `TestBookingErrors` |
| AK-12 | Parallele Buchungen | `service` `TestBookConcurrent`, `handler` `TestBookingConcurrentHTTP` (mit `-race`) |
| AK-13 | Rückgabe, Endpreis | `domain` `TestReturnFinalPrice`, `service` `TestReturn`, `handler` `TestReturnAndCancel` |
| AK-14 | Storno | `service` `TestCancel`, `handler` `TestReturnAndCancel` |
| AK-15 | Eingaberegeln | `domain` `TestValidatePeriod`, `service` `TestBookValidation`, `handler` `TestBookingErrors` |
| AK-16 | Wartung | `service` `TestBookMaintenance` |
| AK-17 | Filter | `service` `TestBookingFilters`, `TestItemFilters`, `handler` `TestBikeListFilters` |
| AK-18 | Tarif-Flag | `fahrrad` `TestParseRates`, `main_test.go` |
| AK-19 | Registry | `plugin` `TestRegister`, `TestRegisterRejectsInvalid`, `TestRegistryConcurrent` |
| AK-20, AK-21 | Austauschbarkeit | `service` `TestFrameworkWithTestPlugin`, `TestTwoRentalKindsSideBySide`, `ski` `TestDailyRate` |
| AK-22 | Architektur | `framework` `TestFrameworkImportsNoApplicationCode`, `TestFrameworkKnowsNoRentalKind`, `TestPluginsImportOnlyDomainAndPlugin` |
| AK-23 | Unbekannte Verleihart | `service` `TestCreateItemValidation` |
| AK-24, AK-25 | Konflikt, Umbuchen | `domain` `TestEvaluateConflictAndWaiting`, `TestWaitingChain`, `service` `TestConflictAndWaiting`, `TestReassign`, `handler` `TestConflictAndReassign`, `TestReassignWaitingRedirectsToActive` |
| AK-27 | Alle Links und Buttons | `handler` `TestCrawlAllLinks` (88 Seiten), `TestEveryFormButtonResponds` (53 Formulare) |
| F-8.3 | Nur Fahrräder in der Fahrrad-App | `handler` `TestBikeAppWithSkiPluginRegistered`, `TestBookingRejectsOtherKind` |

Nach jedem Test im Service prüft `checkInvariants` die Invarianten I-1, I-2a, I-3, I-4 und I-6.

## Abschlussprüfung

| # | Prüfpunkt | Ergebnis | Nachweis |
|---|-----------|----------|----------|
| 1 | Alle Menüpunkte und Buttons | ✅ | Crawler: 88 Seiten über Links erreichbar (alle 200), 53 Formulare abgeschickt (kein 404/405/500) |
| 2 | Fahrräder anlegen, bearbeiten, verwalten | ✅ | `TestBikeCreate`, `TestBikeEditHTTP` (inkl. Typwechsel, Wartung), `TestBikeLockedAndDelete`, `TestBikeListFilters` |
| 3 | Personen inkl. optionalem Ausweisbild | ✅ | `TestPersonImage`, `TestPersonEditImageHTTP` (ersetzen, behalten, entfernen, 413) |
| 4 | Reservierung für frei wählbare Zeiträume | ✅ | `TestBookNowAndReservation`, `TestBookingConfirm`, `TestAvailabilityPage` |
| 5 | Stundenpreise, angefangene Stunden | ✅ | `TestHourlyTariffExamples` (alle Beispiele aus 4.6), `TestHourlyTariffAcrossDST`, `TestConfiguredRates` |
| 6 | Keine Überschneidungen | ✅ | `TestPeriodOverlaps`, `TestBookRejectsOverlaps`, parallel: `TestBookConcurrent`, `TestBookingConcurrentHTTP` (mit `-race`) |
| 7 | Ausleihe, Rückgabe, Storno | ✅ | `TestReturn`, `TestCancel`, `TestReturnAndCancel` |
| 8 | Verspätete Rückgabe, gefährdete Folgereservierungen | ✅ | `TestConflictAndWaiting`, `TestWaitingChain`, `TestReassign`, `TestConflictAndReassign` |
| 9 | Dashboard-Kennzahlen | ✅ | `TestDemoNumbers` (3 Uhrzeiten), `TestDashboardFollowsActions` (nach Buchung, Ausleihe, Rückgabe, Storno) |
| 10 | Framework unabhängig von Fahrrad-Logik | ✅ | Architekturtest (Importe und Begriffe), Test-Plugin `testgeraet` |
| 11 | Registry, Ski-Plugin ohne Framework-Änderung | ✅ | `TestRegister…`, `TestTwoRentalKindsSideBySide`, `TestBikeAppWithSkiPluginRegistered` |
| 12 | Eigenständig ohne `ski-verleih` | ✅ | `go.mod` ohne `require`, keine Importe von `skiverleih`, Kopie außerhalb von `VOAufgabe` baut und besteht alle Tests |

**In der Abschlussprüfung behobene Fehler** (je mit Regressionstest in `internal/handler/regression_test.go`):

1. Das 64-KB-Limit für Formulare griff nicht, wenn der Content-Type einen `charset`-Zusatz hatte.
2. Über eine manipulierte ID ließ sich ein Objekt einer anderen Verleihart (z. B. Ski) über die Fahrrad-App buchen, sofern dieses Plugin registriert ist. Jetzt gilt es als ungültige Auswahl (F-8.3).
3. Nach dem Umbuchen einer wartenden Buchung führte die Weiterleitung zu „Reserviert“, obwohl die Buchung schon aktiv ist.

**Offene Punkte** (bewusst nicht umgesetzt, Spezifikation Abschnitt 12):
- **O-1:** Es gibt keine ausdrückliche Abholung. Eine Reservierung wird zur Startzeit automatisch aktiv, wie in 1b.
- **O-2:** Eine wartende Folgebuchung wird nach sehr später Rückgabe sofort aktiv, eventuell gleich überfällig.
- **O-3:** Tarife lassen sich nur beim Start ändern.
- **O-4:** Es gibt keine dauerhafte Speicherung.

## Entscheidungen

| Nr. | Entscheidung | Begründung |
|-----|--------------|------------|
| E-1 | Statische, interface-basierte Registry statt `plugin`-Paket von Go | Das `plugin`-Paket läuft nicht unter Windows, und dynamisches Laden ist nicht gefordert. Die Interfaces zeigen die Austauschbarkeit genauso. |
| E-2 | Objekte sind generisch (`Item` mit `Category` und `Attributes`), Merkmale beschreibt das Plugin (`Field`) | So braucht das Framework weder Generics noch fahrradspezifische Structs, und das Formular entsteht aus der Beschreibung. |
| E-3 | Prüfung in drei Stufen: Framework, Feldbeschreibung, `Plugin.Validate` | Einfache Regeln (Pflicht, Bereich) schreibt kein Plugin neu. Fachregeln über mehrere Felder bleiben im Plugin. |
| E-4 | Preis ist ein Ergebnis mit Erklärung (`Price{Amount, Explanation}`) | Vorschau, Liste und Endpreis zeigen nachvollziehbar, wie gerechnet wurde, egal welche Strategie es ist. |
| E-5 | Grenzen (Mindest- und Höchstdauer) kommen aus dem Plugin | Für Fahrräder gelten 1 Std bis 30 Tage, für Ski keine Mindestdauer bis 60 Tage, ohne `if` im Framework. |
| E-6 | „Beginn nicht in der Vergangenheit“ wird minutengenau geprüft, dazu gibt es die Option „Sofort“ | Die Formulare erfassen Minuten. „Sofort“ setzt den Beginn bei jeder Anfrage neu, damit eine Ausleihe ab jetzt nicht am Ausfüllen scheitert. |
| E-7 | Zustand `wartend` und Konflikt-Erkennung (Spezifikation 4.5a) | Eine verspätete Rückgabe überschneidet nachfolgende Reservierungen. Statt zwei „aktive“ Ausleihen zu zeigen, wird der Konflikt sichtbar und lässt sich durch Umbuchen oder Stornieren lösen. |
| E-8 | Tarife werden beim Start konfiguriert, nicht zur Laufzeit | So passen Vorschau, Buchung und Endpreis immer zusammen, ohne Tarif-Historie. |
| E-9 | Der Handler kennt das Fahrrad-Plugin direkt, das Framework nicht | Die Weboberfläche ist die Fahrrad-*Anwendung*. Wiederverwendbar ist das Framework darunter. |
| E-10 | Fehler im Framework sind neutral („objekt ist in wartung“), Texte mit „Fahrrad“ stehen nur in `handler/messages.go` | Eine Ski-App könnte dasselbe Framework mit eigenen Texten nutzen. |
| E-11 | `BookAsOf` / `ReturnAsOf` für Demo-Daten | Beispieldaten mit vergangenen Ausleihen laufen durch dieselben Regeln, statt Daten direkt in den Store zu schreiben. |
| E-12 | Eigenes Modul, kein Code aus `ski-verleih` importiert | Das Projekt muss allein als ZIP abgegeben werden können. Bewährte Teile (Layout, Personen, Preisvorschau) wurden als eigene Kopie übernommen und verallgemeinert. |
| E-13 | Eigenes Erscheinungsbild: Name „Velo-Verleih“, Fahrrad-Logo, Petrol statt Blau, Farbpunkte je Typ | angelehnt an den Ski-Verleih, aber als eigenständige Anwendung erkennbar |

## Fiktive Daten

Alle Namen, Ausweisnummern (Präfix `TEST`) und E-Mail-Adressen (`example.com`) sind erfunden.
Ausweisbilder werden nur im Arbeitsspeicher gehalten. Für den Prototyp bitte nur Testbilder verwenden.
