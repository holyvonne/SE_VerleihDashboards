# Spezifikation – Fahrradverleih auf Basis eines Verleih-Frameworks (Aufgabe 1c)

| Feld        | Wert                                                        |
|-------------|-------------------------------------------------------------|
| Version     | 1.1                                                         |
| Stand       | 2026-11-02                                                  |
| Status      | **Freigegeben** – umgesetzt und abschließend geprüft        |
| Gilt für    | Aufgabe 1c (Verleih-Framework, Plugin-Architektur, Fahrradverleih mit Stundentarif) |

### Änderungshistorie

| Version | Änderung |
|---------|----------|
| 0.1 | Erster Entwurf: Framework, Plugin-Schnittstelle, Fahrrad-Plugin, Beispiel-Plugin, Weboberfläche |
| 1.0 | **Freigabe mit Festlegungen:** Mindestdauer 1 Stunde und Höchstdauer 30 Tage (4.3). Beginn nie in der Vergangenheit, mit Option „Sofort“ (4.4, F-5.1). Optionales Ausweisbild (4.1a). Konflikte durch verspätete Rückgaben und Umbuchen (4.5a, F-5.6). Stundensätze über die Plugin-Konfiguration (4.7). Ski-Plugin wird implementiert (4.8). Neu: AK-24 bis AK-26. Angepasst: AK-1, AK-13, AK-15. |
| 1.1 | **Abschlussprüfung:** Die Fahrrad-App nimmt nur Objekte der Verleihart `fahrrad` an (F-8.3). Weiterleitung nach dem Umbuchen abhängig vom Zustand (F-5.6). Das Größenlimit gilt auch mit `charset`-Zusatz (NF-4). Neu: AK-27 (alle Links und Buttons). Offene Punkte O-1 bis O-4 (Abschnitt 12). |

Diese Spezifikation ist die **einzige verbindliche Quelle** für Verhalten und
Struktur des Systems (Specification-Driven Development). Code, Tests und
Oberfläche werden aus ihr abgeleitet. Änderungen am Verhalten werden **zuerst
hier** eingetragen und erst danach umgesetzt.

Das Projekt ist **vollständig eigenständig**. Es teilt keinen Code mit `ski-verleih`
und importiert nichts daraus. Bewährte Lösungen aus Aufgabe 1a/1b (Zeiträume, Belegung,
Preisvorschau, Post/Redirect/Get) werden fachlich übernommen und hier neu und allgemein formuliert.

---

## 1. Ziel und Abgrenzung

### 1.1 Ziel
1. Ein **wiederverwendbares Verleih-Framework** mit der Geschäftslogik, die jeder Verleih braucht:
   - Personen und Verleihobjekte
   - Buchungen als Reservierung oder Ausleihe, mit freien Start- und Endzeitpunkten
   - Prüfung der Verfügbarkeit und Schutz vor Überschneidungen
   - Rückgabe und Stornierung
   - Preisberechnung über austauschbare Strategien
2. Eine **Plugin-Architektur**, über die Verleiharten (z. B. Fahrrad, Ski) registriert werden.
   Eine Verleihart legt fest, *was* verliehen wird und *wie* abgerechnet wird. Das Framework legt fest,
   *wie* verliehen wird.
3. Ein **Fahrradverleih** als Webanwendung, die das Framework mit dem Fahrrad-Plugin nutzt.

### 1.2 Im Umfang
- Framework-Pakete ohne fahrrad- oder ski-spezifische Regeln
- Interface-basierte Plugin-Registry (statisch beim Start, kein dynamisches Nachladen)
- Fahrrad-Plugin:
  - Fahrradtypen Citybike, Mountainbike, E-Bike
  - stundenweise Abrechnung mit konfigurierbarem Preis je Typ
  - Mindestabrechnung 1 Stunde
- Beispiel-Plugin „Ski (Tagestarif)“: Es weist nach, dass eine zweite Verleihart ohne Änderung am Framework funktioniert (ohne Weboberfläche).
- Weboberfläche:
  - Dashboard
  - Fahrräder und Personen verwalten (Personen optional mit Ausweisbild)
  - Verfügbarkeitssuche
  - Buchen mit Preisvorschau
  - Rückgabe, Stornierung, Umbuchen bei Konflikten
  - Buchungsübersicht und Tarifübersicht

### 1.3 Nicht im Umfang
- Dynamisch ladbare Plugins (`plugin`-Paket von Go, Shared Libraries, Skripte)
- Änderung von Tarifen zur Laufzeit über die Oberfläche (Tarife werden beim Start konfiguriert, siehe 4.7)
- Tagesdeckel, Rabatte, Kaution, Saisonpreise
- Nachträgliches Ändern von Zeitraum oder Person einer Buchung (stattdessen stornieren und neu buchen)
- Login, Rechte, dauerhafte Speicherung, Zahlungsabwicklung, Mehrsprachigkeit
- Jede Änderung am Projekt `ski-verleih`

---

## 2. Begriffe

| Begriff          | Bedeutung |
|------------------|-----------|
| Framework        | Die wiederverwendbaren Pakete unter `framework/`. Sie kennen keine konkrete Verleihart. |
| Verleihart       | Eine Art von Verleih, z. B. Fahrrad oder Ski. Im Code `plugin.Kind`, Schlüssel z. B. `fahrrad`. |
| Plugin           | Implementierung des Interfaces `plugin.Plugin` für genau eine Verleihart. |
| Registry         | Verzeichnis aller registrierten Plugins (`plugin.Registry`). |
| Verleihobjekt    | Ein einzelnes, physisch vorhandenes Objekt (z. B. ein Fahrrad). Im Code `domain.Item`. |
| Kategorie        | Untertyp innerhalb einer Verleihart, z. B. Citybike, Mountainbike, E-Bike. Bestimmt beim Fahrrad den Tarif. |
| Merkmal          | Zusätzliche Eigenschaft eines Objekts, die das Plugin festlegt (z. B. Rahmengröße). Im Code `domain.Field`. |
| Person           | Kundin oder Kunde |
| Buchung          | Ein Objekt für eine Person in einem Zeitraum. Im Code `domain.Booking`. In der Oberfläche heißt sie „Ausleihe“ oder „Reservierung“. |
| Zeitraum         | Halboffenes Intervall `[Beginn, Ende)`: Der Beginn gehört dazu, das Ende nicht. |
| Reservierung     | Buchung, deren Beginn in der Zukunft liegt |
| Aktive Ausleihe  | Buchung, deren Beginn erreicht ist, die ausgegeben und nicht zurückgegeben wurde |
| Überfällig       | Aktive Ausleihe, deren geplante Rückgabe vorbei ist |
| Belegung         | Zeitraum, in dem eine Buchung ihr Objekt blockiert (4.5) |
| Konflikt         | Eine noch nicht ausgegebene Buchung, deren Zeitraum mit der Belegung einer überfälligen Ausleihe desselben Objekts kollidiert (4.5a) |
| Umbuchen         | Eine noch nicht ausgegebene Buchung auf ein anderes, freies Objekt derselben Verleihart verlegen |
| Preisstrategie   | Implementierung von `plugin.PricingStrategy`. Sie berechnet Preis und Erklärung für ein Objekt und einen Zeitraum. |
| Angefangene Stunde | Jede begonnene Stunde der Dauer, gerechnet ab Beginn: 61 Minuten = 2 angefangene Stunden |

**Nutzerrolle:** nur *Mitarbeitende*. Sie dürfen alles.

---

## 3. Architektur: Framework und Plugins

### 3.1 Pakete und Abhängigkeiten

