# Spezifikation – Ski-Verleihsystem (Aufgaben 1a + 1b)

| Feld        | Wert                                   |
|-------------|----------------------------------------|
| Version     | 1.1                                    |
| Stand       | 2026-11-02                             |
| Status      | **Freigegeben** – 1a und 1b umgesetzt, abschließend geprüft |
| Gilt für    | Aufgabe 1a (Grundsystem) und 1b (Verleihbeginn, Verleihdauer, Preise) |

### Änderungshistorie

| Version | Änderung |
|---------|----------|
| 0.1 | Erster Entwurf |
| 0.2 | Freigabe inkl. aller Annahmen aus Abschnitt 11. Ergänzt: F-4.5 (kein Ski mehrfach gleichzeitig ausgeliehen), F-6.7 (Gestaltung im Dashboard-Stil), NF-11 (nur fiktive Testdaten), 9.1 (Prüfung auf Überschneidungen in 1b ist Pflicht) |
| 0.3 | Präzisierungen für die Umsetzung: Rücksprung nach der Rückgabe (F-4.3), Anzeige der Dauer unter 1 Stunde (F-5.1), Liste der Meldungen (F-6.2), Hinweis auf Historie schon auf der Lösch-Bestätigung (F-6.3), Upload-Obergrenze für die gesamte Anfrage (3.4), doppelte Ausweisnummer als Feldfehler (Abschnitt 7) |
| 1.0 | **Aufgabe 1b:** Buchungen mit wählbarem Beginn und geplanter Rückgabe, Reservierungen in der Zukunft, Preisregel (3.7), Belegung und Überschneidungsprüfung (3.6), Status „ausgeliehen“ wird aus der Zeit abgeleitet (3.1, 3.5), Preisvorschau vor der Bestätigung (F-4), Stornieren von Reservierungen, Dashboard mit Reservierungen (F-1). Neue Akzeptanzkriterien AK-12 bis AK-21. AK-8, AK-9 und AK-11 an 1b angepasst. |
| 1.1 | **Abschlussprüfung:** Fehlerkorrektur für verspätete Rückgaben mit nachfolgender Reservierung (3.6a): Zustand `wartend`, Kennzeichnung „Konflikt“, Hinweis auf dem Dashboard (F-1.4), Stornieren wartender Buchungen. I-2 präzisiert, I-2a ergänzt. Neu: AK-22 (Konflikt) und AK-23 (alle Links und Buttons). |

Diese Spezifikation ist die **einzige verbindliche Quelle** für Verhalten und
Struktur des Systems (Specification-Driven Development). Code, Tests und
Oberfläche werden aus ihr abgeleitet. Änderungen am Verhalten werden **zuerst
hier** eingetragen und erst danach umgesetzt.

---

## 1. Ziel und Abgrenzung

### 1.1 Ziel
Ein webbasierter Prototyp, mit dem Mitarbeitende eines Skiverleihs ihren
Ski-Bestand und ihre Kundschaft verwalten, Ski für einen frei wählbaren Zeitraum
verleihen oder im Voraus reservieren, den Preis vorab sehen und Ski zurücknehmen können.

### 1.2 Im Umfang
**1a:**
- Dashboard mit Kennzahlen und Schnellzugriffen
- Ski anlegen, anzeigen, bearbeiten, löschen
- Personen anlegen, anzeigen, bearbeiten, löschen, inkl. Ausweisdaten und optionalem Ausweisbild
- Ski an Personen ausleihen und zurückgeben
- Übersicht aller Ausleihen

**1b:**
- Verleihbeginn per Datum und Uhrzeit wählbar, auch in der Zukunft (Reservierung)
- Geplante Rückgabe per Datum und Uhrzeit
- Automatische Berechnung von Verleihdauer und Mietpreis, sichtbar vor der Bestätigung
- Keine zeitlich überlappenden Buchungen für denselben Ski
- Reservierungen stornieren
- Dashboard unterscheidet aktive Ausleihen und zukünftige Reservierungen

### 1.3 Nicht im Umfang
- Fahrradverleih, Plugin-Architektur → **1c**
- Verschiedene Tarife je Ski-Typ, Rabatte, Kaution, Saisonpreise (die Preisregel ist austauschbar, siehe 3.7)
- Nachträgliches Ändern einer Buchung (stattdessen: stornieren und neu buchen)
- Login, Benutzer- und Rechteverwaltung
- Dauerhafte Speicherung (Datenbank, Dateien): nach einem Neustart sind alle Daten weg
- Mehrsprachigkeit, Zahlungsabwicklung, Rechnungen, E-Mail-Versand

---

## 2. Begriffe

| Begriff        | Bedeutung                                                                        |
|----------------|----------------------------------------------------------------------------------|
| Ski            | Ein einzelnes, physisch vorhandenes Paar Ski im Bestand                          |
| Person         | Kundin oder Kunde, die/der Ski ausleihen kann                                    |
| Ausweis        | Ausweisdokument einer Person (Typ + Nummer, optional ein Bild)                   |
| Buchung        | Oberbegriff: genau ein Ski für genau eine Person in einem Zeitraum. Im Code `Rental`. In der Oberfläche heißt sie „Ausleihe“ bzw. „Reservierung“, je nach Zustand. |
| Zeitraum       | Halboffenes Intervall `[Beginn, Ende)`: Der Beginn gehört dazu, das Ende nicht.    |
| Reservierung   | Buchung, deren Beginn in der Zukunft liegt (Zustand `reserviert`)                |
| Aktive Ausleihe | Buchung, deren Beginn erreicht ist und die noch nicht zurückgegeben wurde (Zustand `aktiv`) |
| Überfällig     | Aktive Ausleihe, deren geplante Rückgabe schon vorbei ist                         |
| Abgeschlossene Ausleihe | Buchung mit tatsächlicher Rückgabe (Zustand `abgeschlossen`)            |
| Belegung       | Der Zeitraum, in dem eine Buchung den Ski blockiert (siehe 3.6)                   |
| Kalendertag    | Ein Datum von 00:00 bis 24:00 in der Zeitzone des Servers                         |

**Nutzerrolle:** Es gibt nur eine Rolle, *Mitarbeitende*. Sie dürfen alles.

---

## 3. Domänenmodell

### 3.1 Ski

| Feld      | Typ        | Pflicht | Regeln                                                         |
|-----------|------------|---------|----------------------------------------------------------------|
| ID        | int        | auto    | Fortlaufend ab 1, wird nie wiederverwendet                     |
| Marke     | string     | ja      | 1–50 Zeichen, nach Trimmen nicht leer                          |
| Modell    | string     | ja      | 1–50 Zeichen, nach Trimmen nicht leer                          |
| Länge     | int (cm)   | ja      | 70–220                                                         |
| Typ       | SkiTyp     | ja      | `allround`, `carving`, `freeride`, `touren`, `kinder`          |
| Zustand   | Zustand    | ja      | `neu`, `gut`, `gebraucht`                                      |
| Grundstatus | SkiStatus | auto   | gespeichert: nur `verfuegbar` oder `wartung`                   |
| Notiz     | string     | nein    | max. 200 Zeichen                                               |

