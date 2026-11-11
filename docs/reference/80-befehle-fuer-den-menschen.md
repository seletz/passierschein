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

## Exit-Codes

| Code | Bedeutung |
|---|---|
| 0 | Der Befehl ist gelungen. |
| 1 | Der Befehl ist gescheitert, oder `doctor` hat einen Befund. |
| 2 | Der Aufruf ist falsch. |

Läuft der Daemon nicht, nennen `grants` und `fault` den Grund und enden mit 1.

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
| `approval UI` | Zustand der Freigabe-App, soweit der Daemon ihn meldet. |

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