```
fahrrad-verleih/                 Go-Modul "fahrradverleih"
├── framework/                   ── wiederverwendbar, kein HTTP, keine Verleihart ──
│   ├── domain/                  Person, Item, Booking, Period, Money, Field, Fehler, Validierung
│   ├── plugin/                  Interfaces Plugin, PricingStrategy; Registry
│   ├── store/                   MemoryStore (In-Memory, threadsicher)
│   └── service/                 Anwendungsfälle: Objekte, Personen, Buchen, Rückgabe, Storno, Umbuchen, Statistik
├── plugins/
│   ├── fahrrad/                 Fahrrad-Plugin: Typen, Merkmale, Stundentarif
│   └── ski/                     Beispiel-Plugin: Ski mit Tagestarif (ohne Weboberfläche)
├── internal/demo/               Beispieldaten für -demo
├── internal/handler/            HTTP-Handler der Fahrrad-Webanwendung
├── web/                         Templates und CSS (embed)
└── main.go                      Verdrahtung: Registry + Fahrrad-Plugin + Service + Handler, Port :8081
```

Erlaubte Importe (Pfeil = „darf importieren“):

```
                 main.go
          ┌────────┼──────────────┬───────────────┐
          ▼        ▼              ▼               ▼
  internal/handler  internal/demo  plugins/fahrrad  (plugins/ski nur in Tests)
          │        │              │               │
          ▼        ▼              ▼               ▼
     framework/service ──► framework/plugin ──► framework/domain
          │                                        ▲
          └──────► framework/store ────────────────┘
```

- **A-R1** Pakete unter `framework/` importieren **nie** `plugins/…`, `internal/…` oder `web/`.
- **A-R2** Plugins importieren nur `framework/domain` und `framework/plugin`. Sie importieren nie `framework/service`, `framework/store` oder ein anderes Plugin.
- **A-R3** Im Framework-Code (ohne Tests) kommen keine Begriffe einer konkreten Verleihart vor
  (`fahrrad`, `bike`, `velo`, `ski`). Importpfade sind ausgenommen, weil der Modulname „fahrradverleih“ lautet.
  Ein automatisierter Test prüft A-R1 bis A-R3 (AK-22).
- **A-R4** Der Handler darf das Fahrrad-Plugin direkt importieren. Er ist Teil der Fahrrad-*Anwendung*,
  nicht des Frameworks, und braucht es z. B. für Texte wie „Fahrrad“ und für die Tarifseite.

### 3.2 Plugin-Schnittstelle

```go
package plugin

// Kind beschreibt eine Verleihart.
type Kind struct {
    Key      string // URL-tauglich, eindeutig, z. B. "fahrrad"
    Singular string // "Fahrrad"
    Plural   string // "Fahrräder"
}

// Plugin legt fest, was verliehen wird und wie abgerechnet wird.
type Plugin interface {
    Kind() Kind
    Categories() []domain.Category               // Kategorien, z. B. Citybike, Mountainbike, E-Bike
    Fields() []domain.Field                      // zusätzliche Merkmale eines Objekts
    Validate(item domain.Item) domain.ValidationErrors // fachliche Regeln über mehrere Merkmale
    DisplayName(item domain.Item) string         // Anzeigename, z. B. "Cube Kathmandu"
    Pricing() PricingStrategy
    Limits() domain.BookingLimits                // Mindest- und Höchstdauer
}

// PricingStrategy berechnet den Preis einer Buchung.
type PricingStrategy interface {
    Quote(item domain.Item, p domain.Period) domain.Price // Betrag + Erklärung, z. B. "3 angefangene Stunden × 8,00 €"
    Describe(category string) string                       // Regeltext, z. B. "8,00 € pro angefangene Stunde, mindestens 1 Stunde"
}
```

Wichtige Typen aus `framework/domain`:

| Typ             | Inhalt |
|-----------------|--------|
| `Category`      | `Key`, `Label` (z. B. `ebike`, „E-Bike“) |
| `Field`         | `Key`, `Label`, `Type` (`text`, `number`, `choice`), `Required`, `MaxLen`, `Min`, `Max`, `Options`, `Unit`, `Hint` |
| `Item`          | `ID`, `Kind`, `Category`, `Attributes map[string]string`, `Status` (`verfuegbar`/`wartung`), `Note` |
| `Price`         | `Amount Money`, `Explanation string` |
| `BookingLimits` | `MinDuration`, `MaxDuration` (0 = keine Grenze) |
| `Period`        | `Start`, `End`, `Overlaps`, `Duration` |
| `Money`         | Cent als `int64`, Anzeige `1.240,00 €` |

**Prüfung eines Objekts (F-2)** in drei Stufen:
1. Das Framework prüft die allgemeinen Regeln: bekannte Verleihart, gültige Kategorie, Notiz.
2. Es prüft jedes Merkmal gegen seine `Field`-Beschreibung: Pflicht, Länge, Zahlbereich, erlaubte Optionen.
3. Es ruft `Plugin.Validate` für Regeln auf, die mehrere Felder betreffen.

Alle Feldfehler werden zusammen angezeigt. Merkmale, die das Plugin nicht kennt, werden verworfen.

### 3.3 Registry

```go
reg := plugin.NewRegistry()
err := reg.Register(fahrradPlugin)
p, err := reg.Get("fahrrad")   // ErrUnknownKind, falls nicht registriert
all := reg.All()               // in Registrierungsreihenfolge
```

- `Register` lehnt ab:
  - `nil`
  - einen leeren oder nicht URL-tauglichen Schlüssel (erlaubt ist nur `[a-z0-9_-]+`, gilt auch für Kategorien)
  - einen bereits registrierten Schlüssel (`ErrDuplicateKind`)
  - ein Plugin ohne Kategorien oder ohne Preisstrategie
  - doppelte Kategorie- oder Merkmalsschlüssel
  - eine Auswahl ohne Optionen
- Die Registry ist threadsicher. Plugins werden beim Start registriert, das Framework liest danach nur noch.
- Der Service bekommt die Registry im Konstruktor: `service.New(repo, reg, now)`.
  Jede Operation an einem Objekt holt sich das Plugin über `item.Kind`.

### 3.4 Ablauf einer Buchung im Framework

```
Handler ──Quote(in)──► Service ──(Lesesperre)──────────────────────────────────────────────┐
                         │ 1 Objekt und Person laden            (ErrNotFound → Feldfehler)   │
                         │ 2 Plugin = registry.Get(item.Kind)    (ErrUnknownKind)            │
                         │ 3 Zeitraum prüfen gegen plugin.Limits()                           │
                         │ 4 Wartung?                            (ErrItemInMaintenance)      │
                         │ 5 Belegungen des Objekts prüfen       (ErrItemBooked)             │
                         │ 6 Preis = plugin.Pricing().Quote(item, zeitraum)                  │
                         └──► Quote{Objekt, Person, Zeitraum, Preis, Erklärung, Reservierung?}┘

Handler ──Book(in)───► Service ──(Schreibsperre)── Schritte 1–6 erneut ── Buchung speichern
```

Prüfen und Speichern passieren unter **einer** Schreibsperre. Dadurch können parallele Anfragen keine Überschneidung erzeugen (AK-12).

### 3.5 Erweiterbarkeit nachweisen

Eine neue Verleihart braucht **nur** zwei Dinge: ein neues Paket, das `plugin.Plugin` implementiert, und eine Zeile
`reg.Register(...)`. Nachweis durch drei Tests:

1. **Test-Plugin im Testcode** (`framework/service/plugin_test.go`): Eine Verleihart `testgeraet` mit
   Pauschalpreis wird nur im Test definiert und registriert. Anlegen, Buchen, Überschneidungsprüfung,
   Rückgabe und Storno laufen damit (AK-20).