Regeln zum Status (**geändert in 1b**):
- Gespeichert wird nur der **Grundstatus** `verfuegbar` oder `wartung`. Neu angelegte Ski sind `verfuegbar`.
- Der **angezeigte Status** wird zu jedem Zeitpunkt aus Grundstatus und Buchungen **abgeleitet**:
  `wartung`, falls der Grundstatus `wartung` ist. Sonst `ausgeliehen`, falls es eine aktive Ausleihe gibt.
  Sonst `verfuegbar`. So wird eine Reservierung zur Startzeit automatisch zur Ausleihe, ohne dass jemand etwas speichern muss.
- Hat ein verfügbarer Ski eine Reservierung, zeigt die Oberfläche zusätzlich „reserviert ab …“ an (nächste Reservierung).
- Zwischen `verfuegbar` und `wartung` dürfen Mitarbeitende manuell wechseln, solange der Ski **weder aktiv
  ausgeliehen noch reserviert** ist.

### 3.2 Person

| Feld          | Typ          | Pflicht | Regeln                                                           |
|---------------|--------------|---------|------------------------------------------------------------------|
| ID            | int          | auto    | Fortlaufend ab 1                                                 |
| Vorname       | string       | ja      | 1–50 Zeichen                                                     |
| Nachname      | string       | ja      | 1–50 Zeichen                                                     |
| Telefon       | string       | nein    | max. 30 Zeichen, erlaubt: Ziffern, Leerzeichen, `+ - / ( )`      |
| E-Mail        | string       | nein    | max. 100 Zeichen, muss `@` enthalten, falls angegeben            |
| Ausweistyp    | AusweisTyp   | ja      | `personalausweis`, `reisepass`, `fuehrerschein`                  |
| Ausweisnummer | string       | ja      | 4–30 Zeichen, nur Buchstaben und Ziffern, wird in Großbuchstaben gespeichert |
| Ausweisbild   | Bild         | nein    | JPEG oder PNG, max. 2 MB, siehe 3.4                              |
| Angelegt am   | Zeitpunkt    | auto    | Zeitpunkt der Registrierung                                      |

Die Kombination aus **Ausweistyp + Ausweisnummer muss eindeutig** sein.

### 3.3 Buchung (Ausleihe / Reservierung) – **erweitert in 1b**

| Feld               | Typ         | Pflicht | Regeln                                                       |
|--------------------|-------------|---------|--------------------------------------------------------------|
| ID                 | int         | auto    | Fortlaufend ab 1                                             |
| SkiID              | int         | ja      | Verweis auf existierenden Ski                                |
| PersonID           | int         | ja      | Verweis auf existierende Person                              |
| Beginn             | Zeitpunkt   | ja      | frei wählbar (Datum + Uhrzeit), auch in der Zukunft          |
| Geplante Rückgabe  | Zeitpunkt   | ja      | Datum + Uhrzeit, muss **nach** dem Beginn liegen             |
| Gebucht am         | Zeitpunkt   | auto    | Zeitpunkt, an dem die Buchung angelegt wurde                 |
| Preis              | Geld (Cent) | auto    | bei der Buchung berechnet für `[Beginn, geplante Rückgabe)`  |
| Tatsächliche Rückgabe | Zeitpunkt? | nein  | leer = nicht zurückgegeben                                   |
| Endpreis           | Geld (Cent) | auto    | wird bei der Rückgabe berechnet (siehe 3.7)                  |

Regeln für eine **neue** Buchung (AK-19):
- Beginn und geplante Rückgabe müssen gültige Zeitpunkte sein.
- Die geplante Rückgabe liegt nach dem Beginn.
- Der Beginn liegt höchstens **15 Minuten in der Vergangenheit**. Die Toleranz deckt die Zeit ab, die das Ausfüllen des Formulars dauert. Rückwirkende Buchungen sind nicht vorgesehen.
- Die Dauer beträgt höchstens **60 Tage**, um Tippfehler (z. B. falsches Jahr) abzufangen.
- Der Ski ist nicht in Wartung und im Zeitraum frei (3.6).

Geldbeträge werden als ganze Cent (`int64`) gespeichert, nie als Gleitkommazahl.
Sie werden im deutschen Format angezeigt: `1.240,00 €`.

### 3.4 Ausweisbild
- Wird **im Speicher** gehalten (Bytes + MIME-Typ), nicht auf der Festplatte.
- Der MIME-Typ wird aus dem Dateiinhalt ermittelt (`http.DetectContentType`), nicht aus dem Dateinamen.
- Abruf nur über eine eigene Route (siehe Abschnitt 6); es gibt keinen frei zugänglichen Upload-Ordner.
- Beim Bearbeiten einer Person kann das Bild ersetzt oder entfernt werden. Wird kein neues Bild gewählt, bleibt das alte erhalten.
- Die gesamte Formular-Anfrage ist auf 3 MB begrenzt und wird vollständig im Arbeitsspeicher verarbeitet,
  ohne temporäre Dateien. Ist eine Datei zwischen 2 und 3 MB groß, erscheint die Meldung aus AK-6 direkt
  am Formularfeld. Ist die Anfrage größer als 3 MB, antwortet der Server mit `413` und einer Fehlerseite, die dieselbe Meldung zeigt.
- Das Bild wird mit `Cache-Control: no-store` und `X-Content-Type-Options: nosniff` ausgeliefert.

### 3.5 Zustände einer Buchung (neu in 1b)

Der Zustand wird aus den Zeitpunkten und der aktuellen Uhrzeit `jetzt` abgeleitet. Er wird nicht gespeichert.

| Zustand         | Bedingung                                     | Erlaubte Aktion          |
|-----------------|-----------------------------------------------|--------------------------|
| `reserviert`    | keine Rückgabe und `jetzt < Beginn`           | Stornieren               |
| `wartend`       | keine Rückgabe, `Beginn ≤ jetzt`, Ski aber noch durch eine frühere, überfällige Ausleihe belegt (3.6a) | Stornieren |
| `aktiv`         | keine Rückgabe, `Beginn ≤ jetzt`, nicht wartend | Zurückgeben            |
| ↳ überfällig    | `aktiv` und `jetzt ≥ geplante Rückgabe`       | Zurückgeben              |
| `abgeschlossen` | tatsächliche Rückgabe gesetzt                 | –                        |

```
 Buchung anlegen ──► reserviert ──(Beginn erreicht)──► aktiv ──(Zurückgeben)──► abgeschlossen
        │                 │                              │
        │            (Stornieren)                  (geplante Rückgabe vorbei)
        │                 ▼                              ▼
        │             gelöscht                       überfällig ──(Zurückgeben)──► abgeschlossen
        └─ Beginn ≤ jetzt ──► sofort aktiv
```

- Eine Reservierung kann **nicht zurückgegeben** werden, weil der Ski noch gar nicht ausgegeben wurde. Sie wird stattdessen storniert.
- Beim **Stornieren** wird die Reservierung gelöscht, weil sie nie stattgefunden hat. Danach ist der Zeitraum wieder frei.
- Eine aktive Ausleihe kann nicht storniert werden, nur zurückgegeben.
- Die Rückgabe setzt die tatsächliche Rückgabe auf `jetzt`.

### 3.6 Belegung und Überschneidungen (neu in 1b)

