---
title: "Security Boundaries"
description: "Zonen, Grenzen und Schutzmaßnahmen von Passierschein je Stufe: was a38 erzwingt, was ein anderer Dienst erzwingt und was niemand leistet."
render_macros: true
weight: 5
tags:
  - Thema/Sicherheit
---
# Security Boundaries

Diese Seite ist das Bedrohungsmodell von Passierschein
({{ ref("D20") }}). Sie trägt keine eigene Nummer. Sie nennt je
Schutzmaßnahme eine Klasse:

| Klasse | Bedeutung |
|---|---|
| **enforced** | `a38` erzwingt die Maßnahme technisch. Ein Testfall belegt sie. |
| **inherited** | Ein anderer Dienst erzwingt die Maßnahme. `a38` verlässt sich darauf. |
| **not-provided** | Niemand leistet die Maßnahme. Der Grund steht dabei. |

**Stand:** Designlauf 1, von Stefan am 2026-11-11 im Review abgenommen.

## Zwei Rahmenfakten

1. **Die Stufen 1 bis 4 isolieren nicht.** Der Agent läuft in diesen Stufen
   unter demselben macOS-Benutzer wie der Daemon. Ein Prozess dieses
   Benutzers kann den Daemon, seine Dateien und 1Password direkt angreifen.
   `a38` verbessert in diesen Stufen die Nachvollziehbarkeit. `a38` ist in
   diesen Stufen keine Sicherheitsgrenze ({{ ref("RI5") }}).
2. **Ein ausgeliefertes dev-Secret ist nicht mehr geheim.** Der Wert steht
   danach im Kontext des Modells und im Transkript. Stefan hat das für
   dev-Secrets akzeptiert, weil sie rotierbar sind ({{ ref("D9") }},
   {{ ref("RI1") }}).

## Schutzgüter

| Schutzgut | Schutzziel |
|---|---|
| Secret-Werte außerhalb von dev | Vertraulichkeit: Der Wert erreicht nie den Agent als MCP-Antwort. |
| Secret-Werte in dev | Nachvollziehbarkeit: Jede Auslieferung hat eine Freigabe und einen Eintrag im Audit-Log. |
| Zugriff auf 1Password | Nur der Daemon nutzt den Zugriff. |
| Freigabe des Menschen | Authentizität: Nur der Mensch gibt frei, mit Touch ID. |
| Audit-Log | Integrität: Eine Änderung ist erkennbar. |
| Konfiguration und Policies | Integrität: Nur der Mensch ändert sie. |

## Zonen

| Zone | Inhalt | Vertrauen |
|---|---|---|
| Z1 Mensch | Stefan an Bildschirm und Touch ID | vertraut |
| Z2 Daemon | Prozess `a38 serve` auf dem Mac | vertraut; einziger Bestandteil mit Zugriff auf 1Password |
| Z3 Freigabe-App | Swift-App, vom Daemon gestartet | vertraut für die Anzeige; erhält keine Secrets |
| Z4 1Password | Dienst und Desktop-App von 1Password | vertraut, extern |
| Z5 Agent | Coding Agent, seine Werkzeuge und alle Prozesse, die er startet; dazu `a38 mcp` | **nicht vertraut** |
| Z6 Dateien des Daemons | Audit-Datenbank, Konfiguration, Sockets | vertraut nur so weit, wie das Dateisystem sie schützt |
| Z7 Außenwelt | Modell-Anbieter, Transkripte, Sitzungsarchive | außerhalb der Kontrolle |

`a38 mcp` gehört zur Zone Z5. Der Prozess läuft dort, wo der Agent läuft.
Der Daemon vertraut `a38 mcp` deshalb nicht mehr als dem Agent
({{ ref("D2") }}).

## Grenzen

| Grenze | Zwischen | Transport | Was die Grenze überquert |
|---|---|---|---|
| B1 Agent-Socket | Z5 → Z2 | Unix-Socket ({{ ref("D3") }}) | Anfragen des Agents; Antworten mit dev-Werten |
| B2 Freigabe | Z2 ↔ Z3 | stdin und stdout der Freigabe-App ({{ ref("D6") }}) | Freigabeanfrage ohne Secret-Werte; Entscheidung |
| B3 Backend | Z2 → Z4 | Go-SDK von 1Password ({{ ref("D1") }}) | Namen von Vaults und Items; Secret-Werte |
| B4 Zugang aus der VM | VM → Mac | offen (Stufe 5): SSH-`RemoteForward`, eigener Port-Forward, Tailscale oder Host-Anbindung der VM ({{ ref("G1") }}) | dieselben Nachrichten wie B1 |
| B5 Admin-Socket | Z1 → Z2 | zweiter Unix-Socket ({{ ref("D44") }}) | Widerruf, Liste aller Grants |
| B6 Dateisystem | Z2 ↔ Z6 | Dateien des Benutzers | Audit-Datenbank, Konfiguration |

