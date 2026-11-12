---
title: Offene Gaps
description: "Durchsicht der offenen Gaps am 2026-11-12: was geschlossen ist, was vertagt ist, was offen bleibt und wer handelt."
render_macros: true
tags:
  - Thema/Design
---
# Offene Gaps

Diese Seite hält die Durchsicht der offenen Gaps vom 2026-11-12 fest ([Issue 58](https://github.com/seletz/passierschein/issues/58)).
Zu Beginn waren 30 Gaps offen.

## Regel der Durchsicht

- Ein Gap ist geschlossen, wenn jede offene Frage eine Antwort hat.
- Eine Antwort ist eine Entscheidung ohne `beleg.annahme`, eine Aussage von Stefan in einem gemergten Pull Request oder einem Artefakt, ein bestandener Test oder ein festgehaltener Praxistest.
- Eine Annahme der Entwicklung schließt kein Gap. Das gilt auch für eine Entscheidung mit `beleg.annahme: true` ({{ ref("D49") }}, {{ ref("D51") }}, {{ ref("D52") }}).
- Ein Gap ist vertagt, wenn es eine spätere Stufe betrifft und nichts in den Stufen 1 bis 3 davon abhängt ({{ ref("D23") }}).
- Ein Gap mit einer Teilantwort bleibt offen. Die Tabelle nennt den Rest.

## Ergebnis

| Ergebnis | Anzahl |
|---|---|
| geschlossen | 1 |
| vertagt | 3 |
| offen | 26 |

Die offenen Gaps stehen nach ihrer Bedeutung für das MVP geordnet.
Rang 1 ist am wichtigsten.

## Tabelle

| Rang | Gap | Thema | Ergebnis | Beleg | Was fehlt | Wer handelt |
|---|---|---|---|---|---|---|
| 1 | {{ ref("G30") }} | Grant hängt an Namen statt an einer festen Kennung | offen | {{ ref("D52") }} beantwortet beide Fragen; Tests `TestReadByID`, `TestDuplicateTitlesFailClosed` | {{ ref("D52") }} ist eine Annahme. | Stefan entscheidet |
| 2 | {{ ref("G23") }} | Referenz in anderer Schreibweise | offen | {{ ref("D52") }}: genauer Vergleich, keine Kennung im Segment des Items | {{ ref("D52") }} ist eine Annahme. | Stefan entscheidet |
| 3 | {{ ref("G3") }} | Zugang zu 1Password | offen | Integrationstest {{ ref("T15") }} und Praxistest am 2026-11-12 bestanden ([PR 51](https://github.com/seletz/passierschein/pull/51), [PR 52](https://github.com/seletz/passierschein/pull/52)); Ziel Schlüsselbund von Stefan festgelegt ({{ ref("D51") }}, Punkt 1); Namen ohne Werte: das SDK liefert die Felder nur mit Werten, `Resolve` verwirft das Item sofort | Wie viele Vaults der Service Account liest; ob das Rate Limit reicht; Bestätigung der Token-Datei als Übergang ({{ ref("D51") }} ist eine Annahme); Entscheidung über die Empfehlung des [Spikes zum Schlüsselbund](../explanation/spikes/50-token-im-schluesselbund.md) und dessen Praxistest; Desktop-Integration unter einem LaunchAgent | Stefan entscheidet; Praxistest |
| 4 | {{ ref("G5") }} | Signatur der Freigabe-App mit Developer ID | offen | {{ ref("D50") }} legt fest, wie der Daemon die App findet; Swift 6.4.2 ist je Task in `mise.toml` festgelegt ([PR 46](https://github.com/seletz/passierschein/pull/46)) | Notarisierung und Ticket im Archiv; Ablage durch `mise` mit dem Backend `github:`; ob der Daemon die Signatur prüft (heute `NoopVerifier`); Swift auf dem macOS-Runner ([Issue 55](https://github.com/seletz/passierschein/issues/55)); neu aus dem Spike zum Schlüsselbund: Signiert der Release-Job auch `a38` mit Hardened Runtime? | Entwicklung (Release-Job); Stefan entscheidet über die Prüfung der Signatur; Praxistest mit Developer ID |
| 5 | {{ ref("G19") }} | Name der Sitzung | offen | {{ ref("D25") }}: Name aus der Anmeldung des MCP-Servers, unbestätigt, gleicher Name ist ein Fehler; Praxistest: der Name stimmt ([PR 52](https://github.com/seletz/passierschein/pull/52)) | Form `--session-name` und `A38_SESSION_NAME` ist nicht als Entscheidung festgehalten; wer den Wert je neuer Sitzung setzt; Anzeige nach einer Umbenennung; Gegenstück in anderen Agent-Werkzeugen | Stefan entscheidet |
| 6 | {{ ref("G2") }} | Eine Verbindung je Agent unter herdr | offen | {{ ref("D33") }}: ein Grant überlebt keinen Neustart des Agents | Ob jedes Werkzeug und jeder Unteragent einen eigenen Prozess `a38 mcp` startet; ob das Arbeitsverzeichnis in einem Worktree das Projekt ergibt ({{ ref("D49") }} ist eine Annahme) | Praxistest unter herdr; Stefan bestätigt {{ ref("D49") }} |
| 7 | {{ ref("G28") }} | Welche Fehler die ganze Anfrage beenden | offen | Code setzt die Annahme um ([PR 34](https://github.com/seletz/passierschein/pull/34)) | Ob Einträge unter einem gültigen Grant trotz Ablehnung der neuen Einträge geliefert werden | Stefan entscheidet |
| 8 | {{ ref("G21") }} | Fehlercode für einen internen Fehler | offen | Code und Tests `TestPanicLocksDelivery`, `TestPanicInHandlerLocks`; {{ ref("D48") }} gilt für jede Störung | Bestätigung, dass ein interner Fehler eine Störung nach {{ ref("D48") }} ist | Stefan entscheidet |
| 9 | {{ ref("G20") }} | Form und Aktionen der Meldung | offen | Die abgenommene Freigabe-App zeigt die Meldung ([PR 46](https://github.com/seletz/passierschein/pull/46)); {{ ref("D48") }}: ein Popup je Störung | Häufigkeit der Meldung bei `config_invalid`; Meldung, wenn der Mensch nicht am Mac sitzt; Name des Admin-Befehls (siehe {{ ref("G31") }}) | Stefan entscheidet |
| 10 | {{ ref("G33") }} | Vollbild, Arbeitsbereich, Zeitablauf, Meldung am Bildschirm | offen | Praxistest vom 2026-11-12 deckt Touch ID, OmniWM, Fokus und Tasten ab | Popup über Vollbild, beim Wechsel des Arbeitsbereichs, Zeitablauf, beide Meldungen, Touch ID bei geschlossenem Deckel und ohne Fingerabdruck | Praxistest mit Stefan |
| 11 | {{ ref("G18") }} | Zu viele Zugangsdaten in einem Bündel | offen | Stefan: der Praxistest gibt die Antwort; {{ ref("D29") }} bleibt Annahme | Praxistest mit langen Bündeln; Entscheidung über die Obergrenze und „Select all“ | Praxistest; Stefan entscheidet |
| 12 | {{ ref("G25") }} | Referenz mit Abschnitt oder Attribut | offen | {{ ref("D52") }}: Feld in einem Abschnitt erlaubt, Attribut endet mit `not_found` | {{ ref("D52") }} ist eine Annahme; ob ein Agent Attribute wie `otp` braucht | Stefan entscheidet |
| 13 | {{ ref("G31") }} | Namen von Schaltern und Befehlen | offen | {{ ref("D50") }} legt `--ui-app` fest; Stefan nennt `--session-name` und `A38_SESSION_NAME` als Weg des Namens | Bestätigung von `--fake-file`, `--ui-script`, `--session-name`, `A38_SESSION_NAME`, `a38 fault status`, `a38 fault clear` | Stefan entscheidet |
| 14 | {{ ref("G32") }} | `a38 mcp` nach dem Ende der Verbindung | offen | Code setzt die Annahme um; Test `TestDaemonUnavailableThenReachable` | Ob `a38 mcp` sich beendet | Stefan entscheidet |
| 15 | {{ ref("G27") }} | Code für das Ende der Sitzung während einer Anfrage | offen | Code setzt die Annahme um; Test `TestSessionEndedWhileRequestInFlight` | Ob der Fall einen eigenen Code braucht | Stefan entscheidet |
| 16 | {{ ref("G29") }} | Ereignisart je Ausgang | offen | Code setzt die Tabelle des Gaps um; Praxistest: erwartete Einträge, Kette intakt | Ob eine Ablehnung zusätzlich eine Entscheidung schreibt | Stefan entscheidet |
| 17 | {{ ref("G22") }} | Ereignisart für die globale Sperre | offen | Code und Test `TestT28GlobalLock` ([PR 38](https://github.com/seletz/passierschein/pull/38)) | Ob die Sperre eine eigene Ereignisart braucht | Stefan entscheidet |
| 18 | {{ ref("G24") }} | Quelle der Angabe „Name“ des Agents | offen | {{ ref("D4") }} und die abgenommene Oberfläche zeigen die Zeile „Agent“ | Bestätigung der Quelle `clientInfo` des MCP-Clients | Stefan entscheidet |
| 19 | {{ ref("G6") }} | Wie ein Agent die Namen der Items findet | offen | – | Werkzeug, Schutzbedarf der Namen, Datei im Projekt | Stefan entscheidet |
| 20 | {{ ref("G13") }} | Rotation nach einem Leck | offen | – | Wer rotiert, wie schnell; Hilfe durch `a38` | Stefan entscheidet |
| 21 | {{ ref("G8") }} | Aufbewahrung im Audit-Log | offen | – | Dauer, Löschung mit neuem Anker, Kürzung der Begründung | Stefan entscheidet |
| 22 | {{ ref("G12") }} | Maß für Freigabemüdigkeit | offen | `a38 tui` und `a38 audit stats` zeigen Popups je Tag, Anteil der Ablehnungen, Abweichungen und Größe der Bündel; Definitionen in {{ ref("D57") }} (Annahme) | Annehmbare Zahl je Tag, Warnsignal, Hinweise in der Freigabeanfrage | Stefan entscheidet; Praxistest liefert Zahlen |
| 23 | {{ ref("G14") }} | Design-Check in der CI | offen | `mise run design:check` läuft weiter nur lokal | Token für den Download von `dg-register` | Stefan entscheidet |
| 24 | {{ ref("G15") }} | Pages, Provenienz und cosign im privaten Repo | offen | Der Release-Workflow attestiert nur bei einem öffentlichen Repo | Öffentliche Doku, Provenienz, Signatur der Prüfsummendatei | Stefan entscheidet |
| 25 | {{ ref("G16") }} | Persönlicher Assistent auf dem Mac | offen | – | Ausnahme oder Weg über `a38` | Stefan entscheidet |
| 26 | {{ ref("G10") }} | Persona der Coding Agents im Cluster | offen | {{ ref("D14") }}: nicht Teil von `a38` | Name, Tag und Rechte der Persona | Stefan entscheidet, außerhalb von `a38` |
| – | {{ ref("G1") }} | Erkennung der VM und Transport in die VM | vertagt | Spike 40 empfiehlt Tailscale; Stufe 5 nach {{ ref("D23") }} | Entscheidung über die Empfehlung und Praxistest | Spätere Stufe (5) |
| – | {{ ref("G11") }} | Auslieferung als Datei auf einem tmpfs | vertagt | {{ ref("D24") }}: die Auslieferung als Datei entsteht mit der VM | Entwurf des Ablaufs | Spätere Stufe (5) |
| – | {{ ref("G9") }} | Freigabe, wenn der Mac nicht erreichbar ist | vertagt | {{ ref("D53") }}: der Daemon läuft nur auf dem Mac; {{ ref("D23") }}: das MVP ist lokal | Freigabe ohne erreichbaren Mac | Spätere Stufe (5 und später) |
| – | {{ ref("G26") }} | Prototyp zeigt ausgeschlossene Fälle | geschlossen | Abnahme der Oberfläche durch Stefan ([PR 46](https://github.com/seletz/passierschein/pull/46), [Issue 39](https://github.com/seletz/passierschein/issues/39)); Text zum Umfang: „The grant covers only the selected credentials …“; {{ ref("D32") }}, {{ ref("D24") }} | – | – |

## Was eine aufgezeichnete Entscheidung braucht

Zwei Antworten hat Stefan gegeben, aber kein Artefakt hält sie als Entscheidung fest:

- Die Form des Namens der Sitzung: `a38 mcp --session-name` und `A38_SESSION_NAME` in der Anmeldung des MCP-Servers ({{ ref("G19") }}, {{ ref("G31") }}).
- Die Abnahme der Oberfläche der Freigabe-App mit Sprache, Meldung und Zeile „Agent“ steht nur im Pull Request und im Issue ({{ ref("G20") }}, {{ ref("G24") }}).

Drei Entscheidungen warten auf die Bestätigung durch Stefan.
Jede Bestätigung schließt Gaps:

| Entscheidung | Schließt nach der Bestätigung |
|---|---|
| {{ ref("D52") }} | {{ ref("G30") }}, {{ ref("G23") }}; {{ ref("G25") }}, wenn kein Attribut nötig ist |
| {{ ref("D51") }} | engt {{ ref("G3") }} weiter ein |
| {{ ref("D49") }} | Teil von {{ ref("G2") }} |