2. **Beispiel-Plugin `plugins/ski`**: Tagestarif 20,00 € pro angefangenem Kalendertag (Regel aus Aufgabe 1b).
   Fahrrad und Ski werden im **selben** Service registriert. Gleicher Zeitraum, unterschiedliche Preise (AK-21).
3. **Architekturtest** prüft A-R1 bis A-R3 mit `go/parser` aus der Standardbibliothek (AK-22).

---

## 4. Domänenmodell

### 4.1 Person (Framework)

| Feld          | Typ        | Pflicht | Regeln |
|---------------|------------|---------|--------|
| ID            | int        | auto    | fortlaufend ab 1, wird nie wiederverwendet |
| Vorname       | string     | ja      | 1–50 Zeichen nach Trimmen |
| Nachname      | string     | ja      | 1–50 Zeichen nach Trimmen |
| Telefon       | string     | nein    | max. 30 Zeichen: Ziffern, Leerzeichen, `+ - / ( )` |
| E-Mail        | string     | nein    | max. 100 Zeichen, muss `@` enthalten |
| Ausweistyp    | AusweisTyp | ja      | `personalausweis`, `reisepass`, `fuehrerschein` |
| Ausweisnummer | string     | ja      | 4–30 Buchstaben/Ziffern, in Großbuchstaben gespeichert, zusammen mit dem Typ eindeutig |
| Ausweisbild   | Bild       | nein    | JPEG oder PNG, max. 2 MB, siehe 4.1a |
| Angelegt am   | Zeitpunkt  | auto    | |

In Listen wird die Ausweisnummer maskiert angezeigt (`****0001`), in der Detailansicht vollständig.

### 4.1a Ausweisbild (optional)
- Personen können **ohne** Bild angelegt werden. Ausweistyp und -nummer sind immer Pflicht.
- Das Bild liegt im Arbeitsspeicher (Bytes + MIME-Typ). Der Typ wird aus dem Inhalt bestimmt (`http.DetectContentType`). Erlaubt sind JPEG und PNG bis 2 MB.
- Abruf nur über `GET /personen/{id}/ausweisbild` mit `Cache-Control: no-store` und `X-Content-Type-Options: nosniff`.
- Beim Bearbeiten kann das Bild ersetzt oder entfernt werden. Ohne neue Auswahl bleibt es erhalten.
- Die Formular-Anfrage ist auf 3 MB begrenzt und wird komplett im Speicher verarbeitet. Bei größeren Anfragen antwortet der Server mit 413.
- Nur fiktive Testbilder verwenden (NF-11).

### 4.2 Verleihobjekt `Item` (Framework)

| Feld       | Typ                 | Pflicht | Regeln |
|------------|---------------------|---------|--------|
| ID         | int                 | auto    | fortlaufend ab 1 |
| Verleihart | string              | ja      | Schlüssel eines registrierten Plugins |
| Kategorie  | string              | ja      | einer der `Categories()` des Plugins |
| Merkmale   | map[string]string   | je nach Plugin | geprüft nach `Fields()` und `Validate()` |
| Grundstatus| `verfuegbar`/`wartung` | auto | neu angelegt: `verfuegbar` |
| Notiz      | string              | nein    | max. 200 Zeichen |

Der **angezeigte Status** wird abgeleitet und nie gespeichert (wie in 1b):
- `wartung`, wenn der Grundstatus `wartung` ist;
- sonst `ausgeliehen`, wenn das Objekt gerade ausgegeben ist (aktive Ausleihe);
- sonst `verfuegbar`.

Ein verfügbares Objekt mit anstehender Reservierung zeigt zusätzlich „reserviert ab …“.
Der Grundstatus darf nur gewechselt werden, solange das Objekt keine offene Buchung hat (aktiv, wartend oder reserviert).

### 4.3 Fahrrad (Fahrrad-Plugin)

Verleihart `fahrrad` („Fahrrad“ / „Fahrräder“).

**Kategorien (Fahrradtypen):** `citybike` „Citybike“, `mountainbike` „Mountainbike“, `ebike` „E-Bike“.

**Merkmale:**

| Schlüssel       | Bezeichnung     | Typ    | Pflicht | Regeln |
|-----------------|-----------------|--------|---------|--------|
| `marke`         | Marke           | text   | ja      | 1–50 Zeichen |
| `modell`        | Modell          | text   | ja      | 1–50 Zeichen |
| `rahmengroesse` | Rahmengröße     | choice | ja      | `S`, `M`, `L`, `XL` |
| `akku_wh`       | Akkukapazität   | number | s. u.   | 200–1000 Wh |

**Plugin-Regel über mehrere Felder (`Validate`):**
- E-Bike ohne Akkukapazität → „Für E-Bikes bitte die Akkukapazität angeben“
- Citybike oder Mountainbike mit Akkukapazität → „Akkukapazität nur bei E-Bikes angeben“

**Anzeigename:** „Marke Modell“, z. B. „Cube Kathmandu“.

**Grenzen (`Limits`):** Dauer mindestens **1 Stunde**, höchstens **30 Tage**. Die Werte stehen in `fahrrad.Config` und sind dort änderbar.

### 4.4 Buchung `Booking` (Framework)

| Feld                  | Typ         | Regeln |
|-----------------------|-------------|--------|
| ID                    | int         | fortlaufend ab 1 |
| ItemID, PersonID      | int         | Verweise auf existierende Einträge |
| Beginn                | Zeitpunkt   | Datum + Uhrzeit (minutengenau), jetzt oder in der Zukunft |
| Geplante Rückgabe     | Zeitpunkt   | nach dem Beginn |
| Gebucht am            | Zeitpunkt   | auto |
| Preis, Erklärung      | Money, Text | bei Buchung über die Preisstrategie berechnet |
| Tatsächliche Rückgabe | Zeitpunkt?  | leer = nicht zurückgegeben |
| Endpreis, Erklärung   | Money, Text | bei Rückgabe berechnet (4.6) |

Regeln für eine neue Buchung (AK-15), gültig für **jede** Verleihart:
- Beginn und Rückgabe sind gültige Zeitpunkte, die Rückgabe liegt nach dem Beginn.
- Der Beginn liegt **nicht in der Vergangenheit**. Verglichen wird minutengenau, weil die Formulare
  Minuten erfassen: Beginn ≥ aktuelle Minute (`jetzt` auf die Minute abgerundet).
- Die Dauer liegt zwischen `MinDuration` und `MaxDuration` aus dem Plugin (Fahrrad: 1 Stunde bis 30 Tage).
- Das Objekt ist nicht in Wartung, und der Zeitraum ist frei (4.5).
- Eine Person darf mehrere Objekte gleichzeitig buchen.

**Zustände** (abgeleitet aus der Uhrzeit `jetzt` und den übrigen Buchungen des Objekts):

| Zustand         | Bedingung                              | Erlaubte Aktion |
|-----------------|----------------------------------------|-----------------|
| `reserviert`    | keine Rückgabe und `jetzt < Beginn`    | Stornieren, Umbuchen |
| `wartend`       | keine Rückgabe und `Beginn ≤ jetzt`, aber das Objekt ist noch durch eine frühere, überfällige Ausleihe belegt (4.5a) | Stornieren, Umbuchen |
| `aktiv`         | keine Rückgabe, `Beginn ≤ jetzt`, nicht wartend | Zurückgeben |
| ↳ überfällig    | `aktiv` und `jetzt ≥ geplante Rückgabe`| Zurückgeben |
| `abgeschlossen` | tatsächliche Rückgabe gesetzt          | – |