Jede Buchung blockiert ihren Ski in einem **Belegungszeitraum**:

| Buchung                                      | Belegung                                  |
|----------------------------------------------|-------------------------------------------|
| reserviert oder aktiv, nicht überfällig      | `[Beginn, geplante Rückgabe)`             |
| aktiv und überfällig                         | `[Beginn, ∞)`: blockiert, bis der Ski zurück ist |
| abgeschlossen                                | `[Beginn, tatsächliche Rückgabe)`. Eine vorzeitige Rückgabe gibt die Restzeit frei |

Zwei Zeiträume `[a, b)` und `[c, d)` **überschneiden sich**, wenn `a < d && c < b`.
Angrenzende Zeiträume (Rückgabe 10:00, nächster Beginn 10:00) überschneiden sich also **nicht**.

Eine neue Buchung für Ski S mit Zeitraum Z wird abgelehnt, wenn Z sich mit der Belegung
irgendeiner anderen Buchung von S überschneidet (AK-16). Prüfen und Anlegen passieren
atomar, damit parallele Anfragen keine Überschneidung erzeugen (AK-20).

### 3.6a Verspätete Rückgabe und nachfolgende Reservierungen (neu in 1.1)

Beim Buchen überschneiden sich Buchungen nie. Wird eine Ausleihe aber **nicht rechtzeitig zurückgegeben**, wächst ihre
Belegung auf `[Beginn, ∞)` und kann eine bereits bestehende **nachfolgende** Buchung desselben Ski treffen.

- Eine nicht zurückgegebene Buchung B hat einen **Konflikt**, wenn eine andere, nicht zurückgegebene Buchung A desselben Ski
  mit früherem Beginn existiert, deren aktuelle Belegung den geplanten Zeitraum von B überschneidet.
- Hat B noch nicht begonnen, bleibt sie `reserviert` und wird als **Konflikt** markiert.
- Ist der Beginn von B erreicht, ist B `wartend` (Badge „Wartet auf Rückgabe“) statt `aktiv`. So hat ein Ski nie zwei aktive Ausleihen.
  Eine wartende Buchung kann nicht zurückgegeben, aber storniert werden.
- Wird A zurückgegeben, ist der Konflikt aufgelöst: B ist wieder `reserviert` bzw. sofort `aktiv`.
- Das Dashboard zeigt Konflikte in einer roten Hinweiskarte (F-1.4).

### 3.7 Preisregel (neu in 1b)

> **20,00 € pro angefangenem Kalendertag, mindestens 20,00 €.**

- Gezählt wird jeder Kalendertag (Datum in der Zeitzone des Servers), in dem mindestens ein Teil
  des Zeitraums `[Beginn, Ende)` liegt.
- Weil das Ende nicht zum Zeitraum gehört, zählt eine Rückgabe um genau 00:00 den neuen Tag **nicht** mit.
- Es wird mindestens 1 Tag berechnet, also mindestens 20,00 €.
- Es geht um Kalendertage, nicht um 24-Stunden-Blöcke: Von 22:00 bis 08:00 am nächsten Tag sind es 2 Kalendertage.

| Zeitraum                                   | Kalendertage | Preis   |
|--------------------------------------------|--------------|---------|
| 02.11. 10:00 – 02.11. 10:30                | 1            | 20,00 € |
| 02.11. 10:00 – 02.11. 17:00                | 1            | 20,00 € |
| 02.11. 22:00 – 03.11. 08:00                | 2            | 40,00 € |
| 02.11. 10:00 – 03.11. 00:00                | 1            | 20,00 € |
| 02.11. 10:00 – 05.11. 10:00                | 4            | 80,00 € |

**Preis bei Buchung:** Er wird für `[Beginn, geplante Rückgabe)` berechnet und vor der Bestätigung angezeigt.

**Endpreis bei Rückgabe:** Er wird für `[Beginn, max(geplante Rückgabe, tatsächliche Rückgabe))` berechnet:
- Bei vorzeitiger Rückgabe gilt der gebuchte Preis, es gibt keine Erstattung.
- Bei verspäteter Rückgabe kommen weitere angefangene Kalendertage hinzu.

**Austauschbarkeit:** Die Regel ist als Schnittstelle `PricingPolicy` umgesetzt (Preis, Erklärung, Regeltext).
Die konkrete Regel `DailyRate` (Tagessatz) wird in `main.go` eingesetzt. Andere Tarife wie Stundenpreise in 1c
oder Preise je Ski-Typ brauchen also eine neue Implementierung, aber keine Änderung am Service.

### 3.8 Invarianten
Die folgenden Aussagen müssen **nach jeder Operation** gelten. Tests prüfen sie.

- **I-1** Der angezeigte Status `ausgeliehen` gilt genau dann, wenn der Ski eine aktive Ausleihe hat (Ableitung aus 3.1).
- **I-2** Beim Anlegen überschneidet sich eine Buchung mit keiner Belegung desselben Ski (3.6). Spätere Überschneidungen entstehen nur
  durch überfällige Ausleihen und sind dann als Konflikt markiert (3.6a).
- **I-2a** Jeder Ski hat höchstens eine Buchung im Zustand `aktiv`.
- **I-3** Jede Buchung verweist auf einen existierenden Ski und eine existierende Person.
- **I-4** Die geplante Rückgabe liegt nach dem Beginn. Die tatsächliche Rückgabe liegt, falls gesetzt, nicht vor dem Beginn.
- **I-5** Ausweistyp + Ausweisnummer sind über alle Personen eindeutig.
- **I-6** Ein Ski mit Grundstatus `wartung` hat keine aktive Ausleihe und keine Reservierung.

Daraus folgt: Ski und Personen mit Buchungen (auch Reservierungen) werden
**nicht gelöscht**, damit die Historie vollständig bleibt (siehe F-2.5, F-3.5).

---

## 4. Funktionale Anforderungen

Notation der Akzeptanzkriterien: **Gegeben** (Ausgangslage), **Wenn** (Aktion),
**Dann** (erwartetes Ergebnis).

### F-1 Dashboard (`GET /`)

**F-1.1** Das Dashboard zeigt fünf Kennzahlen: verfügbare Ski (jetzt), ausgeliehene Ski
(aktive Ausleihen), Reservierungen (zukünftig), Ski gesamt, registrierte Personen.
- Ski in Wartung zählen nur zu „gesamt“. Die Seite zeigt sie zusätzlich als Hinweis an („davon x in Wartung“).
- Ein Ski, der jetzt frei ist und erst später reserviert ist, zählt als **verfügbar** (AK-15).
- Überfällige Ausleihen werden bei „ausgeliehen“ als Hinweis genannt („davon x überfällig“).

**F-1.2** Schnellzugriffe: „Ski ausleihen / reservieren“, „Ski zurückgeben“ (zu aktiven Ausleihen),
„Neuer Ski“, „Neue Person“.

**F-1.3** Zwei getrennte Listen:
- **Aktive Ausleihen** (max. 5, neuester Beginn zuerst) mit geplanter Rückgabe, Markierung „überfällig“ und Button „Zurückgeben“.
- **Anstehende Reservierungen** (max. 5, nächster Beginn zuerst) mit Zeitraum, Preis und Button „Stornieren“.

