# Ski-Verleih

Webbasierter Prototyp für einen Skiverleih: Bestand verwalten, Personen mit Ausweis
registrieren, Ski für einen frei wählbaren Zeitraum ausleihen oder reservieren, den Preis
vorab sehen und Ski zurücknehmen.
Studienaufgabe Software Engineering (Aufgaben 1a und 1b), entwickelt nach
**Specification-Driven Development**. Grundlage ist [docs/SPEZIFIKATION.md](docs/SPEZIFIKATION.md) (v1.0).

## Stand der Umsetzung

| Aufgabe | Inhalt | Status |
|---|---|---|
| 1a | Ski, Personen, Ausweise, Ausleihen, Rückgaben, Dashboard | ✅ umgesetzt |
| 1b | Verleihbeginn und -dauer frei wählbar, Reservierungen, Preise, Prüfung auf Überschneidungen | ✅ umgesetzt |
| 1c | Fahrradverleih mit Plugin-Architektur | ✅ als eigenes Projekt `fahrrad-verleih` |
| Abschlussprüfung | Spezifikation v1.1, Korrektur für verspätete Rückgaben, Crawler über alle Links und Buttons, Neustart | ✅ siehe unten |

## Funktionen

- **Dashboard:** verfügbare und ausgeliehene Ski (inkl. „überfällig“), Reservierungen, Ski gesamt, Personen. Dazu getrennte Listen für aktive Ausleihen und anstehende Reservierungen sowie Schnellzugriffe
- **Ski:** anlegen, anzeigen, bearbeiten, löschen. Status verfügbar / ausgeliehen / Wartung, dazu „reserviert ab …“
- **Personen:** anlegen und verwalten, Ausweistyp und -nummer, optionales Ausweisbild (JPEG/PNG, max. 2 MB)
- **Buchen (1b):**
  - Beginn und geplante Rückgabe jeweils mit Datum und Uhrzeit wählen
  - Liegt der Beginn in der Zukunft, entsteht eine Reservierung
  - Verleihdauer und Preis werden automatisch berechnet und **vor** der verbindlichen Buchung angezeigt
- **Überschneidungen:** Ein Ski kann nie für überlappende Zeiträume gebucht werden, auch nicht bei gleichzeitigen Anfragen
- **Rückgabe:** Der Endpreis wird berechnet: keine Erstattung bei vorzeitiger Rückgabe, Aufpreis bei verspäteter
- **Stornieren:** Reservierungen lassen sich vor ihrem Beginn stornieren
- **Übersicht:** alle Buchungen, filterbar nach aktiv / reserviert / abgeschlossen / alle

## Preisregel (vorläufig, Aufgabe 1b)

> **20,00 € pro angefangenem Kalendertag, mindestens 20,00 €.**

- Gezählt wird jeder Kalendertag (Ortszeit des Servers), in dem ein Teil des Zeitraums `[Beginn, Rückgabe)` liegt.
- Weil das Ende nicht mitzählt, beginnt eine Rückgabe um genau 00:00 keinen neuen Tag.

| Zeitraum | Kalendertage | Preis |
|---|---|---|
| 02.11. 10:00 – 02.11. 10:30 | 1 | 20,00 € |
| 02.11. 10:00 – 02.11. 17:00 | 1 | 20,00 € |
| 02.11. 22:00 – 03.11. 08:00 | 2 | 40,00 € |
| 02.11. 10:00 – 03.11. 00:00 | 1 | 20,00 € |
| 02.11. 10:00 – 05.11. 10:00 | 4 | 80,00 € |

**Bei der Rückgabe** wird `[Beginn, max(geplante, tatsächliche Rückgabe))` abgerechnet:
- Vorzeitige Rückgabe: Es bleibt beim gebuchten Preis.
- Verspätete Rückgabe: Jeder weitere angefangene Kalendertag kostet zusätzlich.

Die Regel ist austauschbar (Schnittstelle `domain.PricingPolicy`, Implementierung `domain.DailyRate`,
eingesetzt in `main.go`). Details stehen in Spezifikation Abschnitt 3.7.

## Voraussetzungen

