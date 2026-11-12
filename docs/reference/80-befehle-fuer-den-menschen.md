---
title: Befehle für den Menschen
description: "a38 grants, fault, doctor und audit: Argumente, Ausgabe und Exit-Codes."
tags:
  - Thema/Betrieb
---
# Befehle für den Menschen

Diese Befehle bedient der Mensch.
Ein Agent hat keinen Zugang zu ihnen: Die Befehle `grants`, `fault` und `doctor` sprechen mit dem Daemon über den Admin-Socket.
Der Agent-Socket lehnt die Nachrichten des Admin-Sockets ab.
Ein Befehl zeigt nie einen Secret-Wert.

Alle Befehle lesen die Konfigurationsdatei mit `--config <datei>`.
Ohne das Argument gilt `A38_CONFIG`, danach der Standardort.
Die Befehle `grants`, `fault` und `doctor` kennen `--json`.

Die Schalter dürfen vor und nach den Argumenten stehen.
`a38 grants revoke <grant-id> --config <datei>` und `a38 grants revoke --config <datei> <grant-id>` sind gleich.
Ein `--` beendet die Schalter.

## Exit-Codes

| Code | Bedeutung |
|---|---|
| 0 | Der Befehl ist gelungen. |
| 1 | Der Befehl ist gescheitert, oder `doctor` hat einen Befund. |
| 2 | Der Aufruf ist falsch. |

Läuft der Daemon nicht, nennen `grants` und `fault` den Grund und enden mit 1.

## a38 serve

`a38 serve` startet den Daemon.
Der Mensch startet ihn, nicht der Agent.

| Schalter | Bedeutung |
|---|---|
| `--config <datei>` | Konfigurationsdatei, siehe oben. |
| `--backend fake\|1password` | Das Backend. Ein echtes Backend verlangt `--ui app` ({{ ref("D40") }}). |
| `--fake-file <datei>` | YAML-Datei des Fake-Backends. Pflicht mit `--backend fake`. |
| `--ui app\|auto-approve\|auto-deny\|script` | Die Freigabe. `app` startet die Freigabe-App. Die anderen Werte sind nur mit dem Fake-Backend erlaubt. `app` mit dem Fake-Backend ist erlaubt, um die Oberfläche zu testen. |
| `--ui-script <befehl>` | Programm der Freigabe mit `--ui script`. |
| `--ui-app <pfad>` | Pfad der Freigabe-App mit `--ui app`. |

### Obergrenze eines Bündels

Der Schlüssel `bundle.max_entries` der Konfiguration legt fest, wie viele Einträge ein Bündel höchstens hat.
Der Standard ist 10.
Der größte erlaubte Wert ist 50.
Die Freigabe-App lehnt eine Anfrage mit mehr als 50 Einträgen ab.
`a38 serve` und jeder andere Befehl lehnen die Konfiguration mit einem größeren Wert beim Laden ab.
In Go steht die Grenze in `config.MaxBundleEntries`, in Swift in `app/Sources/A38ApprovalCore/Decoding.swift`.

### Ort der Freigabe-App

Der Daemon sucht den Pfad in dieser Reihenfolge:

1. Schalter `--ui-app`. Ein relativer Pfad gilt ab dem Arbeitsverzeichnis von `a38 serve`.
2. Schlüssel `paths.ui_app` der Konfigurationsdatei.
3. Der Daemon löst die symbolischen Links seines eigenen Binarys auf. Danach sucht er der Reihe nach:
    1. Läuft `a38` in einem Bundle (`<X>.app/Contents/MacOS/a38`), das Geschwister `<X>.app/Contents/MacOS/Passierschein`.
    2. Neben `a38`: `Passierschein.app/Contents/MacOS/Passierschein`, danach eine einfache ausführbare Datei `Passierschein`.
    3. `/Applications/Passierschein.app/Contents/MacOS/Passierschein`, danach `~/Applications/Passierschein.app/Contents/MacOS/Passierschein`.

Der erste vorhandene Treffer gilt.
Die Prüfung der Datei unten gilt für jeden Treffer.
Ein Bundle in `/Applications` gehört meist dem Benutzer oder `root`; beides ist erlaubt.
Der Daemon schreibt beim Start den gewählten Pfad ins Log.
Die App heißt „Passierschein“, das Werkzeug `a38` ({{ ref("D50") }}).

### Auslieferung

Das Release kann `a38` im Bundle `Passierschein.app` ausliefern (`Contents/MacOS/a38`) und einen symbolischen Link in den `PATH` legen.
Der Linux-Build bleibt das einzelne Binary `a38` ohne App.
Die Paketierung gehört zur Arbeit am Release ({{ ref("G5") }}, {{ ref("D46") }}) und nicht zu diesem Stand.

### Prüfung vor jedem Start

Der Daemon prüft die Datei vor jedem Start der App ({{ ref("D6") }}, {{ ref("D40") }}):