Stornieren löscht die Buchung. Eine Reservierung kann nicht zurückgegeben werden, eine aktive Ausleihe nicht storniert.
Die Rückgabe speichert den tatsächlichen Rückgabezeitpunkt (`jetzt`) und den Endpreis (4.6).

### 4.5 Belegung und Überschneidungen (Framework)

| Buchung                                  | Belegung |
|------------------------------------------|----------|
| reserviert, wartend oder aktiv, nicht überfällig | `[Beginn, geplante Rückgabe)` |
| aktiv und überfällig                     | `[Beginn, ∞)`: blockiert, bis das Objekt zurück ist |
| abgeschlossen                            | `[Beginn, tatsächliche Rückgabe)`: Eine vorzeitige Rückgabe gibt die Restzeit frei. |

`[a, b)` und `[c, d)` überschneiden sich, wenn `a < d && c < b`. Angrenzende Zeiträume sind erlaubt.
Ein Zeitraum ist für ein Objekt **verfügbar**, wenn das Objekt nicht in Wartung ist und keine Belegung
den Zeitraum überschneidet. Dieselbe Funktion nutzen die Buchung (F-5), die Verfügbarkeitssuche (F-4) und das Umbuchen (F-5.6).

### 4.5a Verspätete Rückgabe und nachfolgende Reservierungen (Framework)

Beim Anlegen überschneiden sich Buchungen nie. Wird aber eine Ausleihe **nicht rechtzeitig zurückgegeben**,
wächst ihre Belegung auf `[Beginn, ∞)` und kann eine **nachfolgende** Buchung desselben Objekts treffen.
Das Framework erkennt diese Fälle und zeigt sie an:

- Eine nicht ausgegebene Buchung B hat einen **Konflikt**, wenn es eine andere, nicht zurückgegebene Buchung A
  desselben Objekts mit `A.Beginn < B.Beginn` gibt, deren aktuelle Belegung den geplanten Zeitraum von B überschneidet.
- Solange B noch nicht begonnen hat, bleibt sie `reserviert`, wird aber als **Konflikt** markiert
  („Fahrrad noch nicht zurück“).
- Ist der Beginn von B erreicht und das Objekt noch nicht zurück, ist B `wartend`, nicht `aktiv`.
  So gibt es nie zwei aktive Ausleihen für ein Objekt (I-2a).
- Wird A zurückgegeben, ist der Konflikt aufgelöst: B ist wieder normal `reserviert` bzw. sofort `aktiv`.
- Mitarbeitende können B **stornieren** oder **umbuchen** (F-5.6). Dashboard und Buchungsliste zeigen Konflikte deutlich an.
- Neue Buchungen sehen die Belegung `[Beginn, ∞)` der überfälligen Ausleihe und werden abgelehnt (AK-13).

### 4.6 Preisregel Fahrrad (Stundentarif)

> **Preis = angefangene Stunden × Stundensatz des Fahrradtyps, mindestens 1 Stunde.**

| Fahrradtyp   | Stundensatz (Standard) |
|--------------|------------------------|
| Citybike     | 5,00 €                 |
| Mountainbike | 8,00 €                 |
| E-Bike       | 12,00 €                |

- Angefangene Stunden = Dauer in Minuten geteilt durch 60, aufgerundet, mindestens 1. Beispiel: 2 Std 15 Min → 3 Stunden.
- Gerechnet wird die tatsächlich vergangene Zeit. In der Nacht der Zeitumstellung vergeht eine Stunde mehr oder weniger, und genau diese Zeit wird berechnet.
- Erklärung in der Vorschau: „3 angefangene Stunden × 8,00 €“.

| Typ          | Zeitraum                    | Angefangene Stunden | Preis    |
|--------------|-----------------------------|---------------------|----------|
| Citybike     | 10:00 – 11:00               | 1                   | 5,00 €   |
| Citybike     | 10:00 – 11:01               | 2                   | 10,00 €  |
| Citybike     | 10:00 – 12:15               | 3                   | 15,00 €  |
| Mountainbike | 09:00 – 12:30               | 4                   | 32,00 €  |
| E-Bike       | 09:00 – 17:00               | 8                   | 96,00 €  |
| E-Bike       | 02.11. 18:00 – 03.11. 10:00 | 16                  | 192,00 € |

Die Preisstrategie rechnet für jede Dauer mindestens 1 Stunde, z. B. 20 Minuten → 5,00 €. Bei neuen Buchungen
greift vorher schon die Mindestdauer von 1 Stunde (4.3). Die Mindestabrechnung bleibt trotzdem als Absicherung für jede Preisberechnung bestehen.

**Endpreis bei Rückgabe** (wie 1b): Preis für `[Beginn, max(geplante Rückgabe, tatsächliche Rückgabe))`.
- Bei vorzeitiger Rückgabe gilt der gebuchte Preis, es gibt keine Erstattung.
- Bei verspäteter Rückgabe werden die weiteren angefangenen Stunden berechnet. Beispiel: E-Bike 09:00–12:00 gebucht, zurück um 12:20 → 4 Stunden.

### 4.7 Konfiguration der Tarife

- Preise werden **über das Fahrrad-Plugin** konfiguriert. `fahrrad.Config` enthält die Stundensätze je Typ
  (`map[Kategorie]Money`) sowie die Mindest- und die Höchstdauer.
  - `fahrrad.DefaultConfig()` liefert die Werte aus 4.3 und 4.6.
  - `fahrrad.New(cfg)` prüft die Konfiguration: Jeder Typ hat einen Satz über 0 € und höchstens 100 €, und die Mindestdauer liegt unter der Höchstdauer.
- Beim Start lassen sich die Sätze mit dem Flag `-tarife` überschreiben, z. B.
  `go run . -tarife "citybike=6,mountainbike=9.50,ebike=14"`. Nicht genannte Typen behalten den Standardwert.
  Das Flag wird vom Plugin geparst (`fahrrad.ParseRates`).
- Ungültige Angaben beenden den Start mit einer verständlichen Meldung: unbekannter Typ, kein Betrag, Betrag ≤ 0 oder > 100 €.
- Während der Laufzeit ändern sich die Tarife nicht. Dadurch passen Vorschau, Buchung und Endpreis immer zusammen.

### 4.8 Beispiel-Plugin Ski (Tagestarif)

- Verleihart `ski`, Kategorien `allround`, `carving`, `freeride`.
- Merkmale: Marke, Modell, Länge (70–220 cm).
- Preisstrategie: 20,00 € pro angefangenem Kalendertag, mindestens 20,00 € (wie Aufgabe 1b). Kalendertage in einer konfigurierbaren Zeitzone.
- Grenzen: Höchstdauer 60 Tage, keine Mindestdauer.

Das Plugin ist vollständig implementiert und getestet, hat aber **keine eigene Weboberfläche**. In Tests wird es
zusammen mit dem Fahrrad-Plugin registriert. Die Webanwendung zeigt ausschließlich Fahrräder.

### 4.9 Invarianten
Nach jeder Operation gilt (Tests prüfen das):
- **I-1** Angezeigter Status `ausgeliehen` ⇔ das Objekt hat eine aktive Ausleihe.
- **I-2** Beim Anlegen oder Umbuchen überschneidet sich eine Buchung mit keiner Belegung desselben Objekts.
  Spätere Überschneidungen entstehen nur durch überfällige Ausleihen und sind dann als Konflikt markiert (4.5a).