- [Go](https://go.dev/dl/) **1.22 oder neuer** (prüfen mit `go version`)
- Keine weiteren Abhängigkeiten, keine Datenbank

## Starten

```sh
cd ski-verleih
go run .                 # http://localhost:8080, leer
go run . -demo           # mit fiktiven Beispieldaten
go run . -addr :9000     # anderer Port
```

Die Demo-Daten umfassen 8 Ski, 4 Personen und 4 Buchungen. Alle Zeiten sind relativ zum Startzeitpunkt:
- Gipfelwerk 170 cm: aktiv
- Zwergenschanz 110 cm: aktiv und **überfällig**
- Pulverfee: **reserviert** ab übermorgen
- Firnflug 165 cm: abgeschlossen, 4 Stunden zu spät zurückgegeben
- Zwergenschanz 120 cm: in Wartung

Alle Daten liegen nur im Arbeitsspeicher. Nach einem Neustart sind sie weg.

## Im Browser testen

Starte mit `go run . -demo` und öffne http://localhost:8080.

### Neu in 1b

| Was | So geht's | Erwartung |
|---|---|---|
| Dashboard | Startseite | 5 verfügbar, 2 ausgeliehen („davon 1 überfällig“), 1 Reservierung, 8 gesamt, 4 Personen. Zwei getrennte Listen: aktive Ausleihen (eine mit rotem „Überfällig“) und anstehende Reservierungen |
| Preis vor der Buchung | *+ Ausleihe* → Ski + Person wählen → *Preis berechnen* | Zusammenfassung mit Zeitraum, Art, Verleihdauer, „x angefangene Kalendertage × 20,00 €“ und großem Preis. Es ist **noch nichts gebucht** |
| Preisregel prüfen | Beginn heute 22:00, Rückgabe morgen 08:00 → *Preis berechnen* | 2 Kalendertage = **40,00 €** |
| Ausleihe ab jetzt | Vorschau mit Beginn = jetzt → *Verbindlich buchen* | Meldung „Ski wurde ausgeliehen.“, Dashboard: verfügbar −1, ausgeliehen +1 |
| Reservierung | Beginn in 3 Tagen → *Preis berechnen* („Reservierung“) → *Verbindlich buchen* | „Reservierung wurde angelegt.“ Der Ski bleibt „Verfügbar“ mit „reserviert ab …“, die Kachel „Reservierungen“ +1, „ausgeliehen“ unverändert |
| Geänderte Angaben | Nach der Vorschau das Rückgabedatum ändern und direkt *Verbindlich buchen* | Es wird nicht gebucht. Neue Vorschau mit neuem Preis und dem Hinweis „Die Angaben haben sich geändert …“ |
| Überschneidung | Pulverfee (reserviert ab übermorgen) für einen Zeitraum buchen, der die Reservierung berührt | „Ski ist im gewählten Zeitraum bereits ausgeliehen oder reserviert.“ |
| Angrenzend erlaubt | Denselben Ski so buchen, dass die Rückgabe genau zum Beginn der Reservierung ist | Wird angenommen |
| Ausgeliehenen Ski reservieren | Gipfelwerk 170 cm (ausgeliehen) für **nach** seiner geplanten Rückgabe buchen | Wird angenommen. Für einen Zeitraum, der sich mit der Ausleihe überschneidet, kommt die Meldung oben |
| Überfällig blockiert | Zwergenschanz 110 cm (überfällig) für morgen buchen | Abgelehnt, bis der Ski zurückgegeben ist |
| Eingabefehler | Rückgabe vor dem Beginn, Beginn gestern oder mehr als 60 Tage | Rote Meldung direkt am Feld |
| Stornieren | Dashboard → *Stornieren* bei der Reservierung | „Reservierung wurde storniert.“, der Zeitraum ist wieder frei |
| Rückgabe mit Endpreis | *Ausleihen → Abgeschlossen* | Firnflug: 4 Stunden zu spät zurück, Endpreis inkl. Aufpreis |
| Wartung gesperrt | Pulverfee (reserviert) → *Bearbeiten* | Statusfeld gesperrt. Ein manipulierter POST wird mit „Ski ist ausgeliehen oder reserviert …“ abgelehnt |
| Wartung | Ski in Wartung (Zwergenschanz 120 cm) | Taucht im Buchungsformular nicht auf und hat keinen „Buchen“-Button |

### Weiterhin aus 1a

| Was | So geht's | Erwartung |
|---|---|---|
| Ski anlegen | *Ski → Neuer Ski*, Länge `300` | Meldung am Feld. Mit `170` → Detailseite + grüne Meldung |
| Ski löschen | Neu angelegten Ski → *Löschen* | Bestätigungsseite → gelöscht. Ski mit Buchungen: Hinweis statt Button |
| Person mit Ausweisbild | *Personen → Neue Person*, ein **fiktives** JPEG/PNG | Detailseite zeigt das Bild. PDF oder > 2 MB → Meldung |
| Doppelte Ausweisnummer | Personalausweis `test0001` | „Diese Ausweisnummer ist bereits registriert“ |
| Doppelt zurückgeben | Nach der Rückgabe mit „Zurück“ im Browser erneut senden | „Ausleihe wurde bereits zurückgegeben.“ |
| Handy-Ansicht | Fenster schmal ziehen (oder F12 → Gerätemodus) | Navigation oben, Kacheln untereinander, Tabellen scrollen in ihrer Karte |

## Tests

```sh
go test ./...            # alle Tests
go test -race ./...      # mit Race-Detector (braucht CGO_ENABLED=1 und gcc)
go vet ./...             # statische Prüfung
```

Die Tests nennen im Namen oder im Kommentar die Anforderung, die sie prüfen.
Zeitabhängige Tests laufen mit einer festen, steuerbaren Uhr. So lässt sich z. B. prüfen,
dass eine Reservierung zur Startzeit von selbst aktiv wird.

### Akzeptanzkriterien und Tests

| AK | Inhalt | Tests (Auswahl) |
|---|---|---|
| AK-1 | Kennzahlen inkl. Wartung | `service.TestDashboardStats`, `handler.TestDashboard_ShowsStats` |
| AK-2 | Leeres Dashboard | `handler.TestDashboard_Empty` |
| AK-3 | Länge 300 abgelehnt | `domain.TestSkiValidate`, `handler.TestSkiCreate_Invalid` |
| AK-4 | Ausgeliehener Ski nicht auf Wartung | `service.TestUpdateSki_InUse`, `handler.TestSkiUpdate_RentedCannotGoToMaintenance` |
| AK-5 | Doppelte Ausweisnummer | `service.TestCreatePerson_DuplicateID`, `handler.TestPersonCreate_DuplicateID` |
| AK-6 | Nur JPEG/PNG ≤ 2 MB | `domain.TestNewIDImage`, `handler.TestPersonCreate_InvalidImage` |
| AK-7 | Person ohne Bild | `handler.TestPersonCreate_WithoutImage` |
| AK-8 | Ausleihe ab jetzt ändert Status und Kennzahlen | `service.TestRent_Now_UpdatesDashboard`, `handler.TestRentalConfirm_Now` |
| AK-9 | Wartung / belegter Ski abgelehnt | `service.TestRent_MaintenanceAndRented`, `handler.TestRentalPreview_Conflicts` |
| AK-10 | Doppelte Rückgabe abgelehnt | `domain.TestRentalReturn`, `service.TestReturn`, `handler.TestRentalReturn` |
| AK-11 | Parallele Buchung desselben Zeitraums → genau eine | `service.TestRent_ConcurrentSameSki` (100 Goroutinen) |
| AK-12 | Preisbeispiele, 60-Tage-Grenze | `domain.TestDailyRate_SpecExamples`, `domain.TestValidateBooking`, `domain.TestCalendarDays_DST` |
| AK-13 | Vorschau zeigt Preis, speichert nichts | `service.TestQuote`, `handler.TestRentalPreview` |
| AK-14 | Geänderte Angaben → keine Buchung | `handler.TestRentalConfirm_ChangedAfterPreview` |
| AK-15 | Reservierung nicht sofort ausgeliehen, wird zur Startzeit aktiv | `domain.TestSkiStatusAt`, `service.TestRent_FutureReservation`, `handler.TestDashboard_ReservationsSeparate` |
| AK-16 | Überschneidungen (teilweise, umschließend, enthalten, angrenzend) | `domain.TestPeriodOverlaps`, `domain.TestFindConflict`, `service.TestRent_Overlaps` |
| AK-17 | Endpreis bei vorzeitiger/verspäteter Rückgabe, überfällig blockiert | `domain.TestRentalReturn`, `domain.TestRentalBlocks`, `service.TestReturn_LateAndOverdueBlocks` |
| AK-18 | Stornieren / Reservierung nicht zurückgebbar | `domain.TestRentalReturnAndCancel_Reservation`, `service.TestCancel`, `handler.TestRentalCancel` |
| AK-19 | Eingabefehler bei Datum/Uhrzeit | `domain.TestValidateBooking`, `handler.TestRentalPreview_InvalidDates` |
| AK-20 | Parallele überlappende Reservierungen → genau eine | `service.TestRent_ConcurrentOverlappingReservations`, `handler.TestRentalConfirm_ConcurrentHTTP` |
| AK-21 | Reservierter Ski nicht auf Wartung | `service.TestUpdateSki_InUse` |
| AK-22 | Verspätete Rückgabe mit Folgereservierung: Konflikt, „Wartet auf Rückgabe“, Storno | `domain.TestEvaluateConflictAndWaiting`, `service.TestOverdueRentalBlocksFollowingReservation`, `service.TestCancelWaitingBooking`, `handler.TestOverdueConflictHTTP` |
| AK-23 | Alle Links und Buttons, mit und ohne Demo-Daten | `handler.TestCrawlAllLinks`, `handler.TestEveryFormButtonResponds`, `handler.TestDashboardFollowsActions` |

Nach jedem Service-Szenario prüft `checkInvariants` die Invarianten I-1 bis I-4 und I-6,
darunter: „keine zwei Belegungen eines Ski überschneiden sich“.

## Aufbau

```
handler → service → store → domain     (Abhängigkeiten nur nach unten)
```

| Pfad                 | Inhalt                                                 |
|----------------------|--------------------------------------------------------|
| `main.go`            | Start, Flags, Verdrahtung (Store, Preisregel, Service) |
| `internal/domain`    | Fachmodell: Ski, Person, Buchung, `Period`, `Money`, `PricingPolicy`, Validierung, Fehler |
| `internal/store`     | In-Memory-Speicher (Maps, threadsicher)                |
| `internal/service`   | Anwendungsfälle: Vorschau, Buchen, Rückgabe, Stornieren, Kennzahlen |
| `internal/handler`   | HTTP-Routen, Formulare, Templates, Fehlerseiten        |
| `web/templates`      | HTML-Templates (Layout, Icons, Partials, Seiten)       |
| `web/static`         | CSS                                                    |
| `docs/`              | Spezifikation                                          |

## Wichtige Entscheidungen

### Aus 1a

| # | Entscheidung | Begründung |
|---|---|---|
| E-1 | **Specification-Driven Development:** Erst ändert sich die Spezifikation, dann der Code | Anforderungen sind nachvollziehbar, und jeder Test lässt sich einem Akzeptanzkriterium zuordnen |
| E-2 | **Nur die Go-Standardbibliothek** (`net/http`, `html/template`, `embed`) | Vorgabe. Weniger Abhängigkeiten, leichter zu verstehen. Seit Go 1.22 kann der `ServeMux` Muster wie `GET /ski/{id}` |
| E-3 | **Vier Schichten** domain → store → service → handler | Klare Verantwortlichkeiten. Die Geschäftslogik lässt sich ohne HTTP testen |
| E-4 | **Fachregeln als Methoden am Domain-Objekt** (`Rental.Return`, `Rental.Blocks`, `Ski.StatusAt`) | Jede Regel steht an genau einer Stelle und ist direkt getestet |
| E-5 | **Service mit `RWMutex`**: Schreib-Sperre fürs Buchen, Zurückgeben, Stornieren und Ändern, Lese-Sperre für Abfragen über mehrere Tabellen | Prüfung auf Überschneidung und Anlegen passieren unter derselben Sperre, deshalb kann es keine Doppelbuchung geben (AK-11, AK-20). Abfragen sehen nie einen halb fertigen Zustand |
| E-6 | **Ski und Personen mit Buchungen werden nicht gelöscht** | Die Historie bleibt vollständig. Statt zu löschen, setzt man einen Ski auf „Wartung“ |
| E-7 | **Status „Wartung“** | So kann man einen Ski realistisch verwalten, ohne ihn zu löschen |
| E-8 | **Serverseitiges HTML, kein JavaScript.** Löschen geht über eine Bestätigungsseite | Vorgabe „möglichst wenig JS“. Einfach, robust, barrierearm |
| E-9 | **Ausweisbild im Arbeitsspeicher**, Typ wird am Dateiinhalt erkannt, max. 2 MB, nur JPEG/PNG | Erlaubt für den Prototyp. Die Prüfung am Inhalt verhindert getarnte Dateien. Abruf nur über eine eigene Route |
| E-10 | **Nur fiktive Testdaten** (Namen wie „Erika Mustermann“, Ausweisnummern mit Präfix `TEST`, `example.com`) | Datenschutz. Es landen keine echten personenbezogenen Daten im Projekt |
| E-11 | **Ausweisnummer in Listen maskiert** (`****1234`) | Datenschutz. Vollständig erscheint sie nur auf der Detailseite |
| E-12 | **Gestaltung als Admin-Dashboard:** Seitenleiste, Kennzahlen-Kacheln, Karten, Status-Badges, Farben über CSS-Variablen, Inline-SVG-Icons | Modern und übersichtlich, ohne Icon- oder CSS-Framework |
| E-13 | **Templates und CSS in die Binary eingebettet** (`embed`) | Das Programm läuft als einzelne Datei |
| E-14 | **Bezeichner im Code englisch, Oberfläche deutsch.** Die Enum-Werte sind deutsch (`verfuegbar`) | Übliche Go-Konvention. Die Enum-Werte erscheinen in URLs und Formularen |
| E-15 | **Uhr als Funktion im Service** (`now func() time.Time`) | Zeitabhängige Tests sind deterministisch |
| E-16 | **Post/Redirect/Get** mit Meldungen über feste Schlüssel (`?msg=rented`) | Neu laden sendet nichts doppelt. Über die URL lässt sich kein fremder Text einschleusen |
| E-17 | **Rücksprung nur auf lokale Pfade** (`safeBack`) | Verhindert Weiterleitungen auf fremde Seiten (Open Redirect) |
| E-18 | **Formular-Upload komplett im Arbeitsspeicher**, Anfrage auf 3 MB begrenzt | Vorgabe „In-Memory“: keine temporären Dateien |

### Neu in 1b

| # | Entscheidung | Begründung |
|---|---|---|
| E-19 | **„Ausgeliehen“ und der Buchungszustand werden aus der Uhrzeit abgeleitet**, nicht gespeichert. Gespeichert wird nur der Grundstatus verfügbar/Wartung | Eine Reservierung wird zur Startzeit von selbst zur Ausleihe, ohne Hintergrundjob und ohne Zustand, der veralten kann (Anforderung 9: Reservierungen markieren den Ski nicht sofort) |
| E-20 | **Zeiträume halboffen** `[Beginn, Ende)` als eigener Typ `Period` mit `Overlaps` | Die Regel `a < d && c < b` ist eine reine, gut testbare Funktion. Angrenzende Buchungen (Rückgabe 10:00, nächster Beginn 10:00) sind erlaubt |
| E-21 | **Belegung statt nur geplanter Zeitraum**: überfällige Ausleihen blockieren bis zur Rückgabe, vorzeitige Rückgaben geben die Restzeit frei | So entspricht die Verfügbarkeit dem, was physisch im Laden steht |
| E-22 | **Geld als ganze Cent (`int64`)**, Anzeige im deutschen Format | Keine Rundungsfehler durch Gleitkommazahlen |
| E-23 | **Preisregel als Interface `PricingPolicy`** (Preis, Erklärung, Regeltext), Implementierung `DailyRate` | Austauschbar ohne Änderung am Service. Vorbereitung auf stundenweise Abrechnung in 1c |
| E-24 | **Kalendertage statt 24-Stunden-Blöcke**, Zählung über reine Datumsangaben | Entspricht „angefangener Kalendertag“. Auch über die Zeitumstellung korrekt (getestet) |
| E-25 | **Zwei Schritte: „Preis berechnen“ → „Verbindlich buchen“** mit Abgleich der geprüften Angaben | Der Preis ist sicher vor der Bestätigung sichtbar, ganz ohne JavaScript. Wer nach der Vorschau etwas ändert, bekommt erst eine neue Vorschau. Gespeichert wird immer der vom Server berechnete Preis |
| E-26 | **Datum und Uhrzeit als getrennte Felder** (`type="date"` und `type="time"`) | Entspricht der Anforderung „Datum und Uhrzeit auswählen“. Native Auswahlhilfen im Browser, ohne Bibliothek |
| E-27 | **Grenzen: Beginn höchstens 15 Minuten in der Vergangenheit, Dauer höchstens 60 Tage** | Fängt Tippfehler ab und deckt die Zeit ab, die das Ausfüllen des Formulars dauert |
| E-28 | **Stornieren löscht die Reservierung**, nur vor ihrem Beginn möglich | Eine nie angetretene Reservierung gehört nicht in die Ausleih-Historie |

### Neu in der Abschlussprüfung (Version 1.1)

| Nr. | Entscheidung | Begründung |
|---|---|---|
| E-29 | **Zustand „Wartet auf Rückgabe“ und Konflikt-Markierung** (Spezifikation 3.6a) | Kam ein Ski zu spät zurück, waren eine nachfolgende Reservierung und die überfällige Ausleihe gleichzeitig „aktiv“. Jetzt hat ein Ski immer höchstens eine aktive Ausleihe, und das Dashboard warnt. |
| E-30 | **Wartende Buchungen lassen sich stornieren, aber nicht zurückgeben**; kein Umbuchen | Der Ski wurde nie ausgegeben. Umbuchen war in 1b nicht gefordert; stornieren und neu buchen genügt. |

Weitere Annahmen zu 1b stehen in Spezifikation Abschnitt 11.

## Abschlussprüfung

| # | Prüfpunkt | Ergebnis | Nachweis |
|---|---|---|---|
| 1 | Menüpunkte, Formulare, Buttons | ✅ | Crawler: 61 Seiten mit Demo-Daten und 14 ohne Daten, alle Status 200. 35 Formulare abgeschickt, keine Antwort 404, 405 oder 500 |
| 2 | Ski anlegen, bearbeiten, löschen | ✅ | `TestSkiCreate…`, `TestSkiUpdate…`, `TestSkiDelete` |
| 3 | Personen mit Ausweistyp, -nummer und -bild | ✅ | `TestPersonCreate_*`, `TestPersonUpdate_Image`, Domain-Tests zu Ausweisnummer und Bild |
| 4 | Ausleihen und zurückgeben | ✅ | `TestRentalConfirm_Now`, `TestRentalReturn` |
| 5 | Flexibler Beginn und Dauer | ✅ | `TestValidateBooking`, `TestRentalPreview_InvalidDates` |
| 6 | Mietpreise | ✅ | Beispiele aus 3.7, Sommerzeit, Endpreis bei verspäteter Rückgabe |
| 7 | Keine Überschneidungen | ✅ | `TestPeriodOverlaps`, parallele Buchungen mit `-race` |
| 8 | Reservierungen und Storno | ✅ | `TestRentalConfirm_Reservation`, `TestRentalCancel` |
| 9 | Überfällige Ausleihen | ✅ (Fehler behoben) | `TestOverdueRentalBlocksFollowingReservation`, `TestOverdueConflictHTTP` |
| 10 | Dashboard-Kennzahlen | ✅ | `TestDashboardFollowsActions` (Demo, nach Ausleihe, Reservierung, Rückgabe, Storno) |
| 11 | Ohne Demo-Daten | ✅ | Crawler ohne Daten, `TestDashboard_Empty` |
| 12 | Neustart | ✅ | echtes Programm dreimal gestartet: mit Demo-Daten, ohne Daten, wieder mit Demo-Daten. Daten liegen nur im Speicher und sind nach dem Neustart erwartungsgemäß weg; alle Seiten laden, IDs beginnen wieder bei 1 |

**Gefundener und behobener Fehler:** Wurde eine Ausleihe nicht rechtzeitig zurückgegeben und war der Ski danach schon reserviert,
galten ab dem Beginn der Reservierung **beide** Buchungen als aktiv. Die nie ausgegebene Reservierung ließ sich sogar „zurückgeben“.
Jetzt ist sie als **Konflikt** markiert und ab ihrem Beginn **„Wartet auf Rückgabe“**. Sie kann storniert, aber nicht zurückgegeben werden,
und das Dashboard zeigt eine Warnkarte (Spezifikation 3.6a, AK-22).

**Bewusst offen** (Spezifikation Abschnitt 11):
- Keine dauerhafte Speicherung, nach einem Neustart sind die Daten weg.
- Keine ausdrückliche Ausgabe des Ski: Eine Reservierung wird zur Startzeit automatisch aktiv.
- Kein Umbuchen einer gefährdeten Reservierung auf einen anderen Ski; stattdessen stornieren und neu buchen.

## Ausblick

- **1c** ist als eigenständiges Projekt `fahrrad-verleih` umgesetzt (stundenweise Abrechnung, Verleih-Framework mit Plugins).
  Es übernimmt die Muster `Period`/`Overlaps`, `Money` und die austauschbare Preisregel, ohne Code aus diesem Projekt zu importieren.