- Die Datei existiert und ist eine reguläre Datei. Ein symbolischer Link gilt als sein Ziel.
- Die Datei gehört dem Benutzer des Daemons oder `root`.
- Weder die Gruppe noch andere dürfen schreiben.
- Der Besitzer darf die Datei ausführen.
- Die Datei hat kein Setuid- und kein Setgid-Bit.

Besteht die Datei eine Prüfung nicht, startet der Daemon die App nicht.
Die Anfrage endet mit `ui_failed`, und der Daemon schreibt eine Zeile ins Log.
Der Daemon selbst startet trotzdem.
`a38 doctor` meldet den Befund in der Prüfung `approval UI`.
Die Prüfung der Code-Signatur fehlt noch ({{ ref("G5") }}).

## a38 grants

| Aufruf | Wirkung |
|---|---|
| `a38 grants list` | Zeigt die Grants aller Sitzungen. |
| `a38 grants revoke <grant-id>` | Beendet einen Grant. Die nächste Anfrage braucht eine neue Freigabe. |

`list` zeigt eine Tabelle mit diesen Spalten:

| Spalte | Inhalt |
|---|---|
| `GRANT` | Kennung des Grants |
| `SESSION` | die ersten acht Zeichen der Kennung der Sitzung |
| `NAME` | Name der Sitzung |
| `PROJECT` | Projekt der Sitzung |
| `CREDENTIALS` | Referenzen, nie ein Wert |
| `REMAINING` | Restlaufzeit |

Ohne Grants steht `No active grants.` in der Ausgabe.
Mit `--json` gibt `list` ein JSON-Array mit den vollen Angaben aus, auch mit der ganzen Kennung der Sitzung.
Ein unbekannter Grant endet mit einer Meldung und dem Exit-Code 1.
Der Widerruf steht als Ereignis `revocation` im Audit-Log.

## a38 fault

Der Befehl bedient die globale Sperre der Auslieferung (siehe [Störung finden und beheben](../how-to/24-stoerung-finden-und-beheben.md)).

| Aufruf | Wirkung |
|---|---|
| `a38 fault status` | Zeigt, ob die Auslieferung gesperrt ist, mit Ursache und Beginn, soweit der Daemon sie meldet. Exit-Code 1, solange die Sperre besteht. |
| `a38 fault clear` | Bittet den Daemon, die Sperre aufzuheben. Der Daemon hebt sie nur auf, wenn die Ursache behoben ist. Sonst steht der Befund in der Ausgabe, und der Exit-Code ist 1. |

Die Namen sind eine Wahl von Issue 25 und warten auf die Bestätigung durch Stefan.

## a38 doctor

`a38 doctor` führt die Prüfungen aus.
Jede Prüfung steht als Zeile `OK` oder `FINDING`.
Ein Befund nennt die Ursache und den nächsten Schritt.
Der Exit-Code ist 0 ohne Befund.

| Prüfung | Inhalt |
|---|---|
| `configuration` | Die Konfiguration lädt. Je ungültigem Projekt folgt ein Befund mit dem Namen des Projekts. |
| `socket modes` | Das Verzeichnis hat den Modus 0700, die Sockets 0600. |
| `daemon` | Der Daemon antwortet auf dem Admin-Socket. Die Zeile nennt Version, Sitzungen und Grants. |
| `delivery lock` | Zustand der Sperre mit Ursache und Beginn. Nächster Schritt: `a38 fault clear`. |
| `audit file` | Die Datei existiert und ist schreibbar, das Verzeichnis ebenfalls. |
| `audit space` | Im Verzeichnis sind mindestens 100 MiB frei. |
| `audit chain` | Die Hash-Kette stimmt. Die Prüfung liest die Datei nur. |
| `backend` | Zustand des Backends, soweit der Daemon ihn meldet. |
| `approval UI` | Zustand der Freigabe-App, soweit der Daemon ihn meldet. Im Modus `app` prüft die Prüfung die Datei der App wie der Daemon vor dem Start. |

Mit `--json` gibt der Befehl ein Objekt mit `ok` und der Liste `results` aus.
Jedes Ergebnis hat `check`, `ok`, `detail` oder `finding`, `cause` und `next_step`.

## a38 audit

| Aufruf | Wirkung |
|---|---|
| `a38 audit list` | Listet das Audit-Log mit Filtern (`--project`, `--session-name`, `--ref`, `--type`, `--code`, `--since`, `--until`, `--limit`). |
| `a38 audit verify` | Prüft die Hash-Kette. Bei einem Bruch nennt der Befehl den ersten fehlerhaften Eintrag und endet mit 1. |

Beide Befehle öffnen die Audit-Datei nur lesend.
Der Daemon muss nicht laufen.
`--file <audit-datei>` ersetzt den Pfad aus der Konfiguration.
`list` kennt `--json` und gibt je Eintrag eine Zeile aus.