- **I-2a** Jedes Objekt hat höchstens eine Buchung im Zustand `aktiv`.
- **I-3** Jede Buchung verweist auf ein existierendes Objekt und eine existierende Person.
- **I-4** Die geplante Rückgabe liegt nach dem Beginn, die tatsächliche Rückgabe nicht vor dem Beginn.
- **I-5** Ausweistyp + Ausweisnummer sind eindeutig.
- **I-6** Ein Objekt in `wartung` hat keine offene Buchung.
- **I-7** Jedes gespeicherte Objekt gehört zu einer registrierten Verleihart und einer gültigen Kategorie.

Objekte und Personen mit Buchungen werden nicht gelöscht.

---

## 5. Funktionale Anforderungen

Notation: **Gegeben** / **Wenn** / **Dann**. „Jetzt“ ist in den Tests eine feste Uhr.

### F-1 Dashboard (`GET /`)
- **F-1.1 Kacheln:**
  - Verfügbar
  - Ausgeliehen, mit Zusatz „davon x überfällig“
  - Reserviert (anstehende Reservierungen)
  - Gesamt
  - Personen
- **F-1.2 Bestand nach Typ:** Tabelle Citybike/Mountainbike/E-Bike mit Stundensatz, Gesamt, Verfügbar, Ausgeliehen und Wartung.
- **F-1.3 Listen:** aktive Ausleihen (überfällige zuerst) und die nächsten Reservierungen, je höchstens 5, mit Rückgabe- bzw. Storno-Button.
- **F-1.4 Schnellzugriffe:** „+ Buchung“, „Verfügbarkeit prüfen“, „Fahrrad anlegen“, „Person anlegen“.
- **F-1.5 Konflikte:** Gibt es Konflikte (4.5a), steht oben eine rote Hinweiskarte mit allen betroffenen Buchungen und den Buttons „Umbuchen“ und „Stornieren“.

> AK-1: **Gegeben** die Demo-Daten (NF-7), **dann** zeigt das Dashboard 6 verfügbar, 2 ausgeliehen (davon 1 überfällig),
> 2 reserviert, 9 gesamt, 4 Personen und 1 Konflikt. Nach jeder Buchung, Rückgabe und Stornierung stimmen die Zahlen mit den Daten überein.

### F-2 Fahrräder verwalten
- **F-2.1 Liste** (`GET /fahrraeder`): Name, Typ, Rahmengröße, Akku, Stundensatz, Status-Badge, „reserviert ab …“.
  - Filter `?status=verfuegbar|ausgeliehen|reserviert|wartung` und `?typ=citybike|mountainbike|ebike`, beide kombinierbar.
  - `reserviert` bedeutet: Fahrräder mit mindestens einer anstehenden Reservierung.
- **F-2.2 Anlegen/Bearbeiten** (`/fahrraeder/neu`, `/fahrraeder/{id}/bearbeiten`): Das Formular wird aus `Fields()` des Plugins erzeugt.
  Bei Fehlern erscheint es erneut mit Status 422, Eingaben und Feldfehlern.
- **F-2.3 Detail** (`GET /fahrraeder/{id}`): Merkmale, Status, aktuelle Ausleihe, Reservierungen, Historie, Button „Buchen“.
- **F-2.4 Status:** Wechsel zwischen verfügbar und Wartung im Bearbeitungsformular. Gesperrt, solange eine offene Buchung besteht.
- **F-2.5 Löschen** mit Bestätigungsseite. Hat das Fahrrad Buchungen, ist Löschen nicht möglich.

> AK-2: **Wenn** ein Mountainbike mit Marke, Modell und Rahmengröße angelegt wird, **dann** erscheint es als `verfuegbar`
> mit 8,00 €/Std. **Wenn** Pflichtfelder fehlen oder die Rahmengröße `XXL` ist, **dann** antwortet der Server mit 422 und Feldfehlern, und nichts wird gespeichert.

> AK-3: E-Bike ohne Akkukapazität und Citybike mit Akkukapazität werden mit den Meldungen aus 4.3 abgelehnt.
> Akkukapazität 150 → „Wert muss zwischen 200 und 1000 liegen“.

> AK-4: **Gegeben** ein Fahrrad mit Reservierung, **wenn** es auf Wartung gesetzt werden soll, **dann** erscheint
> „Fahrrad ist ausgeliehen oder reserviert – Status kann nicht geändert werden.“ **Wenn** ein Fahrrad mit Buchungen
> gelöscht werden soll, **dann** wird das abgelehnt (409). Ein Fahrrad ohne Buchungen wird gelöscht.

### F-3 Personen verwalten
Liste mit Suche (`?q=` in Name, Ausweisnummer, E-Mail), Anlegen mit optionalem Ausweisbild, Detail mit Buchungen, Bearbeiten, Löschen mit Bestätigung.

> AK-5: Eine gültige Person wird angelegt. Gleicher Ausweistyp und gleiche Nummer (auch in Kleinbuchstaben) ergeben den
> Feldfehler „Diese Ausweisnummer ist bereits registriert“. Eine Person mit Buchungen kann nicht gelöscht werden.

### F-4 Verfügbarkeit prüfen (`GET /verfuegbarkeit`)
- Eingabe: Beginn (Datum + Uhrzeit), Ende (Datum + Uhrzeit), optional Fahrradtyp. Vorbelegt mit nächster voller Stunde bis + 2 Stunden.
- Ergebnis: alle Fahrräder, die im Zeitraum verfügbar sind (4.5), gruppiert nach Typ. Jedes zeigt den Preis für den Zeitraum
  und hat den Button „Buchen“, der das Buchungsformular mit Fahrrad und Zeitraum vorbelegt.
- Ohne Treffer erscheint „Im gewählten Zeitraum ist kein passendes Fahrrad frei.“. Bei ungültigem Zeitraum erscheinen Feldfehler wie in AK-15.

> AK-6: **Gegeben** Fahrrad A ist von 10:00 bis 12:00 gebucht, B ist frei, C ist in Wartung. **Wenn** 11:00–13:00 gesucht wird,
> **dann** erscheint nur B. **Wenn** 12:00–13:00 gesucht wird, **dann** erscheinen A und B. Mit Filter `ebike` erscheinen nur E-Bikes.

### F-5 Buchen, Zurückgeben, Stornieren, Umbuchen
- **F-5.1 Formular** (`GET /buchungen/neu`, optional `?fahrrad=&person=&start=&ende=` im Format `2006-01-02T15:04`):
  Fahrrad (nicht in Wartung, Status im Text), Person, Beginn und geplante Rückgabe (Datum + Uhrzeit). Die Preisregel steht als Hinweis dabei.
  - Beginn: Auswahl **„Sofort“** (Standard) oder **„Später“** mit Datum und Uhrzeit. Bei „Sofort“ setzt der Server den Beginn
    bei jeder Anfrage auf die aktuelle Minute. So scheitert eine Ausleihe ab jetzt nicht daran, dass beim Ausfüllen Zeit vergeht.
  - Vorbelegt ist die Rückgabe mit der nächsten vollen Stunde + 2 Stunden. Kommen `start`/`ende` aus der Verfügbarkeitssuche, ist „Später“ gewählt.
- **F-5.2 Vorschau** (`POST /buchungen`, `aktion=vorschau`): prüft alles und speichert nichts. Angezeigt werden:
  - Fahrrad und Person
  - Zeitraum
  - Art (Ausleihe ab sofort / Reservierung)
  - Dauer und Erklärung
  - **Preis**
  - der Button „Verbindlich buchen“
- **F-5.3 Bestätigen** (`aktion=bestaetigen`): Der Server rechnet neu.
  - Weicht etwas von der Vorschau ab (Prüfwert im versteckten Feld), wird nicht gebucht. Stattdessen erscheint die neue Vorschau mit Hinweis.
  - Sonst wird gebucht und mit der Meldung „Fahrrad wurde ausgeliehen.“ bzw. „Reservierung wurde angelegt.“ weitergeleitet.