**F-1.4** Gibt es Konflikte (3.6a), steht über den Kennzahlen eine rote Hinweiskarte mit den betroffenen Buchungen und dem Button „Stornieren“.
Die Kachel „Reservierungen“ zählt nur Buchungen im Zustand `reserviert`.

> AK-1: **Gegeben** 5 Ski (2 aktiv ausgeliehen, 1 in Wartung) und 3 Personen,
> **wenn** das Dashboard aufgerufen wird, **dann** zeigt es
> verfügbar = 2, ausgeliehen = 2, gesamt = 5, Personen = 3, Hinweis „davon 1 in Wartung“.

> AK-2: **Gegeben** ein leerer Bestand, **dann** zeigt das Dashboard überall 0, statt der Ausleihliste
> den Text „Keine aktiven Ausleihen“ und statt der Reservierungsliste „Keine anstehenden Reservierungen“.

### F-2 Ski verwalten

**F-2.1 Liste** (`GET /ski`): Tabelle mit ID, Marke, Modell, Länge, Typ, Zustand und angezeigtem Status.
Darunter steht gegebenenfalls „reserviert ab …“. Filter per Statusauswahl (`alle`, `verfuegbar`, `ausgeliehen`,
`wartung`) über den GET-Parameter `?status=`, gefiltert wird nach dem angezeigten Status. Bei Ski, die nicht in Wartung
sind, gibt es einen Button „Buchen“.

**F-2.2 Anlegen** (`GET /ski/neu`, `POST /ski`): Formular mit allen Pflichtfeldern aus 3.1.
Ist das Formular gültig, wird der Ski angelegt und die Seite leitet zur Detailseite weiter.
Ist es ungültig, erscheint das Formular erneut mit den eingegebenen Werten und einer
Fehlermeldung pro Feld.

**F-2.3 Detail** (`GET /ski/{id}`): Alle Felder des Ski, die aktuelle Ausleihe (falls vorhanden),
anstehende Reservierungen und die Historie der abgeschlossenen Ausleihen mit Endpreis.

**F-2.4 Bearbeiten** (`GET /ski/{id}/bearbeiten`, `POST /ski/{id}`): Ändert Stammdaten.
Der Status ist nur zwischen `verfuegbar` und `wartung` wählbar. Bei einem aktiv ausgeliehenen oder
reservierten Ski ist das Statusfeld gesperrt.

**F-2.5 Löschen** (`POST /ski/{id}/loeschen`): Nur erlaubt, wenn der Ski **keine** Buchungen hat.
Sonst kommt die Meldung „Ski hat Ausleih-Historie und kann nicht gelöscht werden.
Setze ihn stattdessen auf Wartung.“

> AK-3: **Wenn** ein Ski mit Länge 300 angelegt wird, **dann** erscheint
> „Länge muss zwischen 70 und 220 cm liegen“ und es wird kein Ski angelegt.

> AK-4: **Gegeben** ein ausgeliehener Ski, **wenn** versucht wird, ihn auf `wartung` zu setzen
> (z. B. per manipuliertem POST), **dann** lehnt der Server mit einer Fehlermeldung ab und der Status bleibt `ausgeliehen`.

### F-3 Personen verwalten

**F-3.1 Liste** (`GET /personen`): Tabelle mit Name, Ausweistyp, Ausweisnummer (maskiert, z. B. `****1234`),
Anzahl aktiver Ausleihen und Anzahl Reservierungen. Suche über GET-Parameter `?q=`, die Vorname, Nachname und
Ausweisnummer durchsucht (Groß- und Kleinschreibung egal, Teiltreffer zählen).

**F-3.2 Anlegen** (`GET /personen/neu`, `POST /personen`): Formular als `multipart/form-data`
mit den Feldern aus 3.2, inklusive optionalem Datei-Upload.

**F-3.3 Detail** (`GET /personen/{id}`): Stammdaten, vollständige Ausweisnummer, Vorschau des
Ausweisbildes (falls vorhanden), aktive Ausleihen mit „Zurückgeben“, Reservierungen mit „Stornieren“,
Historie, Button „Ski an diese Person ausleihen“.

**F-3.4 Bearbeiten** (`GET /personen/{id}/bearbeiten`, `POST /personen/{id}`): wie Anlegen. Zusätzlich
gibt es die Checkbox „Ausweisbild entfernen“.

**F-3.5 Löschen** (`POST /personen/{id}/loeschen`): Nur erlaubt, wenn die Person **keine** Buchungen hat.

> AK-5: **Gegeben** eine Person mit Reisepass `C01X00T47`, **wenn** eine zweite Person mit
> Reisepass `c01x00t47` angelegt wird, **dann** erscheint „Diese Ausweisnummer ist bereits registriert“.

> AK-6: **Wenn** als Ausweisbild eine PDF-Datei oder eine Datei über 2 MB hochgeladen wird,
> **dann** wird die Person nicht gespeichert und es erscheint
> „Ausweisbild muss JPEG oder PNG und höchstens 2 MB groß sein“.

> AK-7: **Wenn** eine Person ohne Bild angelegt wird, **dann** ist das gültig und die Detailseite zeigt
> „Kein Ausweisbild hinterlegt“.

### F-4 Buchen (Ausleihen / Reservieren), Zurückgeben, Stornieren – **erweitert in 1b**

**F-4.1 Buchungsformular** (`GET /ausleihen/neu`):
- Auswahl Ski: alle Ski, die **nicht in Wartung** sind, mit ihrem aktuellen Status im Text der Option.
  Ein gerade ausgeliehener Ski kann also für später reserviert werden.
- Auswahl Person: alle Personen.
- **Beginn:** Datum (`<input type="date">`) und Uhrzeit (`<input type="time">`). Vorbelegt mit „jetzt“, auf die Minute abgerundet.
- **Geplante Rückgabe:** Datum und Uhrzeit. Vorbelegt mit 17:00 am selben Tag, oder 17:00 am Folgetag, wenn der Beginn nach 16:00 liegt.
- Die Preisregel aus 3.7 steht als Hinweis im Formular.
- Die optionalen Parameter `?ski=` und `?person=` belegen die Auswahl vor.

**F-4.2 Preisvorschau** (`POST /ausleihen`, Button „Preis berechnen“, Feld `aktion=vorschau`):
Der Server prüft alle Regeln aus 3.3 und 3.6 und **speichert nichts**. Danach erscheint das Formular erneut,
jetzt mit einer Zusammenfassung:
- Ski und Person
- Zeitraum
- Art: „Ausleihe ab sofort“ oder „Reservierung“
- Verleihdauer (z. B. „2 T 7 Std“)
- Rechnung: „3 angefangene Kalendertage × 20,00 €“
- **Preis**

Außerdem erscheint der Button „Verbindlich buchen“ (AK-13). Ist eine Regel verletzt, erscheint das Formular mit
Feldfehlern bzw. Meldung (422).