## Maßnahmen

### B1 Agent-Socket

| Maßnahme | Klasse | Mechanismus | Nachweis |
|---|---|---|---|
| Nur der eigene Benutzer erreicht den Socket. | enforced | Datei `0600` in einem Verzeichnis `0700` ({{ ref("D43") }}) | {{ ref("T12") }} |
| Die Nachrichten folgen einem festen Schema. | enforced | versioniertes Schema, Größengrenze, Ablehnung unbekannter Felder ({{ ref("D26") }}) | {{ ref("T10") }} |
| Die Sitzung und die Herkunft bestimmt der Daemon. | enforced | Peer-Daten des Sockets, Sitzungs-ID vom Daemon ({{ ref("D25") }}) | {{ ref("T11") }} |
| Die Angaben des Agents ändern die geprüften Fakten nicht. | enforced | getrennte Felder, getrennte Anzeige ({{ ref("D4") }}) | {{ ref("T11") }} |
| Der Name der Sitzung steht nur bei den Angaben des Agents. | enforced | Startwert von `a38 mcp`, getrenntes Feld ({{ ref("D25") }}) | {{ ref("T21") }} |
| Der Daemon prüft den Namen der Sitzung. | **not-provided** | Der Name kommt aus der Zone des Agents ({{ ref("G19") }}). | – |
| Der Daemon bündelt die Anfragen. | enforced | Bündelfenster im Daemon ({{ ref("D5") }}, {{ ref("D29") }}) | {{ ref("T2") }}, {{ ref("T19") }} |
| Eine Anfrage außerhalb der Allowlist erreicht den Menschen nicht. | enforced | Policy-Prüfung vor der Freigabe ({{ ref("D28") }}) | {{ ref("T3") }} |
| Eine Anfrage außerhalb von dev liefert im MVP keinen Wert. | enforced | Ablehnung ohne Freigabeanfrage ({{ ref("D24") }}) | {{ ref("T3") }} |
| Admin-Befehle sind auf dem Agent-Socket nicht möglich. | enforced | eigener Admin-Socket ({{ ref("D44") }}) | {{ ref("T12") }} |
| Der Daemon unterscheidet lokale Prozesse desselben Benutzers verlässlich. | **not-provided** | Ein Prozess desselben Benutzers kann sich als anderer Prozess ausgeben. Die Peer-Daten sind ein Hinweis, kein Beweis ({{ ref("RI5") }}). | – |
| Der Daemon unterscheidet Sitzungen in der VM. | **not-provided** (bis Stufe 5) | offen ({{ ref("G1") }}) | – |

### B2 Freigabe

| Maßnahme | Klasse | Mechanismus | Nachweis |
|---|---|---|---|
| Die Freigabe-App erhält keinen Secret-Wert. | enforced | Die Freigabeanfrage enthält nur Namen ({{ ref("D6") }}, {{ ref("D34") }}). | {{ ref("T8") }} |
| Nur Touch ID bestätigt eine Freigabe. | enforced | `LocalAuthentication`, kein Standard-Button, Verzögerung ({{ ref("D7") }}, {{ ref("D31") }}) | {{ ref("T14") }} |
| Ein Fehler, eine Ablehnung oder eine Zeitüberschreitung gibt nichts frei. | enforced | fail closed ({{ ref("D30") }}, {{ ref("D35") }}) | {{ ref("T6") }} |
| Eine gefälschte Freigabe ist mit dem echten Backend unmöglich. | enforced | Der Daemon startet mit dem 1Password-Backend nur mit der Freigabe-App ({{ ref("D40") }}). | {{ ref("T9") }} |
| Der Daemon startet nur die echte Freigabe-App. | **not-provided** im MVP | Ein Prozess desselben Benutzers kann die App-Datei ersetzen. Die Prüfung der Signatur vor dem Start ist offen ({{ ref("G5") }}). | – |
| Ein anderer Prozess kann kein täuschend echtes Popup zeigen. | **not-provided** | macOS verhindert das nicht. Die Touch-ID-Abfrage nennt die App ({{ ref("RI6") }}). | – |

### B3 Backend