- **F-5.4 Zurückgeben** (`POST /buchungen/{id}/rueckgabe`) und **F-5.5 Stornieren** (`POST /buchungen/{id}/stornieren`)
  springen danach zur Herkunftsseite zurück (nur lokale Pfade).
- **F-5.6 Umbuchen** (`GET /buchungen/{id}/umbuchen`, `POST /buchungen/{id}/umbuchen`): für Buchungen im Zustand `reserviert`
  oder `wartend`.
  - Die Seite zeigt die Buchung und alle anderen Fahrräder, die im geplanten Zeitraum frei sind, mit Typ und neuem Preis.
  - „Auf dieses Fahrrad umbuchen“ verlegt die Buchung. Zeitraum und Person bleiben, der Preis wird mit dem Tarif des neuen Fahrrads berechnet.
  - Danach geht es mit der Meldung „Buchung wurde umgebucht.“ zur Buchungsliste: zum Reiter „Reserviert“, wenn der Beginn noch bevorsteht, sonst zu „Aktiv“ (eine wartende Buchung ist nach dem Umbuchen sofort aktiv).
  - Ob das Ziel frei ist, wird unter derselben Sperre geprüft wie beim Buchen.

> AK-7 (Preis): Alle Beispiele der Tabelle in 4.6 ergeben exakt den angegebenen Preis. Mit geänderter Konfiguration
> (Citybike 6,00 €) kostet 10:00–11:01 genau 12,00 €.

> AK-8 (Vorschau): „Preis berechnen“ zeigt Zeitraum, Dauer, Erklärung und Preis. Danach existiert keine neue Buchung.

> AK-9 (geänderte Angaben): **Gegeben** eine Vorschau für 1 Std, **wenn** die Rückgabe auf 3 Std geändert und direkt
> „Verbindlich buchen“ gedrückt wird, **dann** entsteht keine Buchung, und die neue Vorschau zeigt den neuen Preis mit Hinweis.

> AK-10 (Reservierung): Ein Fahrrad, das für morgen gebucht wird, bleibt `verfuegbar` mit „reserviert ab …“, und das Dashboard zählt
> 1 Reservierung mehr. **Wenn** die Uhr den Beginn erreicht, ist es `ausgeliehen`, ohne weiteren Speichervorgang.

> AK-11 (Überschneidung): **Gegeben** `[10:00, 12:00)`.
> - Abgelehnt werden `[11:00, 13:00)`, `[09:00, 14:00)`, `[10:30, 11:30)` und derselbe Zeitraum, mit der Meldung „Fahrrad ist im gewählten Zeitraum bereits ausgeliehen oder reserviert.“
> - Erlaubt sind `[12:00, 13:00)`, `[09:00, 10:00)` und jeder Zeitraum für ein anderes Fahrrad.

> AK-12 (parallel): Viele gleichzeitige Anfragen für überlappende Zeiträume desselben Fahrrads ergeben genau eine Buchung. Das gilt auch über HTTP und wird mit `-race` geprüft.

> AK-13 (Rückgabe): **Gegeben** E-Bike 09:00–12:00 (36,00 €).
> - Rückgabe um 10:15: Die tatsächliche Rückgabe 10:15 wird gespeichert, der Endpreis bleibt 36,00 €, und das Fahrrad ist ab 10:15 wieder buchbar.
> - Rückgabe um 12:20: 4 angefangene Stunden, Endpreis 48,00 €.
> - Erneute Rückgabe → „Ausleihe wurde bereits zurückgegeben.“
> - Eine überfällige Ausleihe blockiert neue Buchungen bis zur Rückgabe.

> AK-14 (Storno): Eine stornierte Reservierung ist gelöscht, und der Zeitraum ist frei. Reservierung zurückgeben → „Die Reservierung hat noch nicht begonnen.
> Storniere sie stattdessen.“ Aktive Ausleihe stornieren → „Nur Reservierungen, die noch nicht begonnen haben, können storniert werden.“

> AK-15 (Eingaben):
> - Rückgabe ≤ Beginn → „Rückgabe muss nach dem Beginn liegen“.
> - Beginn vor der aktuellen Minute → „Beginn darf nicht in der Vergangenheit liegen“. Ein Beginn in der aktuellen Minute ist erlaubt.
> - Fehlendes Datum → „Bitte Datum und Uhrzeit angeben“.
> - 59 Minuten → „Verleihdauer muss mindestens 1 Stunde betragen“. Genau 1 Stunde ist erlaubt.
> - Mehr als 30 Tage → „Verleihdauer darf höchstens 30 Tage betragen“. Genau 30 Tage sind erlaubt. Beide Grenzen kommen aus dem Plugin.
> - Bei „Sofort“ ist der Beginn die aktuelle Minute, und die Buchung ist danach `aktiv`.

> AK-16 (Wartung): Ein Fahrrad in Wartung kann nicht gebucht werden: „Fahrrad ist in Wartung und kann nicht gebucht werden.“

### F-6 Buchungsübersicht (`GET /buchungen`)
- Tabelle: ID, Fahrrad (mit Typ), Person, Zeitraum, tatsächliche Rückgabe, Dauer, Preis bzw. Endpreis, Status-Badge, Aktion.
- Filter `?status=aktiv|reserviert|abgeschlossen|alle` (Standard `aktiv`). `reserviert` enthält auch wartende Buchungen.
- Sortierung: neuester Beginn zuerst. Reservierungen stehen mit dem nächsten Beginn zuerst.
- Reservierte und wartende Buchungen haben die Buttons „Umbuchen“ und „Stornieren“.
- Dauer: „2 T 3 Std“, „3 Std 30 Min“, „45 Min“.

> AK-17: Jeder Filter zeigt genau die Buchungen im passenden Zustand, Reservierungen aufsteigend nach Beginn.

### F-7 Tarife (`GET /tarife`)
Nur lesend. Die Seite zeigt:
- Stundensatz je Fahrradtyp
- Regeltext aus `Describe()`
- Mindest- und Höchstdauer
- Rechenbeispiele aus 4.6 mit den aktuell konfigurierten Sätzen
- einen Hinweis auf das Flag `-tarife`

> AK-18: Mit `-tarife "citybike=6"` zeigen Tarifseite, Fahrradliste und Vorschau 6,00 €/Std. Ein ungültiges Flag (`ebike=0`, `roller=5`,
> `citybike=abc`) verhindert den Start, und das Programm nennt den Fehler.

### F-8 Framework und Plugins
- **F-8.1** Der Service arbeitet mit jeder registrierten Verleihart. Er kennt nur die Interfaces aus 3.2.
- **F-8.2** Die Webanwendung registriert genau das Fahrrad-Plugin. `main.go` ist die einzige Stelle, an der das passiert.
- **F-8.3** Sind weitere Verleiharten registriert (z. B. in Tests das Ski-Plugin), zeigt und bucht die Fahrrad-App trotzdem nur Fahrräder. Ein Objekt einer anderen Verleihart gilt in ihren Formularen als ungültige Auswahl und auf ihren Seiten als nicht gefunden.

> AK-19 (Registry):
> - `Register` lehnt ab: doppelten Schlüssel (`ErrDuplicateKind`), leeren oder ungültigen Schlüssel, `nil` und Plugins ohne Kategorien oder ohne Preisstrategie.
> - `Get` liefert für einen unbekannten Schlüssel `ErrUnknownKind`.
> - `All` liefert die Plugins in Registrierungsreihenfolge.
> - Parallele Lesezugriffe sind sicher (`-race`).

