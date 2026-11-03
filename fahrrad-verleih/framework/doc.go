// Package framework ist das wiederverwendbare Verleih-Framework (Spezifikation Abschnitt 3).
//
//   - domain:  Typen und Regeln jedes Verleihs (Person, Objekt, Buchung, Zeitraum, Geld)
//   - plugin:  Schnittstellen für Verleiharten und die Registry
//   - store:   In-Memory-Speicher
//   - service: Anwendungsfälle (Buchen, Rückgabe, Storno, Umbuchen, Statistik)
//
// Das Framework kennt keine konkrete Verleihart. Was verliehen wird und wie abgerechnet
// wird, liefert ein Plugin. Der Architekturtest in diesem Verzeichnis prüft das (AK-22).
package framework