**F-4.3 Bestätigen** (`POST /ausleihen`, Button „Verbindlich buchen“, Feld `aktion=bestaetigen`):
Die Vorschau schickt in einem versteckten Feld die geprüften Angaben mit (Ski, Person, Beginn, Ende, Preis).
Der Server berechnet alles neu. Stimmt etwas nicht mehr mit der Vorschau überein, wird **nicht gebucht**. Stattdessen
erscheint die neue Vorschau mit dem Hinweis „Die Angaben haben sich geändert – bitte neuen Preis prüfen und erneut bestätigen.“ (AK-14).
Sonst wird die Buchung angelegt. Danach Weiterleitung mit Erfolgsmeldung:
- Beginn ≤ jetzt: zur Liste der aktiven Ausleihen, Meldung „Ski wurde ausgeliehen.“
- Beginn in der Zukunft: zur Liste der Reservierungen, Meldung „Reservierung wurde angelegt.“

So ist sichergestellt, dass niemand eine Buchung bestätigt, ohne den aktuellen Preis gesehen zu haben.

**F-4.4 Zurückgeben** (`POST /ausleihen/{id}/rueckgabe`): Nur für aktive Ausleihen. Setzt die tatsächliche
Rückgabe auf „jetzt“ und berechnet den Endpreis (3.7). Danach Weiterleitung zur Herkunftsseite mit der Meldung
„Ski wurde zurückgegeben und ist wieder verfügbar.“ Die Herkunftsseite steht im versteckten Feld `zurueck`.
Erlaubt sind nur lokale Pfade (beginnen mit `/`, aber nicht mit `//`), alles andere führt zu `/ausleihen`.

**F-4.5 Stornieren** (`POST /ausleihen/{id}/stornieren`): Nur für Reservierungen (Beginn in der Zukunft).
Löscht die Reservierung. Rücksprung wie in F-4.4, Meldung „Reservierung wurde storniert.“

**F-4.6** Eine Person darf **mehrere** Ski gleichzeitig ausleihen oder reservieren (z. B. für eine Familie).

**F-4.7** Ein Ski darf **niemals in überlappenden Zeiträumen** gebucht sein (I-2). Das gilt auch bei manipulierten
Anfragen, beim erneuten Absenden und bei parallelen Anfragen. Der Server prüft das unabhängig davon, was die Oberfläche anzeigt.

> AK-8: **Gegeben** ein verfügbarer Ski S und eine Person P, **wenn** S ab jetzt bis morgen an P gebucht wird,
> **dann** ist S `ausgeliehen`, es gibt eine aktive Ausleihe (S, P), und auf dem Dashboard
> ist „verfügbar“ um 1 gesunken und „ausgeliehen“ um 1 gestiegen.

> AK-9: **Gegeben** ein Ski in `wartung`, **wenn** er gebucht werden soll, **dann** wird das mit
> „Ski ist in Wartung und kann nicht gebucht werden.“ abgelehnt. **Gegeben** ein aktiv ausgeliehener Ski,
> **wenn** er für einen überlappenden Zeitraum gebucht werden soll, **dann** wird das mit
> „Ski ist im gewählten Zeitraum bereits ausgeliehen oder reserviert.“ abgelehnt. In beiden Fällen entsteht keine Buchung.

> AK-10: **Gegeben** eine bereits abgeschlossene Ausleihe, **wenn** sie erneut zurückgegeben wird
> (z. B. durch Doppelklick oder „Zurück“ im Browser), **dann** erscheint „Ausleihe wurde bereits zurückgegeben.“
> und es ändert sich nichts.

> AK-11: **Wenn** zwei Anfragen gleichzeitig denselben Ski für denselben Zeitraum buchen, **dann** ist genau
> eine erfolgreich (I-2). Ein Test mit parallelen Goroutinen prüft das.

> AK-12 (Preis): Die Beispiele aus der Tabelle in 3.7 ergeben genau die angegebenen Preise.
> Außerdem gilt: 61 Tage werden als Fehler abgelehnt, 60 Tage sind erlaubt.

> AK-13 (Vorschau): **Wenn** „Preis berechnen“ gedrückt wird, **dann** zeigt die Seite Zeitraum, Dauer,
> Kalendertage und Preis sowie „Verbindlich buchen“. Es gibt danach **keine** neue Buchung.

> AK-14 (geänderte Angaben): **Gegeben** eine Vorschau für 1 Tag (20,00 €), **wenn** die Rückgabe auf 3 Tage
> geändert und direkt „Verbindlich buchen“ gedrückt wird, **dann** entsteht keine Buchung und die neue Vorschau
> zeigt 60,00 € mit dem Hinweis aus F-4.3.

> AK-15 (Reservierung): **Gegeben** ein verfügbarer Ski S, **wenn** S für übermorgen reserviert wird,
> **dann** bleibt S `verfuegbar`, „ausgeliehen“ auf dem Dashboard ändert sich nicht und „Reservierungen“ steigt um 1.
> **Wenn** die Uhr den Beginn erreicht, **dann** ist S `ausgeliehen` und die Buchung `aktiv`, ohne weiteren Speichervorgang.

> AK-16 (Überschneidung): **Gegeben** Ski S ist für `[10.11. 10:00, 12.11. 10:00)` gebucht. Dann wird
> `[11.11. 10:00, 13.11. 10:00)` (teilweise) abgelehnt, ebenso `[09.11. 10:00, 15.11. 10:00)` (umschließend),
> `[10.11. 12:00, 11.11. 12:00)` (enthalten) und derselbe Zeitraum. Erlaubt sind `[12.11. 10:00, 14.11. 10:00)`
> (angrenzend danach), `[08.11. 10:00, 10.11. 10:00)` (angrenzend davor) und jeder Zeitraum für einen anderen Ski.

> AK-17 (Rückgabe und Endpreis): **Gegeben** eine aktive Ausleihe von 3 Kalendertagen (60,00 €). **Wenn** sie
> am ersten Tag zurückgegeben wird, **dann** bleibt der Endpreis 60,00 € und der Ski ist ab der Rückgabe wieder
> buchbar. **Wenn** sie einen Kalendertag zu spät zurückgegeben wird, **dann** beträgt der Endpreis 80,00 €.
> Eine überfällige Ausleihe blockiert neue Buchungen des Ski, bis er zurück ist.

> AK-18 (Stornieren): **Gegeben** eine Reservierung, **wenn** sie storniert wird, **dann** ist sie gelöscht und
> der Zeitraum wieder frei. **Wenn** eine Reservierung zurückgegeben werden soll, **dann** erscheint
> „Die Reservierung hat noch nicht begonnen. Storniere sie stattdessen.“ **Wenn** eine aktive Ausleihe storniert
> werden soll, **dann** erscheint „Nur Reservierungen, die noch nicht begonnen haben, können storniert werden.“

> AK-19 (Eingaben): Rückgabe gleich oder vor dem Beginn → „Rückgabe muss nach dem Beginn liegen“.
> Beginn mehr als 15 Minuten in der Vergangenheit → „Beginn darf nicht in der Vergangenheit liegen“.
> Fehlendes oder ungültiges Datum → „Bitte Datum und Uhrzeit angeben“. Mehr als 60 Tage → „Verleihdauer darf höchstens 60 Tage betragen“.

> AK-20 (parallele Reservierungen): **Wenn** viele Anfragen gleichzeitig überlappende Zeiträume für denselben Ski
> buchen, **dann** ist genau eine erfolgreich.