> AK-20 (Test-Plugin): Eine nur im Test definierte Verleihart mit Pauschalpreis 7,00 € kann Objekte anlegen, buchen,
> zurückgeben und stornieren. Überschneidungsprüfung und Merkmalsprüfung funktionieren, ohne dass Framework-Code geändert wird.

> AK-21 (zweite Verleihart): Fahrrad- und Ski-Plugin laufen im selben Service. Für `[10.11. 10:00, 12.11. 10:00)` kostet ein Citybike
> 48 angefangene Stunden × 5,00 € = 240,00 €, ein Ski 3 Kalendertage × 20,00 € = 60,00 €.
> Eine Buchung des Ski blockiert das Fahrrad nicht. Die Statistik lässt sich je Verleihart abfragen.

> AK-22 (Architektur): Ein Test liest alle Nicht-Test-Dateien unter `framework/` und schlägt fehl, wenn A-R1 oder A-R3 verletzt ist.
> Für A-R2 prüft er die Importe unter `plugins/`.

> AK-23 (unbekannte Verleihart): Ein Objekt mit nicht registrierter Verleihart oder unbekannter Kategorie wird nicht angelegt (I-7).

> AK-24 (Konflikt): **Gegeben** Citybike C mit Ausleihe A `[10:00, 12:00)` und Reservierung B `[13:00, 15:00)`.
> - Um 12:30 ist A überfällig, B ist `reserviert` mit Konflikt, und das Dashboard zeigt 1 Konflikt.
> - Um 13:30 ist B `wartend` (nicht `aktiv`), und C hat genau eine aktive Ausleihe (A).
> - Wird A um 13:40 zurückgegeben, ist B ohne Konflikt `aktiv` und kann zurückgegeben werden. A erhält den Endpreis für 4 angefangene Stunden.

> AK-25 (Umbuchen): **Gegeben** eine Buchung B mit Konflikt und ein freies Citybike D. **Wenn** B auf D umgebucht wird, **dann** gehört B zu D,
> der Konflikt ist weg, und der Preis entspricht dem Preis von D für denselben Zeitraum. Abgelehnt (409) wird das Umbuchen
> - auf ein belegtes Fahrrad oder ein Fahrrad in Wartung,
> - einer aktiven oder abgeschlossenen Buchung, mit der Meldung „Nur Buchungen, die noch nicht ausgegeben wurden, können umgebucht werden.“

> AK-26 (Ausweisbild): Eine Person ohne Bild wird angelegt. Ein PNG-Testbild wird gespeichert und über die Bild-Route mit `no-store` ausgeliefert.
> Eine Textdatei oder ein Bild über 2 MB ergibt „Ausweisbild muss JPEG oder PNG und höchstens 2 MB groß sein“. Eine Anfrage über 3 MB ergibt 413.

> AK-27 (Bedienung): Ausgehend vom Dashboard mit Demo-Daten führt jeder interne Link zu einer Seite mit Status 200. Jedes POST-Formular (Button) auf diesen Seiten hat eine Route und antwortet mit 200, 303, 409 oder 422, nie mit 404, 405 oder 500.

### F-9 Bedienoberfläche
- Navigation: Dashboard · Fahrräder · Personen · Buchungen · Verfügbarkeit · Tarife · Button „+ Buchung“.
- Konflikte sind überall mit einem roten Badge „Konflikt“ markiert, wartende Buchungen mit „Wartet auf Rückgabe“.
- Post/Redirect/Get. Flash-Meldungen kommen nur über feste Schlüssel.
- Löschen nur über eine Bestätigungsseite, ohne JavaScript.
- Leere Listen zeigen einen erklärenden Text und einen passenden Button.
- Responsiv ab 360 px: Tabellen scrollen horizontal, die Seite selbst nicht.
- Deutsch, Datum `02.11.2026 14:05`, Geld `1.240,00 €`.
- **Gestaltung:** angelehnt an den Ski-Verleih (Seitenleiste, Kacheln, Karten, Badges, Preis-Karte, CSS-Variablen, Inline-SVG-Symbole),
  aber als eigene Anwendung erkennbar:
  - eigenes Logo (Fahrrad-Symbol) und eigener Name „Velo-Verleih“
  - eigene Akzentfarbe (Grün-Petrol statt Blau)
  - Fahrradtypen mit eigenen Farbpunkten

---

## 6. Nicht-funktionale Anforderungen

| ID    | Anforderung |
|-------|-------------|
| NF-1  | Go ≥ 1.22, eigenes Modul `fahrradverleih`, nur Standardbibliothek, keine `require`-Einträge |
| NF-2  | Kein JavaScript notwendig, alle Funktionen mit HTML-Formularen |
| NF-3  | In-Memory-Daten, threadsicher (`sync.RWMutex`), Store gibt Kopien zurück |
| NF-4  | `html/template`, Größenbegrenzung der Formulare (`http.MaxBytesReader`: 64 KB, auch mit `charset`-Zusatz im Content-Type; Personenformular 3 MB) |
| NF-5  | Jede Validierung serverseitig |
| NF-6  | Start mit `go run .` auf **`:8081`**, änderbar mit `-addr` |
| NF-7  | `-demo` lädt Beispieldaten: 9 Fahrräder (je 3 pro Typ, davon 1 in Wartung), 4 Personen und 5 Buchungen: 2 aktiv (davon 1 überfällig), 2 Reservierungen (davon 1 im Konflikt mit der überfälligen Ausleihe), 1 abgeschlossen |
| NF-8  | `go test ./...` und `go test -race ./...` für Framework, Plugins und Handler (`httptest`) |
| NF-9  | Templates und CSS per `embed`, eine einzige ausführbare Datei |
| NF-10 | Request-Logging (Methode, Pfad, Status, Dauer) |
| NF-11 | Nur fiktive Daten: erfundene Namen, Ausweisnummern mit Präfix `TEST`, E-Mails auf `example.com` |
| NF-12 | Zeitabhängige Regeln bekommen `now` von außen. Der Service bekommt `now func() time.Time` |
| NF-13 | Das Projekt läuft unabhängig von `ski-verleih` und lässt sich allein als ZIP weitergeben |

---

## 7. Routen

| Methode | Pfad | Zweck |
|---------|------|-------|
| GET  | `/` | Dashboard |
| GET  | `/fahrraeder` | Liste mit Filtern |
| GET  | `/fahrraeder/neu` | Formular |
| POST | `/fahrraeder` | Anlegen |
| GET  | `/fahrraeder/{id}` | Detail |
| GET  | `/fahrraeder/{id}/bearbeiten` | Formular |
| POST | `/fahrraeder/{id}` | Speichern |
| GET  | `/fahrraeder/{id}/loeschen` | Bestätigung |
| POST | `/fahrraeder/{id}/loeschen` | Löschen |
| GET/POST | `/personen…` | analog zu Fahrrädern, plus `?q=` |
| GET  | `/personen/{id}/ausweisbild` | Ausweisbild (falls vorhanden) |
| GET  | `/verfuegbarkeit` | Verfügbarkeitssuche (Formular per GET) |
| GET  | `/buchungen` | Übersicht mit Filter |
| GET  | `/buchungen/neu` | Buchungsformular |
| POST | `/buchungen` | Vorschau oder Bestätigung |
| POST | `/buchungen/{id}/rueckgabe` | Zurückgeben |
| POST | `/buchungen/{id}/stornieren` | Stornieren |
| GET  | `/buchungen/{id}/umbuchen` | freie Alternativen anzeigen |
| POST | `/buchungen/{id}/umbuchen` | Umbuchen (Feld `fahrrad`) |
| GET  | `/tarife` | Tarifübersicht |
| GET  | `/static/…` | CSS |