| Maßnahme | Klasse | Mechanismus | Nachweis |
|---|---|---|---|
| Nur der Daemon enthält Code für 1Password. | enforced | Das Backend liegt hinter einem Build-Tag. Der linux-Build enthält es nicht ({{ ref("D42") }}). | {{ ref("T13") }} |
| Der Daemon liest nur Vaults aus der Konfiguration und aus der Allowlist des Projekts. | enforced | Liste der Vaults mit Umgebung ({{ ref("D12") }}, {{ ref("D47") }}), Policy je Projekt ({{ ref("D28") }}) | {{ ref("T3") }}, {{ ref("T22") }} |
| Der Zugang ist auf die dev-Vaults begrenzt. | inherited | Rechte des Service Accounts in 1Password ({{ ref("D39") }}) | {{ ref("T15") }} |
| Der Daemon hält keinen Secret-Wert länger als für die Auslieferung. | enforced | kein Cache, kein Wert in Log oder Audit-Log ({{ ref("D34") }}) | {{ ref("T8") }} |
| Das Token des Service Accounts ist vor anderen Prozessen des Benutzers geschützt. | **not-provided** | Ort und Schutz des Tokens sind offen ({{ ref("G3") }}). | – |

### B4 Zugang aus der VM (Stufe 5)

Der Transport ist offen ({{ ref("G1") }}). Die Klassen dieser Tabelle gelten erst, wenn der Transport entschieden ist.

| Maßnahme | Klasse | Mechanismus | Nachweis |
|---|---|---|---|
| Die VM enthält keine Werkzeuge von 1Password. | inherited | Aufbau der VM ({{ ref("D2") }}) | – |
| Nur die vorgesehene VM erreicht den Agent-Endpunkt. | inherited, Mechanismus offen | je Transport: SSH-Authentifizierung, Tailscale-Identität und ACL, oder Netz der VM auf dem Host | – |
| Ein Netzwerk-Endpunkt ist vor anderen Geräten und Prozessen geschützt. | **not-provided** | Nur der Unix-Socket hat Dateirechte. Jeder andere Transport braucht eine eigene Authentifizierung ({{ ref("G1") }}). | – |
| Die VM erreicht nur den Agent-Endpunkt, nie den Admin-Socket. | enforced | Der Admin-Socket ist ein eigener Pfad ({{ ref("D44") }}). | {{ ref("T12") }} |
| Der Daemon kennt die VM und die Sitzung. | **not-provided** | offen ({{ ref("G1") }}) | – |

### B6 Dateisystem

| Maßnahme | Klasse | Mechanismus | Nachweis |
|---|---|---|---|
| Eine Änderung am Audit-Log ist erkennbar. | enforced | Hash-Kette und `a38 audit verify` ({{ ref("D11") }}, {{ ref("D36") }}) | {{ ref("T7") }} |
| Ohne Eintrag im Audit-Log gibt es keine Auslieferung. | enforced | Eintrag vor der Antwort, Code `audit_failed`, Meldung an den Menschen ({{ ref("D36") }}) | {{ ref("T26") }} |
| Ein Daemon mit einer Störung liefert nichts aus. | enforced | globale Sperre, Code `daemon_faulted`, Popup ({{ ref("D48") }}) | {{ ref("T28") }} |
| Eine Änderung am Audit-Log ist unmöglich. | **not-provided** | Die Datei gehört dem Benutzer. Wer die Datei ändern kann, kann die Kette neu berechnen ({{ ref("D37") }}, {{ ref("RI7") }}). | – |
| Nur der Mensch ändert die Konfiguration. | **not-provided** in den Stufen 1 bis 4 | Die Datei gehört dem Benutzer ({{ ref("RI5") }}). Ab Stufe 5 liegt sie außerhalb der VM. | – |

### Z7 Außenwelt

| Maßnahme | Klasse | Mechanismus | Nachweis |
|---|---|---|---|
| Ein dev-Secret erreicht nicht den Modell-Anbieter oder ein Transkript. | **not-provided**, akzeptiert | Der Wert ist die MCP-Antwort ({{ ref("D9") }}, {{ ref("RI1") }}). | – |
| Ein ausgeliefertes Secret lässt sich zurückholen. | **not-provided** | Die Abhilfe ist die Rotation in 1Password ({{ ref("RI11") }}). | – |
| Der Agent umgeht `a38` nicht. | **not-provided** in den Stufen 1 bis 4 | Der Hook ist ein Hinweis, kein Schutz ({{ ref("D15") }}). Ab Stufe 5 fehlt das Ziel in der VM. | – |

## Was ausdrücklich außerhalb liegt

- Der Git-Zugriff der Agents ({{ ref("D13") }}).
- Der Zugang zum Cluster ({{ ref("D14") }}).
- Die Automation für Analysen und Health Checks ({{ ref("D21") }}).