> AK-21 (Wartung): **Gegeben** ein Ski mit einer Reservierung, **wenn** er auf `wartung` gesetzt werden soll,
> **dann** wird das mit „Ski ist ausgeliehen oder reserviert – Status kann nicht geändert werden.“ abgelehnt.

> AK-22 (verspätete Rückgabe): **Gegeben** Ski S mit Ausleihe A `[10:00, 12:00)` und Reservierung B `[13:00, 15:00)`.
> Um 12:30 ist A überfällig und B `reserviert` mit Konflikt; das Dashboard zeigt die Konfliktkarte.
> Um 13:30 ist B `wartend`, S hat genau eine aktive Ausleihe (A), B kann nicht zurückgegeben, aber storniert werden.
> Wird A zurückgegeben, ist B ohne Konflikt `aktiv`.

> AK-23 (Bedienung): Ausgehend vom Dashboard mit Demo-Daten führt jeder interne Link zu einer Seite mit Status 200.
> Jedes POST-Formular (Button) antwortet mit 200, 303, 409, 413 oder 422, nie mit 404, 405 oder 500. Ohne Demo-Daten gilt dasselbe.

### F-5 Übersicht der Buchungen (`GET /ausleihen`)

**F-5.1** Tabelle mit ID, Ski, Person, Zeitraum (Beginn – geplante Rückgabe), tatsächliche Rückgabe, Dauer,
Preis und Status-Badge (`Reserviert`, `Wartet auf Rückgabe`, `Aktiv`, `Überfällig`, `Zurückgegeben`), bei Konflikt zusätzlich `Konflikt`.
- Die Dauer ist bei reservierten und aktiven Buchungen die **geplante** Dauer, bei abgeschlossenen die **tatsächliche**.
- Der Preis ist bei abgeschlossenen Buchungen der Endpreis, sonst der gebuchte Preis.
- Die Dauer wird angezeigt als „2 T 3 Std“ (ab 1 Tag), „3 Std 30 Min“ bzw. „3 Std“ (unter 1 Tag) oder „45 Min“ (unter 1 Stunde).

**F-5.2** Filter `?status=aktiv|reserviert|abgeschlossen|alle` (Standard: `aktiv`). `reserviert` enthält auch wartende Buchungen.

**F-5.3** Sortierung: neuester Beginn zuerst. Ausnahme: Reservierungen stehen mit dem nächsten Beginn zuerst.

**F-5.4** Aktive Buchungen haben den Button „Zurückgeben“, Reservierungen und wartende Buchungen den Button „Stornieren“.

### F-6 Bedienoberfläche

**F-6.1** Auf jeder Seite gibt es eine Navigationsleiste: Dashboard · Ski · Personen · Ausleihen ·
Button „+ Ausleihe“. Der aktuelle Bereich ist hervorgehoben.

**F-6.2** Erfolgs- und Fehlermeldungen nach Aktionen erscheinen als Hinweisbox oben im Inhalt
(Post/Redirect/Get, die Meldung wird per Query-Parameter `?msg=` übergeben und nur aus
einer festen Liste bekannter Meldungen angezeigt, damit kein beliebiger Text eingeschleust werden kann).
Die bekannten Schlüssel sind `ski_created`, `ski_updated`, `ski_deleted`, `person_created`,
`person_updated`, `person_deleted`, `rented`, `reserved`, `returned` und `cancelled`. Unbekannte Schlüssel werden ignoriert.

**F-6.3** Löschbuttons sind farblich als gefährlich markiert. Gelöscht wird erst nach einer
Bestätigungsseite (`GET /ski/{id}/loeschen` zeigt „Wirklich löschen?“), ganz **ohne JavaScript**.
Hat der Eintrag Buchungen, zeigt schon die Bestätigungsseite den Hinweis aus F-2.5
bzw. F-3.5 statt eines Lösch-Buttons. Der Server prüft die Regel beim POST trotzdem noch einmal und antwortet dann mit `409`.

**F-6.4** Leere Listen zeigen einen erklärenden Text und einen passenden Button
(z. B. „Noch keine Ski erfasst – Ersten Ski anlegen“).

**F-6.5** Responsives Layout: Ab 360 px Breite gut bedienbar, Tabellen sind auf schmalen
Bildschirmen horizontal scrollbar.

**F-6.6** Alle Texte der Oberfläche sind auf Deutsch. Datumsformat `02.11.2026 14:05`, Geld `1.240,00 €`.

**F-6.7 Gestaltung:** modern und übersichtlich im Stil eines Admin-Dashboards
(angelehnt an das Dashboard-Beispiel aus der Vorlesung):
- feste Seitenleiste links mit Logo, Navigation und aktivem Menüpunkt. Auf schmalen Bildschirmen wandert
  sie als Leiste nach oben.
- Kopfzeile mit Seitentitel und primärer Aktion (z. B. „+ Neue Ausleihe“)
- Kennzahlen als **Kacheln** (Karten mit Symbol, großer Zahl, Beschriftung), farblich nach Bedeutung:
  grün = verfügbar, orange = ausgeliehen, türkis = reserviert, blau = gesamt, violett = Personen
- Inhalte in weißen Karten mit abgerundeten Ecken und dezentem Schatten auf hellgrauem Hintergrund
- Status als farbige **Badges** (Pillen) in Tabellen. Überfällige Ausleihen sind rot.
- Die Preisvorschau ist eine hervorgehobene Zusammenfassungskarte mit großem Preis.
- eine einheitliche Farbpalette über CSS-Variablen, Systemschrift, Symbole als Inline-SVG
  (keine Icon- oder Font-Bibliothek, NF-1)
- gut lesbare Kontraste (WCAG AA) und sichtbarer Fokusrahmen für die Bedienung per Tastatur

---

## 5. Nicht-funktionale Anforderungen

| ID    | Anforderung                                                                                         |
|-------|-----------------------------------------------------------------------------------------------------|
| NF-1  | Go ≥ 1.22, **nur Standardbibliothek** (`net/http`, `html/template`, `embed`, `sync`, `time`, …)      |
| NF-2  | Kein JavaScript notwendig. Alle Funktionen laufen mit reinen HTML-Formularen, auch die Preisvorschau. |
| NF-3  | Daten liegen nur im Speicher (Maps). Zugriff ist threadsicher (`sync.RWMutex`).                     |
| NF-4  | Ausgaben werden HTML-escaped (`html/template`). Upload-Größe ist serverseitig begrenzt (`http.MaxBytesReader`). |
| NF-5  | Jede Validierung findet **serverseitig** statt. HTML-Attribute wie `required` dienen nur dem Komfort. |
| NF-6  | Start mit einem Befehl: `go run .`. Port über Flag `-addr` (Standard `:8080`).                       |
| NF-7  | Optionales Flag `-demo` lädt Beispieldaten (8 Ski, 4 Personen, 4 Buchungen: 2 aktiv, davon 1 überfällig, 1 Reservierung, 1 abgeschlossen). |
| NF-8  | Tests mit `go test ./...` und `go test -race ./...` für Domain, Store, Service und die wichtigsten Handler (`httptest`). |
| NF-9  | Templates und CSS werden per `embed` in die Binary eingebettet, damit die Anwendung aus einer einzigen Datei läuft. |
| NF-10 | Logging jeder Anfrage (Methode, Pfad, Status, Dauer) auf stdout über eine einfache Middleware.      |
| NF-11 | In Demo-Daten, Tests und Dokumentation stehen **nur fiktive Daten** (erfundene Namen, Ausweisnummern mit Präfix `TEST`, Domain `example.com`). Echte Ausweisbilder werden nicht mitgeliefert. Auch Ausweisbilder im Arbeitsspeicher sind für den Prototyp nur mit Testbildern gedacht. |
| NF-12 | Alle zeitabhängigen Regeln (Zustand, Belegung, Preis) bekommen die Uhrzeit von außen übergeben, damit Tests mit einer festen Uhr laufen. Kalendertage beziehen sich auf die Zeitzone des Servers (`time.Local`). |