Flash-Schlüssel: `bike_created`, `bike_updated`, `bike_deleted`, `person_created`, `person_updated`, `person_deleted`,
`rented`, `reserved`, `returned`, `cancelled`, `reassigned`.

---

## 8. Fehler

| Fehler (`framework/domain`) | HTTP | Text in der Fahrrad-App |
|-----------------------------|------|-------------------------|
| `ErrNotFound`               | 404  | „Eintrag wurde nicht gefunden.“ |
| `ErrUnknownKind`            | 500  | „Unbekannte Verleihart.“ |
| `ErrDuplicateKind`, `ErrInvalidPlugin` | – | nur beim Start |
| `ErrItemInMaintenance`      | 409  | „Fahrrad ist in Wartung und kann nicht gebucht werden.“ |
| `ErrItemBooked`             | 409  | „Fahrrad ist im gewählten Zeitraum bereits ausgeliehen oder reserviert.“ |
| `ErrItemInUse`              | 409  | „Fahrrad ist ausgeliehen oder reserviert – Status kann nicht geändert werden.“ |
| `ErrAlreadyReturned`        | 409  | „Ausleihe wurde bereits zurückgegeben.“ |
| `ErrNotStarted`             | 409  | „Die Reservierung hat noch nicht begonnen. Storniere sie stattdessen.“ (auch für `wartend`) |
| `ErrNotCancellable`         | 409  | „Nur Reservierungen, die noch nicht begonnen haben, können storniert werden.“ |
| `ErrNotReassignable`        | 409  | „Nur Buchungen, die noch nicht ausgegeben wurden, können umgebucht werden.“ |
| `ErrHasHistory`             | 409  | „… hat Buchungen und kann nicht gelöscht werden.“ |
| `ValidationErrors`          | 422  | Feldfehler am Formular |

Die Framework-Fehler sind neutral formuliert. Erst der Handler der Fahrrad-App setzt „Fahrrad“ in die Texte ein.

---

## 9. Projektstruktur

```
fahrrad-verleih/
├── go.mod, main.go, main_test.go
├── CLAUDE.md, README.md, docs/SPEZIFIKATION.md
├── framework/
│   ├── doc.go, architecture_test.go
│   ├── domain/   errors.go text.go money.go period.go person.go item.go field.go booking.go (+ _test.go)
│   ├── plugin/   plugin.go registry.go (+ registry_test.go)
│   ├── store/    memory.go (+ memory_test.go)
│   └── service/  repository.go service.go item.go person.go booking.go stats.go (+ Tests, plugin_test.go)
├── plugins/
│   ├── fahrrad/  plugin.go tariff.go config.go (+ Tests)
│   └── ski/      plugin.go (+ Tests)
├── internal/demo/     demo.go (+ Test)
├── internal/handler/  server.go messages.go dashboard.go bike.go person.go booking.go availability.go tariff.go (+ Tests)
└── web/  embed.go, templates/*.html, static/style.css
```

---

## 10. Umsetzungsplan

| Schritt | Inhalt | Status |
|---------|--------|--------|
| 1 | Spezifikation, CLAUDE.md, README.md, Architektur | ✅ |
| 2 | `framework/domain`: Period, Money, Person, Item, Field-Prüfung, Booking-Zustände, Belegung, Konflikte + Tests | ✅ |
| 3 | `framework/plugin`: Interfaces, Registry + Tests (AK-19) | ✅ |
| 4 | `framework/store` + `framework/service` (Buchen, Vorschau, Rückgabe, Storno, Umbuchen, Verfügbarkeit, Statistik) + Tests inkl. Parallelität | ✅ |
| 5 | `plugins/fahrrad`: Typen, Merkmale, Validierung, Stundentarif, Konfiguration + Tests (AK-2, AK-3, AK-7) | ✅ |
| 6 | `plugins/ski` + Test-Plugin + Architekturtest (AK-20 bis AK-23) | ✅ |
| 7 | Webanwendung: Layout, Dashboard, Fahrräder, Personen | ✅ |
| 8 | Webanwendung: Buchen mit Vorschau, Übersicht, Rückgabe, Storno, Umbuchen, Verfügbarkeit, Tarife | ✅ |
| 9 | Handler-Tests, `go test ./...`, `go test -race ./...`, `go vet`, Browserprüfung (Desktop + 375 px) | ✅ |
| 10 | README: Start, Bedienung, Testanleitung, Entscheidungen | ✅ |
| 11 | Abschlussprüfung: Crawler über alle Links und Buttons, Regressionstests, Eigenständigkeit, Dokumente | ✅ |

---

## 11. Festlegungen (bestätigt am 2026-11-02)

| Nr. | Festlegung |
|-----|------------|
| A-1 | Stundensätze: Citybike 5 €, Mountainbike 8 €, E-Bike 12 €. Konfigurierbar über `fahrrad.Config` und beim Start über `-tarife`. Keine Änderung zur Laufzeit. |
| A-2 | Jede angefangene Stunde wird voll berechnet, mindestens 1 Stunde (2 Std 15 Min → 3 Std). Kein Tagesdeckel. |
| A-3 | Dauer mindestens 1 Stunde, höchstens 30 Tage. Beginn nicht in der Vergangenheit (minutengenau, Option „Sofort“). Ende nach dem Beginn. |
| A-4 | Ausweistyp und -nummer sind Pflicht, ein Ausweisbild ist optional. |
| A-5 | Rückgabe wie 1b: Der tatsächliche Zeitpunkt wird gespeichert, verspätete Stunden werden nachberechnet, vorzeitig gibt es keine Erstattung. Konflikte mit nachfolgenden Buchungen werden erkannt und angezeigt (4.5a). |
| A-6 | Ski-Plugin mit Tagestarif ist implementiert und getestet, ohne Weboberfläche. |
| A-7 | Eigenständige, fahrradspezifische Weboberfläche. Das Framework enthält kein HTTP. |
| A-8 | Name in der Oberfläche: „Velo-Verleih“. |
| A-9 | Eine wartende Buchung behält ihren gebuchten Preis. Ihr Endpreis wird wie bei jeder Buchung ab dem geplanten Beginn berechnet. Wer das nicht möchte, bucht um oder storniert. |

## 12. Offene Punkte (bewusst nicht umgesetzt)

| Nr. | Punkt | Auswirkung | Mögliche Erweiterung |
|-----|-------|------------|----------------------|
| O-1 | Es gibt keine ausdrückliche Ausgabe („Abholung“). Wie in 1b wird eine Buchung zur Startzeit automatisch aktiv. | Wird eine Reservierung nicht abgeholt, gilt sie trotzdem als ausgeliehen und muss storniert oder zurückgegeben werden. | Aktion „Ausgeben“ mit gespeichertem Zeitpunkt |
| O-2 | Wird eine überfällige Ausleihe sehr spät zurückgegeben, ist eine wartende Folgebuchung danach sofort aktiv – und gegebenenfalls selbst schon überfällig. | Mitarbeitende sollten Konflikte vorher umbuchen oder stornieren (Dashboard-Hinweis). | Endpreis ab tatsächlicher Ausgabe (siehe O-1) |
| O-3 | Tarife lassen sich nur beim Start ändern (A-1). | Für neue Preise ist ein Neustart nötig, dabei gehen die In-Memory-Daten verloren. | Tarifhistorie mit Gültigkeitsbeginn |
| O-4 | Keine dauerhafte Speicherung (Abschnitt 1.3). | Nach einem Neustart sind alle Daten weg. | Repository-Implementierung mit Datei oder Datenbank; die Interfaces in `framework/service/repository.go` sind dafür vorbereitet |