---

## 6. Routenübersicht

Ausschließlich GET und POST, weil HTML-Formulare nichts anderes senden können.
Routen nutzen die Muster des Go-1.22-`ServeMux` (`GET /ski/{id}`).

| Methode | Pfad                              | Zweck                                   |
|---------|-----------------------------------|-----------------------------------------|
| GET     | `/`                               | Dashboard                               |
| GET     | `/ski`                            | Ski-Liste (`?status=`)                  |
| GET     | `/ski/neu`                        | Formular neuer Ski                      |
| POST    | `/ski`                            | Ski anlegen                             |
| GET     | `/ski/{id}`                       | Ski-Detail                              |
| GET     | `/ski/{id}/bearbeiten`            | Formular Ski bearbeiten                 |
| POST    | `/ski/{id}`                       | Ski speichern                           |
| GET     | `/ski/{id}/loeschen`              | Lösch-Bestätigung                       |
| POST    | `/ski/{id}/loeschen`              | Ski löschen                             |
| GET     | `/personen`                       | Personen-Liste (`?q=`)                  |
| GET     | `/personen/neu`                   | Formular neue Person                    |
| POST    | `/personen`                       | Person anlegen (multipart)              |
| GET     | `/personen/{id}`                  | Personen-Detail                         |
| GET     | `/personen/{id}/bearbeiten`       | Formular Person bearbeiten              |
| POST    | `/personen/{id}`                  | Person speichern (multipart)            |
| GET     | `/personen/{id}/loeschen`         | Lösch-Bestätigung                       |
| POST    | `/personen/{id}/loeschen`         | Person löschen                          |
| GET     | `/personen/{id}/ausweisbild`      | Ausweisbild ausliefern                  |
| GET     | `/ausleihen`                      | Buchungsübersicht (`?status=`)          |
| GET     | `/ausleihen/neu`                  | Buchungsformular (`?ski=`, `?person=`)  |
| POST    | `/ausleihen`                      | Vorschau (`aktion=vorschau`) oder Buchung (`aktion=bestaetigen`) |
| POST    | `/ausleihen/{id}/rueckgabe`       | Rückgabe                                |
| POST    | `/ausleihen/{id}/stornieren`      | Reservierung stornieren                 |
| GET     | `/static/…`                       | CSS                                     |

HTTP-Statuscodes: `200` für Seiten (auch die erfolgreiche Preisvorschau), `303 See Other` nach erfolgreichem POST,
`422` beim erneuten Anzeigen eines ungültigen Formulars, `404` für unbekannte IDs
(eigene Fehlerseite), `409` für Konflikte, die nicht aus einem Formular kommen
(z. B. eine Rückgabe, die schon erledigt ist), `413` für zu große Uploads.

---

## 7. Fehlermeldungen (Domain-Fehler)

Der Domain-Layer definiert benannte Fehler, die der Handler in deutsche Texte übersetzt:

| Fehler                      | Text in der Oberfläche                                              |
|-----------------------------|---------------------------------------------------------------------|
| `ErrNotFound`               | „Eintrag wurde nicht gefunden.“                                     |
| `ErrSkiInMaintenance`       | „Ski ist in Wartung und kann nicht gebucht werden.“                 |
| `ErrSkiBooked`              | „Ski ist im gewählten Zeitraum bereits ausgeliehen oder reserviert.“ |
| `ErrAlreadyReturned`        | „Ausleihe wurde bereits zurückgegeben.“                             |
| `ErrNotStarted`             | „Die Reservierung hat noch nicht begonnen. Storniere sie stattdessen.“ |
| `ErrNotCancellable`         | „Nur Reservierungen, die noch nicht begonnen haben, können storniert werden.“ |
| `ErrSkiInUse`               | „Ski ist ausgeliehen oder reserviert – Status kann nicht geändert werden.“ |
| `ErrHasHistory`             | „Eintrag hat Ausleih-Historie und kann nicht gelöscht werden.“      |
| doppelte Ausweisnummer      | Feldfehler `id_number`: „Diese Ausweisnummer ist bereits registriert“ (kein eigener Fehlerwert, weil die Meldung direkt am Feld erscheint) |
| `ValidationErrors` (Map)    | Feldbezogene Meldungen direkt am Formularfeld                       |

---

## 8. Architektur

### 8.1 Schichten

```
Browser (HTML-Formulare, CSS)
        │  HTTP GET/POST
        ▼
handler   – HTTP: Parsen (inkl. Datum/Uhrzeit), Templates rendern, Redirects
        │
        ▼
service   – Anwendungsfälle: Vorschau, Buchen, Zurückgeben, Stornieren, Löschregeln, Kennzahlen
        │
        ▼
store     – In-Memory-Repository (Maps + RWMutex), kennt kein HTTP
        │
domain    – Structs, Enums, Zeiträume, Preisregel, Validierung, Fehler. Keine Abhängigkeiten.
```

Abhängigkeiten zeigen **nur nach unten**. `domain` importiert keine anderen Projektpakete.

### 8.2 Wichtige Entwurfsentscheidungen

| Entscheidung | Begründung |
|---|---|
| Eigene `service`-Schicht | Buchen prüft Ski, Person und alle anderen Buchungen und muss atomar laufen. |
| Store als Interface (`SkiRepository`, …) im `service`-Paket definiert, `MemoryStore` implementiert sie | Tests können den Store austauschen. Eine spätere DB-Anbindung bleibt möglich, ohne den Service zu ändern. |
| Ein `RWMutex` im Service: Schreib-Sperre für schreibende Anwendungsfälle, Lese-Sperre für Abfragen über mehrere Tabellen | Einfachste korrekte Lösung für I-2 bei parallelen Anfragen (AK-11, AK-20). Abfragen sehen nie einen halb fertigen Zustand. |
| Store gibt **Kopien** zurück | Kein Aufrufer kann die Daten im Store an den Mutex-Regeln vorbei verändern. |
| Uhr als Funktion `now func() time.Time` im Service | Zeitabhängige Tests sind deterministisch (NF-12). |
| **Status und Buchungszustand werden abgeleitet, nicht gespeichert** (1b) | Eine Reservierung wird zur Startzeit automatisch aktiv. Es gibt keinen Hintergrundjob und keinen Zustand, der veralten kann. |
| **Zeiträume als eigener Typ `Period`** mit `Overlaps` in der Domain (1b) | Die Kernregel ist eine reine Funktion und lässt sich mit Grenzfällen testen. |
| **Preisregel als Interface `PricingPolicy`** in der Domain, Implementierung `DailyRate` (1b) | Austauschbar ohne Änderung am Service. Vorbereitung für stundenweise Abrechnung in 1c. |
| **Vorschau und Bestätigung als zwei Formularschritte** mit Abgleich der geprüften Angaben (1b) | Der Preis ist vor der Bestätigung sichtbar, ganz ohne JavaScript (NF-2). Was gespeichert wird, berechnet immer der Server. |
| Bezeichner im Code englisch, UI-Texte deutsch | Übliche Go-Konvention. Die fachlichen Enum-Werte sind deutsch (`verfuegbar`), weil sie in URLs und Formularen auftauchen. |
| Keine Routing-/Template-Bibliothek | Go 1.22 `ServeMux` + `html/template` reichen aus (NF-1). |

### 8.3 Projektstruktur

```
ski-verleih/
├── go.mod                      module skiverleih, go 1.22
├── main.go                     Flags, Verdrahtung (Store, Preisregel, Service), Serverstart
├── CLAUDE.md                   Arbeitsregeln für KI-Assistenz
├── README.md                   Projektbeschreibung, Start, Bedienung
├── docs/
│   └── SPEZIFIKATION.md        diese Datei
├── internal/
│   ├── domain/
│   │   ├── ski.go              Ski, SkiTyp, Zustand, SkiStatus, StatusAt()
│   │   ├── person.go           Person, AusweisTyp, IDImage, Validate()
│   │   ├── rental.go           Rental, State(), Blocks(), Return(), FindConflict()
│   │   ├── period.go           Period, Overlaps(), ValidateBooking()
│   │   ├── pricing.go          Money, PricingPolicy, DailyRate, CalendarDays()
│   │   ├── errors.go           benannte Fehler, ValidationErrors
│   │   └── *_test.go
│   ├── store/                  MemoryStore + Tests
│   ├── service/
│   │   ├── repository.go       Repository-Interfaces
│   │   ├── service.go          Service, Sperren, Ansichten
│   │   ├── ski.go person.go    Verwaltung
│   │   ├── rental.go           Quote, Rent, Return, Cancel, Rentals
│   │   ├── dashboard.go        Kennzahlen
│   │   ├── demo.go             Beispieldaten (-demo)
│   │   └── *_test.go           inkl. Invarianten- und Nebenläufigkeitstests
│   └── handler/                Routen, Formulare, Meldungen, Tests (httptest)
└── web/
    ├── embed.go                //go:embed templates static
    ├── templates/              Layout, Icons, Seiten
    └── static/style.css
```

---

## 9. Erweiterbarkeit (Vorausschau, **nicht** Teil von 1a/1b)

### 9.1 Aufgabe 1b – umgesetzt
Die Vorausschau aus Version 0.x ist umgesetzt, siehe 3.3 bis 3.7. Eine Abweichung gegenüber dem damaligen Plan:
Die Preisregel bekommt nur den Zeitraum (`Price(p Period) Money`), nicht den Ski. Preise je Ski-Typ wären
eine spätere Erweiterung der Schnittstelle.

### 9.2 Aufgabe 1c – Fahrradverleih und Plugin-Architektur
- Eigenes Projekt. Was dort wiederverwendet wird, ist das **Muster**: Person, Buchung mit Zeitraum,
  Belegungsprüfung (`Period`, `Overlaps`), austauschbare `PricingPolicy`. Der Ski-spezifische Code wird nicht übernommen.
- Mögliche Plugin-Grenze: ein Interface `Rentable` (ID, Bezeichnung, Grundstatus) und eine
  `PricingPolicy` für stundenweise Abrechnung beim Fahrrad.
- Vorbereitung in 1a/1b, die nichts kostet: `Period`, `Money` und `PricingPolicy` enthalten **nichts**
  Ski-Spezifisches. `Rental` kennt den Ski nur über `SkiID`.
- Bewusst **nicht** in 1b: Generics, ein generisches „Item“, Plugin-Registry (YAGNI).

---

## 10. Umsetzungsplan

Jeder Schritt endet mit grünen Tests (`go test ./...`) und einem lauffähigen Stand.

**Aufgabe 1a** – abgeschlossen:
1. ✅ Gerüst · 2. ✅ Domain · 3. ✅ Store + Service · 4. ✅ Ski-Verwaltung · 5. ✅ Personen-Verwaltung ·
6. ✅ Ausleihen · 7. ✅ UI-Feinschliff · 8. ✅ Abnahme AK-1 bis AK-11

**Aufgabe 1b** – abgeschlossen:
9. ✅ **Spezifikation** v1.0 (diese Version).
10. ✅ **Domain:** `Period`, `Money`, `PricingPolicy`/`DailyRate`, Buchungszustände, Belegung, abgeleiteter Ski-Status, mit Tests.
11. ✅ **Service:** Quote, Rent, Return mit Endpreis, Cancel, Wartungsregel, Ansichten und Kennzahlen, mit Tests inkl. Nebenläufigkeit.
12. ✅ **Oberfläche:** Buchungsformular mit Vorschau und Bestätigung, Übersicht mit Reservierungen, Dashboard, Detailseiten.
13. ✅ **Abnahme:** AK-1 bis AK-21 automatisiert (Zuordnung in README.md), `go test ./...` und `go test -race ./...` laufen durch.
    Zusätzlich im Browser mit Demo-Daten geprüft, in Desktop-Breite und bei 375 px.

**Abschlussprüfung** – abgeschlossen:
14. ✅ Spezifikation v1.1, Korrektur 3.6a mit Tests (AK-22), Crawler über alle Links und Buttons (AK-23), Start ohne Demo-Daten und Neustart geprüft.

---

## 11. Annahmen und offene Fragen

**1a** – bestätigt (Stand 0.2):
1. Eine Person darf mehrere Ski gleichzeitig ausleihen (F-4.6).
2. Es gibt den Status „Wartung“.
3. Kein Löschen trotz Historie.
4. Ausweistypen: Personalausweis, Reisepass, Führerschein.
5. Die Ausweisnummer ist in Listen maskiert.

**1b** – Annahmen dieser Version (gelten, solange nichts anderes vorgegeben wird):

6. Der Beginn darf höchstens 15 Minuten in der Vergangenheit liegen, die Dauer höchstens 60 Tage betragen (3.3).
7. Bei vorzeitiger Rückgabe gibt es keine Erstattung. Bei verspäteter Rückgabe werden die zusätzlichen Kalendertage berechnet (3.7).
8. Eine Reservierung wird zur Startzeit automatisch aktiv. Eine eigene Ausgabe-Bestätigung gibt es nicht (3.5).
9. Eine überfällige Ausleihe blockiert den Ski für neue Buchungen, bis er zurück ist (3.6).
10. Stornierte Reservierungen werden gelöscht und nicht als Historie aufbewahrt (3.5).
11. Es gibt einen einheitlichen Preis für alle Ski-Typen (3.7).

**Abschlussprüfung (1.1):**

12. Eine wartende Buchung behält ihren gebuchten Preis. Ein Umbuchen auf einen anderen Ski gibt es nicht; stattdessen wird storniert und neu gebucht.
