---
title: Architektur
description: "Prozesse, Pakete, Abhängigkeitsregel, Protokoll, Konfiguration und Teststrategie von a38 in Stufe 1."
render_macros: true
---
# Architektur

Diese Seite beschreibt den Aufbau des Codes von `a38` für die Stufe 1.
Die verbindlichen Aussagen stehen im [Design-Set](../design/index.md).
Diese Seite zeigt, wie der Code das Design umsetzt und wo welche Aufgabe liegt.

Das Ziel des Aufbaus: Mehrere Entwickler bauen die Teile gleichzeitig.
Kein Teil erfindet einen gemeinsamen Typ selbst.
Kein Entwickler ändert die Datei eines anderen Teils.

## Bestandteile und Prozesse

`a38` ist ein Binary mit Unterbefehlen ({{ ref("D42") }}).
Drei Arten von Prozessen entstehen daraus.

| Prozess | Zone | Aufgabe |
|---|---|---|
| `a38 serve` | Daemon | Der Prozess stellt den Agent-Socket und den Admin-Socket bereit. Er prüft die Policy, bündelt, fragt die Freigabe an, vergibt Grants, schreibt das Audit-Log und liefert aus. Nur dieser Prozess spricht mit dem Backend ({{ ref("D1") }}). |
| `a38 mcp` | Agent | Der Prozess ist ein MCP-Server über stdio (MCP: Model Context Protocol). Das Agent-Werkzeug startet ihn je Sitzung. Er übersetzt die zwei MCP-Werkzeuge in Nachrichten für den Agent-Socket ({{ ref("D41") }}). Er verbindet sich erst beim ersten Aufruf eines Werkzeugs mit dem Daemon und sendet dann den Namen der Sitzung und den Namen des MCP-Clients in `hello`. Ist der Daemon nicht erreichbar, antwortet er mit `daemon_unavailable` und versucht es beim nächsten Aufruf erneut. Er enthält keinen Code für ein Backend. |
| Admin-Befehle | Mensch | `a38 grants`, `a38 fault` und `a38 doctor` sprechen mit dem Daemon über den Admin-Socket ({{ ref("D44") }}). `a38 audit` liest die Audit-Datei direkt. `a38 llm` und `a38 version` brauchen keinen Daemon. |

Die Freigabe-UI ist in Stufe 1 kein eigener Prozess der Auslieferung.
Die Varianten `auto-approve` und `auto-deny` laufen im Daemon.
Die Variante `script` startet ein Skript als Kindprozess ({{ ref("D40") }}).
Die Freigabe-App in Swift folgt in Stufe 2 hinter derselben Schnittstelle.

Die Unterbefehle `tui` und `service` aus {{ ref("D42") }} gehören zu späteren Stufen.
Das Binary kennt sie noch nicht.

## Pakete

Jede Zeile nennt ein Paket, seinen Inhalt und das Issue, dem das Paket gehört.
„Fertig“ heißt: Dieses Issue liefert das Paket vollständig mit Tests.
„Schnittstelle“ heißt: Dieses Issue liefert nur Typen und Schnittstellen.

### Vorhandene Pakete

| Paket | Inhalt | Stand | Issue |
|---|---|---|---|
| `cmd/a38` | Liste der Unterbefehle in `main.go`. Jeder Unterbefehl hat eine eigene Datei mit genau einer Funktion. | Stubs | 16 |
| `internal/protocol` | Nachrichten beider Sockets, strenge Dekodierung, Fehlercodes, Referenz, Behälter für Secret-Werte | fertig | 16 |
| `internal/config` | Konfigurationsdatei: Struktur, Laden, Prüfen, Standardpfade | fertig | 16 |
| `internal/clock` | Uhr als Schnittstelle, mit echter und gefälschter Uhr | fertig | 16 |
| `internal/session` | geprüfte Fakten und Angaben des Agents als Typen | fertig | 16 |
| `internal/backend` | Schnittstelle `Backend`, Fehlerzustände, Zuordnung zu den Fehlercodes | Schnittstelle | 16 |
| `internal/approval` | Freigabeanfrage, Entscheidung, Meldung, Schnittstelle `UI` | Schnittstelle | 16 |
| `internal/audit` | Ereignisarten, Eintrag, Filter, Schnittstelle `Store` | Schnittstelle | 16 |
| `internal/daemon` | Naht zwischen Socket-Schicht und Broker: `AgentHandler`, `AdminHandler` | Schnittstelle | 16 |
| `internal/version` | Version des Binarys | fertig | – |

### Pakete der Folge-Issues

Die Namen dieser Pakete sind ein Vorschlag.
Der Besitzer eines Issues darf sein Paket anders schneiden.
Die Abhängigkeitsregel gilt in jedem Fall.

| Paket | Inhalt | Issue |
|---|---|---|
| `internal/audit/sqlite` | `Store` mit SQLite und Hash-Kette; `cmd/a38/audit.go` | 17 |
| `internal/backend/fake` | `Backend` aus einer YAML-Datei, mit Zählern und den Zuständen „gesperrt“ und „nicht erreichbar“ | 18 |
| `internal/approval/fakeui` | `UI` für `auto-approve`, `auto-deny` und `script` | 19 |
| `internal/approval/queue` | Warteschlange und Wartezeit: höchstens eine Freigabeanfrage zur gleichen Zeit | 19 |
| `internal/daemon` | Sockets, Rechte, Peer-Daten, Sitzungen, Namenskonflikt, Start und Ende; `cmd/a38/serve.go` | 20 |
| `internal/client` | Verbindung zu einem Socket des Daemons: `hello`, Anfrage senden, Antwort über die Kennung zuordnen. `internal/mcp` und die Admin-Befehle nutzen das Paket. | 20 |
| `internal/policy` | Prüfung einer Anfrage gegen die Konfiguration | 21 |
| `internal/bundle` | Bündelfenster und Obergrenze je Sitzung | 21 |
| `internal/grant` | Grants im Arbeitsspeicher: Vergabe, Prüfung, Ablauf, Widerruf | 22 |
| `internal/broker` | Ablauf einer Anfrage von der Policy bis zur Auslieferung; setzt `AgentHandler` und `AdminHandler` um | 22 |
| `internal/fault` | globale Sperre bei einer Störung, Meldung an den Menschen, Einträge im Audit-Log, Aufheben | 23 |
| `internal/mcp` | MCP-Server und Client für den Agent-Socket; `cmd/a38/mcp.go` | 24 |
| Admin-Befehle | `cmd/a38/admin.go`, `grants.go` und `fault.go` nutzen `internal/client` auf dem Admin-Socket; es gibt kein Paket `internal/admin` | 25 |
| `internal/doctor` | Prüfungen von `a38 doctor`; `cmd/a38/doctor.go` | 25 |
| `internal/skill` | eingebettete Anleitung für Agents; `cmd/a38/llm.go` | 26 |

Vier Pakete aus dem ersten Entwurf entstehen in diesem Issue nicht: `internal/grant`, `internal/policy`, `internal/bundle` und ein Sitzungsregister.
Kein anderes Paket braucht heute einen Typ aus ihnen.
Die Liste der Grants auf dem Socket ist ein Typ des Protokolls (`protocol.Grant`).
Das Sitzungsregister gehört zur Socket-Schicht.
`internal/session` enthält deshalb nur die Datentypen.

### Naht im Daemon

Der Daemon hat zwei Hälften.
Die Socket-Schicht nimmt Verbindungen an und prüft Nachrichten (Issue 20).
Der Broker entscheidet über eine Anfrage (Issues 21 bis 23).
Die zwei Hälften kennen einander nur über die Schnittstellen `AgentHandler` und `AdminHandler` im Paket `internal/daemon`.

- Die Socket-Schicht ruft den Broker nur mit einer gültigen Nachricht einer angemeldeten Sitzung auf.
- Die Socket-Schicht behandelt den Namenskonflikt zweier Sitzungen selbst ({{ ref("D25") }}). Der Broker sieht eine Sitzung im Konflikt nicht.
- Der Broker gibt eine Antwort oder einen `*protocol.Error` zurück. Der Typ erzwingt einen stabilen Code für jeden Fehlerweg ({{ ref("D35") }}).
- Die Socket-Schicht meldet dem Broker das Ende einer Sitzung. Der Broker beendet dann die Grants der Sitzung ({{ ref("D33") }}).

Ein Test der Socket-Schicht nutzt einen gefälschten Broker.
Ein Test des Brokers braucht keinen Socket.

### Socket-Schicht

Die Socket-Schicht setzt {{ ref("D43") }}, {{ ref("D25") }} und {{ ref("D44") }} so um:

- **Start:** Der Daemon legt das Verzeichnis der Sockets mit `0700` an. Hat das Verzeichnis andere Rechte, gehört es einem anderen Benutzer oder ist es ein symbolischer Link, startet der Daemon nicht. Der Daemon korrigiert die Rechte nicht.
- **Ein Daemon je Verzeichnis:** Der Daemon sperrt das Verzeichnis mit `flock`. Ein zweiter Daemon mit demselben Verzeichnis startet nicht.
- **Alter Socket:** Der Daemon entfernt einen Socket nur, wenn kein Prozess eine Verbindung annimmt. Eine Datei, die kein Socket ist, entfernt er nie.
- **Peer-Daten:** Der Kernel liefert Benutzer und Prozess-ID. Unter macOS liest der Daemon Programmpfad und Arbeitsverzeichnis mit dem Systemaufruf `proc_info`, unter Linux aus `/proc`. Beides geht ohne cgo. Ein Wert, den der Daemon nicht lesen kann, bleibt leer. Ohne Arbeitsverzeichnis hat die Sitzung kein Projekt.
- **Benutzer:** Der Daemon bedient nur Prozesse desselben Benutzers. Er schließt jede andere Verbindung ohne Antwort.
- **Anmeldung:** Die erste Nachricht ist `hello`. Jede andere erste Nachricht und jeder ungültige Name enden mit `invalid_request` und dem Ende der Verbindung.
- **Ungültige Nachricht:** Eine ungültige Zeile endet mit `invalid_request`. Die Verbindung bleibt bestehen. Eine Zeile über der Größengrenze beendet die Verbindung nach der Fehlermeldung.
- **Gleichzeitige Anfragen:** Eine Verbindung hat höchstens 64 offene Anfragen. Danach liest der Daemon die nächste Nachricht erst, wenn eine Anfrage endet.
- **Namenskonflikt:** Die Socket-Schicht prüft den Konflikt vor und nach dem Aufruf des Brokers. Entsteht der Konflikt während einer Anfrage, endet auch diese Anfrage mit `session_name_conflict`.
- **Fehler des Brokers:** Gibt der Broker weder Antwort noch Fehler zurück, beides, einen unbekannten Code oder eine ungültige Antwort, oder bricht er ab (`panic`), endet die Anfrage mit `daemon_faulted`. Die globale Sperre aus {{ ref("D48") }} setzt der Broker.
- **Ende:** Beim Ende des Daemons schließt die Socket-Schicht beide Sockets und entfernt die Dateien. Sie schließt jede Verbindung und beendet den Kontext jeder offenen Anfrage. Für jede Sitzung ruft sie `SessionEnded` genau einmal auf.

## Abhängigkeitsregel

Ein Paket importiert nur Pakete, die in dieser Tabelle in seiner Zeile stehen.
Die Standardbibliothek ist immer erlaubt.

| Paket | darf importieren |
|---|---|
| `protocol`, `clock`, `version` | kein Paket von `a38` |
| `config` | `protocol` |
| `session` | kein Paket von `a38` |
| `backend` | `protocol` |
| `approval` | `protocol`, `config`, `session` |
| `audit` | `protocol`, `session` |
| Umsetzung einer Schnittstelle (`backend/fake`, `audit/sqlite`, `approval/fakeui`, `approval/queue`) | das eigene Schnittstellenpaket und dessen erlaubte Pakete, dazu `clock` |
| `policy`, `bundle`, `grant`, `fault` | `protocol`, `config`, `session`, `clock`, dazu die Schnittstellenpakete `approval` und `audit` |
| `broker` | alle Pakete oberhalb dieser Zeile, aber nur die Schnittstellenpakete und keine Umsetzung |
| `daemon` | alle Pakete oberhalb dieser Zeile; nur `daemon` wählt die Umsetzungen aus |
| `client` | `protocol` |
| `mcp` | `protocol`, `config`, `version`, `client` |
| `doctor` | `protocol`, `config`, `client`, `audit`, `audit/sqlite` (nur lesend) |
| `skill` | `protocol` |
| `cmd/a38` | jedes Paket |

Vier Regeln folgen aus der Tabelle:

1. `protocol` ist das gemeinsame Vokabular. Es importiert nichts von `a38`.
2. Kein Paket importiert eine Umsetzung eines anderen Pakets. Nur `daemon` und `cmd/a38` verdrahten Umsetzungen.
3. `mcp` importiert weder `backend` noch `daemon`. Der linux-Build von `a38 mcp` enthält so keinen Code für 1Password ({{ ref("D2") }}, {{ ref("T13") }}).
4. Kein Paket ruft `time.Now`, `time.After` oder `time.NewTimer` direkt auf. Es erhält eine `clock.Clock`.

## Datenfluss einer Anfrage

Das Diagramm zeigt {{ ref("UC1") }} mit den Paketen.
Der Agent ruft das MCP-Werkzeug `request_credentials` in `a38 mcp` auf.
Die Notizen nennen die Einträge im Audit-Log.

```mermaid
sequenceDiagram
    autonumber
    participant M as a38 mcp
    participant S as a38 serve
    participant U as Freigabe-UI
    participant B as Backend
    M->>S: hello mit<br/>Name der Sitzung
    S-->>M: welcome mit<br/>Kennung der Sitzung
    M->>S: request_credentials<br/>mit Referenzen
    Note over S: Audit: Anfrage
    S->>S: Policy prüfen<br/>(config, policy)
    S->>B: Resolve:<br/>Namen auflösen
    B-->>S: Namen,<br/>kein Wert
    S->>S: Bündelfenster<br/>abwarten (bundle)
    Note over S: Audit: Bündel
    S->>U: Approve:<br/>Freigabeanfrage
    U-->>S: Entscheidung<br/>mit Auswahl
    Note over S: Audit: Entscheidung
    S->>S: Grant vergeben<br/>(grant)
    Note over S: Audit: Grant,<br/>Auslieferung
    S->>B: Read:<br/>Werte lesen
    B-->>S: Werte
    S-->>M: credentials<br/>mit Werten
```

Vier Punkte sind für den Code wichtig:

- Die Policy-Prüfung steht vor dem Backend. Eine Anfrage außerhalb der Policy erreicht weder das Backend noch den Menschen ({{ ref("D28") }}, {{ ref("T3") }}).
- Der Eintrag für die Auslieferung steht vor dem Lesen der Werte ({{ ref("D36") }}). Schlägt `Append` fehl, ruft der Broker `Read` nicht auf ({{ ref("T26") }}).
- Der Broker hält einen Wert nur für die Antwort ({{ ref("D34") }}). Der Typ `protocol.Secret` gibt den Wert in keiner Textausgabe aus.
- Der Broker löst die Namen gleich nach der Policy auf, noch vor dem Bündelfenster. So endet ein unbekanntes Item sofort mit `not_found` und belegt keinen Platz im Bündel ({{ ref("D27") }}).
- Deckt ein Grant die Anfrage ab, entfallen Bündelfenster, Freigabe und Grant ({{ ref("UC2") }}). Die Auflösung der Namen bleibt: Der Grant gilt für die Namen des Backends.

## Broker

Die Issues 21 und 22 setzen den Broker um, Issue 23 die globale Sperre.
Fünf Pakete teilen sich die Arbeit:

| Paket | Inhalt |
|---|---|
| `internal/policy` | Prüfung einer Anfrage gegen die Konfiguration, alles oder nichts |
| `internal/bundle` | Bündelfenster mit festem Ende, Obergrenze, Reihenfolge des Eingangs |
| `internal/grant` | Grants im Arbeitsspeicher: Abdeckung, Ablauf, Ende mit der Sitzung, Widerruf |
| `internal/fault` | globale Sperre `Global` hinter den Schnittstellen `Gate` und `Lock`; `NoLock` sperrt nie und dient Tests |
| `internal/broker` | Ablauf einer Anfrage; setzt `AgentHandler` und `AdminHandler` um |

`policy`, `bundle` und `grant` sind reine Datenstrukturen.
Sie sind nicht nebenläufig sicher.
Der Broker hält seine Sperre, wenn er sie nutzt.

### Ablauf und Invarianten

1. Die globale Sperre ist offen. Sonst endet die Anfrage mit `daemon_faulted`.
2. Die Anfrage ist gültig. Der Broker schreibt das Ereignis „Anfrage“.
3. Die Policy erlaubt jede Referenz. Sonst endet die ganze Anfrage mit `not_allowed` oder `config_invalid`.
4. Das Backend löst jede Referenz auf. Ein Fehler beendet die ganze Anfrage.
5. Ein Grant der Sitzung deckt die Referenz ab, oder die Referenz kommt in das Bündel der Sitzung.
6. Am Ende des Fensters entsteht genau eine Freigabeanfrage. Der Broker prüft die Entscheidung selbst.
7. Eine Freigabe erzeugt einen Grant für die angehakten Einträge.
8. Der Broker prüft unter seiner Sperre den Grant und schreibt das Ereignis „Auslieferung“. Erst danach liest er die Werte.

Drei Regeln gelten auf jedem Weg:

- Ein Wert verlässt den Broker nur für eine Referenz, die ein lebender Grant derselben Sitzung im Moment der Auslieferung abdeckt.
- Ein Grant existiert erst, wenn sein Ereignis im Audit-Log steht.
- Ein Fehler der ganzen Anfrage liefert keinen Wert, auch nicht für eine Referenz unter einem Grant.

Eine beendete Sitzung kommt nie zurück.
Der Broker merkt sich ihre Kennung eine Stunde lang.
Eine Anfrage, die beim Ende der Sitzung noch läuft, öffnet kein Bündel, erreicht den Menschen nicht und schreibt kein Ereignis „Auslieferung“ ({{ ref("D33") }}).
Der Broker baut eine Freigabeanfrage erst, wenn sie an der Reihe ist.
Einträge, Abweichung und bestehender Grant zeigen so den Stand zur Zeit der Anzeige.
Ein Absturz (Panic) im Broker ist eine Störung des Daemons: Der Broker meldet sie an die Sperre, und die Anfrage endet mit `daemon_faulted` ({{ ref("G21") }}). Das gilt beim Entscheiden eines Bündels und in jeder Methode von `AgentHandler` und `AdminHandler`.

Der Broker schreibt ein Ereignis, das Grants ändert oder nutzt, unter seiner Sperre.
Die Reihenfolge im Audit-Log ist so die Reihenfolge der Änderungen.
Ein Widerruf steht vor einer Auslieferung, oder die Auslieferung steht vor dem Widerruf.

### Ereignisse je Ausgang

| Ausgang | Ereignisse | Code |
|---|---|---|
| Freigabe | Anfrage, Bündel, Entscheidung, Grant, Auslieferung | – |
| Teilauswahl | Anfrage, Bündel, Entscheidung, Grant, Ablehnung, Auslieferung | `denied` je nicht angehaktem Eintrag |
| Grant deckt ab | Anfrage, Auslieferung mit dem Grant | – |
| Policy | Anfrage, Ablehnung | `not_allowed`, `config_invalid` |
| Unbekanntes Item oder Feld | Anfrage, Ablehnung | `not_found` |
| Bündel voll | Ablehnung der überzähligen Einträge | `bundle_too_large` |
| Ablehnung durch den Menschen | Anfrage, Bündel, Ablehnung | `denied` |
| Wartezeit abgelaufen | Anfrage, Bündel, Ablauf | `timeout` |
| Fehler der Freigabe | Anfrage, Bündel, Fehler | `ui_failed` |
| Fehler des Backends | Anfrage, Fehler | `backend_locked`, `backend_unavailable` |
| Sitzung endet während der Freigabe | Ablauf mit `session_ended` | – |
| Verbindung endet während eines Aufrufs des Backends | Ablauf mit `session_ended`, kein Fehler des Backends | – |
| Interne Störung im Broker | Fehler, Beginn der Sperre | `daemon_faulted` |
| Fehler beim Schreiben des Audit-Logs | Beginn der Sperre, sobald das Schreiben möglich ist | `audit_failed` |
| Anfrage während der Sperre | Fehler, wenn das Schreiben möglich ist | `daemon_faulted` |
| Ende eines Grants | Ablauf mit `lifetime_ended` oder `session_ended`, Widerruf | – |

### Was der Daemon verdrahtet

- `broker.New(cfg, backend, ui, auditStore, clock, logger)` erstellt den Broker.
- Die UI ist eine `approval/queue.Queue`. Sie setzt die Wartezeit von {{ ref("D30") }} durch. Der Broker lässt zusätzlich nur eine Freigabeanfrage zur gleichen Zeit zu.
- `Broker.Run(ctx)` entfernt abgelaufene Grants alle 30 Sekunden und schreibt ihr Ereignis. Ohne `Run` gilt ein Grant trotzdem nie über seine Laufzeit hinaus; das Ereignis entsteht dann bei der nächsten Anfrage.
- `a38 serve` verdrahtet den Broker. `--fake-file` nennt die YAML-Datei des Fake-Backends. `--ui-script` nennt das Programm der Variante `script` mit seinen Argumenten, getrennt durch Leerzeichen. Ohne Backend oder Freigabe startet der Daemon nicht und legt keinen Socket an. Eine Audit-Datei, die der Daemon nicht öffnen oder nicht schreiben kann, verhindert den Start nicht: Der Daemon startet gesperrt (siehe [Globale Sperre](#globale-sperre)).
- `broker.WithUIMode` nennt die Variante der Freigabe. Bei `auto-approve`, `auto-deny` und `script` darf eine Entscheidung ohne `auth` sein. Bei `app`, bei einer unbekannten Variante und ohne die Option verlangt der Broker eine bestandene Authentifizierung des Menschen; fehlt `auth`, ist sie gescheitert oder steht dort `skipped`, endet die Freigabe mit `ui_failed` ({{ ref("D7") }}).
- `broker.WithLock` setzt die globale Sperre. `a38 serve` übergibt `fault.Global`. Ohne die Option gilt `fault.NoLock`. Der Broker ruft `Lock.Raise` bei jedem Fehler des Audit-Logs und bei jedem Absturz auf.
- `Broker.Status` liefert für `a38 doctor` die Sperre mit Ursache, Beginn und dem Ergebnis der Prüfung der Ursache, die ungültigen Projekte, den Zustand des Backends (`ok`, `locked`, `unreachable`) und die Variante der Freigabe. Den Zustand des Backends fragt der Broker mit `Backend.Vaults` ab; er liest dabei keinen Wert.

### Globale Sperre

`fault.Global` setzt {{ ref("D48") }} um.

| Ereignis | Verhalten |
|---|---|
| Störung | Ein Fehler beim Schreiben des Audit-Logs auf jedem Weg (auch Ablauf, Ende einer Sitzung und Widerruf) oder ein Absturz im Broker ruft `Raise` auf. Die Anfrage mit dem Fehler endet mit `audit_failed` oder `daemon_faulted`. |
| Während der Sperre | Jede Anfrage jeder Sitzung endet mit `daemon_faulted`, auch unter einem bestehenden Grant. Sie erreicht weder das Backend noch den Menschen. Es entsteht kein Grant. |
| Meldung | Genau eine Meldung `approval.Notice` der Art `daemon_fault` je Störung, nicht eine je Anfrage. Sie nennt die Störung, die Sperre und die nächsten Schritte: `a38 doctor`, dann `a38 fault clear` oder ein Neustart. Sie enthält keinen Wert. Die Meldung läuft in einer eigenen Goroutine; eine langsame Oberfläche hält den Daemon nicht auf. |
| Audit-Log | Beginn und Ende sind Einträge der Art „Fehler“ mit `fault_begin` und `fault_end` ({{ ref("G22") }}). Ist das Audit-Log selbst die Ursache, bleibt der Beginn offen. Das Log des Daemons nennt das. `Global.Run` versucht den Eintrag alle 30 Sekunden erneut. Spätestens beim Aufheben steht er vor dem Ende. |
| Admin-Befehle | `grants_list`, `grant_revoke`, `status` und `fault_clear` arbeiten während der Sperre. Auf dem Agent-Socket endet `fault_clear` mit `invalid_request`. |
| Aufheben | Nur `fault_clear` über den Admin-Socket (`a38 fault clear`) oder ein Neustart hebt die Sperre auf. Verschwindet die Ursache von selbst, bleibt die Sperre. |
| Prüfung der Ursache | Der Daemon prüft das Audit-Log mit einem Probe-Schreibvorgang: Er schreibt eine Zeile in einer Transaktion und nimmt sie zurück. Danach schreibt er den offenen Beginn und das Ende. Scheitert ein Schritt, bleibt die Sperre, und die Antwort nennt den Befund. Ein Absturz hat keine eigene Prüfung; nach dem Absturz gilt dieselbe Prüfung des Audit-Logs. |
| Status | `status_result.fault` nennt Code, Grund und Beginn. `check_passed` und `finding` zeigen die Prüfung der Ursache, ohne Schreibvorgang im Audit-Log. `begin_pending` zeigt einen offenen Beginn. |
| Start | Der Daemon prüft beim Start das Audit-Log mit demselben Probe-Schreibvorgang. Scheitert die Prüfung, startet er gesperrt und legt beide Sockets an. So erreichen `a38 doctor` und die Admin-Befehle den Daemon. Ein Neustart bei bestehender Ursache führt so wieder in die Sperre ({{ ref("T28") }}). |

Keine Störung des Daemons sind: die fehlerhafte Konfiguration eines Projekts (`config_invalid`, nur dieses Projekt, {{ ref("D28") }}), ein gesperrtes oder nicht erreichbares Backend, ein Fehler oder eine Wartezeit der Freigabe und eine Ablehnung.

Eine Anfrage, die das Tor vor der Sperre passiert hat und deren Eintrag „Auslieferung“ steht, liest ihre Werte noch. Der Eintrag „Auslieferung“ ist der Zeitpunkt, der zählt ({{ ref("D36") }}).

## Protokoll auf den Sockets

Das Paket `internal/protocol` setzt {{ ref("D26") }} um.
Eine Nachricht ist ein JSON-Objekt auf einer Zeile (JSON: JavaScript Object Notation).
Jede Nachricht trägt `version` und `type`.
Die Version ist `1`.

### Nachrichten des Agent-Sockets

| Anfrage | Antwort | Inhalt |
|---|---|---|
| `hello` | `welcome` | Erste Nachricht einer Verbindung. Sie nennt den Namen der Sitzung und den Namen des Agent-Werkzeugs. Beide sind Angaben des Agents. Die Antwort nennt die Kennung der Sitzung und das Projekt. |
| `request_credentials` | `credentials` | Die Anfrage nennt Referenzen, Aufgabe und Begründung. Die Antwort hat je Referenz einen Wert oder einen Fehler. |
| `grant_status` | `grants` | Grants der eigenen Sitzung ohne Werte |

### Nachrichten des Admin-Sockets

| Anfrage | Antwort | Inhalt |
|---|---|---|
| `grants_list` | `grants` | Grants aller Sitzungen |
| `grant_revoke` | `grant_revoked` | Widerruf eines Grants |
| `status` | `status_result` | Zustand des Daemons für `a38 doctor`: Störung mit Prüfung der Ursache, ungültige Projekte, Zahl der Sitzungen und Grants, Zustand des Backends, Variante der Freigabe |
| `fault_clear` | `fault_clear_result` | Aufheben der globalen Sperre ({{ ref("D48") }}) |

Scheitert eine Anfrage als Ganzes, ist die Antwort auf beiden Sockets die Nachricht `error` mit Code und Text.

### Regeln

- Jede Anfrage außer `hello` trägt eine Kennung `id`. Die Antwort wiederholt sie. Eine Sitzung hat im Bündelfenster mehrere offene Anfragen; die Kennung ordnet die Antworten zu.
- Jeder Socket hat eine eigene Menge von Nachrichten: `protocol.AgentRequests`, `protocol.AgentResponses`, `protocol.AdminRequests` und `protocol.AdminResponses`. Eine Admin-Nachricht auf dem Agent-Socket ist ein unbekannter Typ und endet mit `invalid_request` ({{ ref("T12") }}, {{ ref("T28") }}).
- `Set.Decode` lehnt ab: eine Zeile über 64 KiB, kein JSON-Objekt, eine falsche Version, einen unbekannten Typ, ein unbekanntes Feld, einen Schlüssel mit Großbuchstaben, einen doppelten Schlüssel und einen ungültigen Inhalt ({{ ref("T10") }}).
- Der Text eines Fehlers der Dekodierung wiederholt nie den Inhalt der Zeile.
- `request_credentials` hat kein Feld für den Namen der Sitzung. Der Name steht je Verbindung fest ({{ ref("T21") }}).
- Ein Steuerzeichen im Namen der Sitzung ist immer ungültig. Die Länge und die erlaubten Zeichen prüft der Daemon mit `config.SessionNameRules.Check`.
- Der Daemon sendet den Code `daemon_unavailable` nie. Nur `a38 mcp` erzeugt ihn, wenn der Socket nicht erreichbar ist.

### Fehlercodes

Die Codes aus {{ ref("D35") }} sind Konstanten vom Typ `protocol.Code`.
`protocol.Codes()` liefert die vollständige Liste.
Der Test für die Anleitung der Agents nutzt diese Liste ({{ ref("T20") }}).
`protocol.Error` ist zugleich der Go-Fehler und das Fehlerobjekt auf dem Socket.

### Referenz und Secret-Wert

`protocol.Reference` ist die Referenz `op://<vault>/<item>/<feld>` aus {{ ref("D27") }}.
`ParseReference` verlangt genau drei Segmente.
Ein Segment ist nicht leer und enthält weder einen Schrägstrich noch ein Steuerzeichen.
Eine Referenz enthält den Text des Agents.
Die geprüften Namen liefert `Backend.Resolve`.

`protocol.Secret` hält einen Secret-Wert.
Die Pakete `fmt` und `log/slog` und `json.Marshal` geben statt des Werts `[redacted]` aus.
Nur `Secret.Reveal` liefert den Wert.
Nur `protocol.Encode` schreibt den Wert, und nur in die Nachricht `credentials` ({{ ref("D34") }}, {{ ref("T8") }}).

## Freigabe-Schnittstelle

Das Paket `internal/approval` beschreibt die Grenze zwischen Daemon und Freigabe-UI ({{ ref("D6") }}).

- `Request` ist die Freigabeanfrage. Sie trägt die geprüften Fakten (`verified`), die Angaben des Agents (`claims`), die Einträge, die vorgeschlagene Laufzeit und bei einer Abweichung den bestehenden Grant.
- `Decision` ist die Entscheidung: Ergebnis, ausgewählte Einträge, gewählte Laufzeit.
- `Notice` ist die Meldung an den Menschen bei einer Störung des Daemons ({{ ref("D48") }}, {{ ref("D36") }}) und bei der fehlerhaften Konfiguration eines Projekts ({{ ref("D28") }}). Sie informiert nur.
- `UI` hat zwei Methoden: `Approve` und `Notify`.

Der Daemon vertraut einer Entscheidung nicht.
Der Broker prüft die Kennung der Anfrage, die Auswahl und die Laufzeit.
Eine ungültige Freigabe endet mit `ui_failed` ({{ ref("D32") }}, {{ ref("T23") }}).

### Verhältnis zum Prototyp

Die JSON-Form folgt dem Prototyp aus dem [Spike zur Freigabe-UI](spikes/20-freigabe-ui.md).
Ein Test liest die Beispielanfragen des Prototyps mit den Typen dieses Pakets.
Die Abweichungen:

| Abweichung | Grund |
|---|---|
| `verified.session_id` ist neu. | Die geprüften Fakten nennen die Kennung des Daemons ({{ ref("T21") }}). |
| `verified.peer_uid` ist neu. | {{ ref("D25") }} nennt den Benutzer als Teil der Herkunft. |
| `claims.session_name` ist neu. | Der Name der Sitzung steht nur bei den Angaben ({{ ref("D25") }}). |
| `credentials[].field` ist neu. | Ein Grant gilt für ein Feld eines Items ({{ ref("D27") }}, {{ ref("D32") }}). Der Mensch sieht, welches Feld er freigibt. |
| `credentials[].deviation` kennt `outside_scope` nicht. | Eine Anfrage außerhalb der Policy erreicht die Freigabe nie ({{ ref("D28") }}). |
| `grant.scope` ist nur Anzeige. | Ein Grant umfasst genau die angehakten Zugangsdaten und keinen ganzen Vault ({{ ref("D32") }}, {{ ref("G7") }}). |
| `verified.vm_name` und `verified.ssh_session` fehlen. | Die Herkunft aus der VM gehört zur Stufe 5 ({{ ref("G1") }}). |
| Der Typ `notice` ist neu. | Der Prototyp kennt nur Anfrage und Entscheidung. Ob die Freigabe-App die Meldung zeigt, ist offen ({{ ref("G20") }}). |

Der Prototyp ignoriert unbekannte Felder.
Die neuen Felder brechen ihn deshalb nicht.

### Freigabe-App

Die Freigabe-App `a38-approve` liegt im Ordner `app/` ({{ ref("F11") }}).
Sie prüft jede Nachricht streng gegen diese Typen: Ein unbekanntes Feld, ein fehlendes Pflichtfeld oder ein Eintrag außerhalb von dev ist ein Fehler.
Die [Referenz der Freigabe-App](../reference/90-freigabe-app.md) beschreibt Fälle, Regeln, Exit-Codes und Testschalter.

## Schnittstellen zu Backend und Audit-Log

`backend.Backend` hat drei Methoden: `Vaults`, `Resolve` und `Read` ({{ ref("D38") }}).
`Resolve` liest keinen Wert.
Die Fehlerzustände sind `ErrLocked`, `ErrUnavailable` und `ErrNotFound`.
`backend.Code` ordnet einen Fehler dem stabilen Code zu ({{ ref("T17") }}).

`audit.Store` hat die Methoden `Append`, `List` und `Verify` ({{ ref("D36") }}).
Der Daemon übergibt ein `audit.Event`.
Der Store ergänzt Nummer, Zeitpunkt und Hash.
Die kanonische Form eines Eintrags legt Issue 17 fest.
Ein Eintrag hält die geprüften Fakten und die Angaben des Agents in getrennten Feldern.

Beginn und Ende der globalen Sperre stehen als Ereignisart „Fehler“ im Audit-Log.
Das Feld `detail` trägt dann `fault_begin` oder `fault_end`.
{{ ref("D36") }} nennt dafür keine eigene Ereignisart.

## Konfigurationsdatei

Format und Ort legt {{ ref("D49") }} fest.
Die Datei ist YAML und liegt unter `~/.config/a38/config.yaml`.
Der Schalter `--config` und die Variable `A38_CONFIG` nennen einen anderen Pfad.

```yaml
version: 1
vaults:
  - name: beispiel.app.k8s-dev
    environment: dev
projects:
  - name: beispielprojekt
    paths:
      - ~/develop/beispielprojekt
      - ~/develop/beispielprojekt__worktrees
    vaults:
      - beispiel.app.k8s-dev
grant:
  lifetime: 4h
bundle:
  window: 3s
  max_entries: 10
session_name:
  max_length: 80
  allowed_characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789 -_():"
paths:
  socket_dir: ~/.local/state/a38/run
  audit_file: ~/.local/state/a38/audit.db
```

Nur `version` ist Pflicht.
Das Beispiel zeigt bei allen anderen Schlüsseln außer `vaults` und `projects` die Standardwerte.

| Regel | Folge | Beleg |
|---|---|---|
| Ein Vault ohne Umgebung oder mit einer anderen Umgebung als `dev`, `test`, `prod` | Das Laden scheitert. Die Meldung nennt den Vault. | {{ ref("D47") }}, {{ ref("T22") }} |
| Derselbe Vault zweimal | Das Laden scheitert. | {{ ref("T22") }} |
| Ein Projekt nennt einen Vault, der in der Liste der Vaults fehlt. | Das Laden gelingt. Das Projekt trägt den Befund im Feld `Invalid`. Der Daemon lehnt seine Anfragen mit `config_invalid` ab. | {{ ref("D28") }} |
| Laufzeit, Bündelfenster, Obergrenze oder Höchstlänge mit dem Wert 0, einem negativen Wert oder Text | Das Laden scheitert. | {{ ref("T4") }}, {{ ref("T19") }}, {{ ref("T21") }} |
| Ein unbekannter Schlüssel | Das Laden scheitert. | {{ ref("D49") }} |

`Config.ProjectFor` leitet das Projekt aus dem Arbeitsverzeichnis ab.
Der längste passende Pfad gewinnt.
Der Daemon löst symbolische Links vor dem Aufruf auf.
Der Daemon vergleicht den Namen eines Vaults genau, ohne Rücksicht auf Muster ({{ ref("D12") }}).

Der Daemon liest die Datei einmal beim Start.
Eine Änderung braucht einen Neustart.

## Kommandozeile

Die Unterbefehle nutzen das Paket `flag` der Standardbibliothek.
Eine Bibliothek für die Kommandozeile bringt für sieben Unterbefehle keinen Vorteil und wäre eine weitere Abhängigkeit im Binary des Daemons.

- `cmd/a38/main.go` enthält nur die Liste der Unterbefehle.
- Jeder Unterbefehl hat eine eigene Datei, zum Beispiel `cmd/a38/serve.go`, mit der Funktion `runServe(args, stdout, stderr) int`.
- Ein Unterbefehl ohne Code schreibt `not implemented` und endet mit dem Exit-Code 1.
- Ein Fehler im Aufruf endet mit dem Exit-Code 2.

Der Besitzer eines Unterbefehls ändert nur seine Datei.

## Teststrategie

Jeder Test nennt seinen Testfall im Kommentar, zum Beispiel `// T22 (part): ...`.
Die Tests sind tabellengetrieben.

| Mittel | Paket | Zweck |
|---|---|---|
| gefälschte Uhr | `clock.Fake` | Der Test stellt die Zeit mit `Advance` vor. Bündelfenster, Wartezeit der Freigabe und Laufzeit eines Grants brauchen keine echte Zeit ({{ ref("T2") }}, {{ ref("T4") }}, {{ ref("T19") }}). `BlockUntil` wartet, bis der Code seinen Timer gestartet hat. |
| Fake-Backend | `backend/fake` | Vaults aus einer YAML-Datei. Das Backend zählt Auflösungen und Lesezugriffe und stellt „gesperrt“ und „nicht erreichbar“ nach ({{ ref("T3") }}, {{ ref("T17") }}). |
| gefälschte Freigabe | `approval/fakeui` | `auto-approve`, `auto-deny` und `script`. Das Skript speichert die Freigabeanfrage und die Meldungen ({{ ref("T21") }}, {{ ref("T26") }}). |
| gefälschter Broker | Test in `daemon` | Die Socket-Schicht läuft ohne Policy, Grants und Audit-Log ({{ ref("T10") }}, {{ ref("T12") }}). |
| Fuzz-Test | `protocol` | `FuzzDecode` prüft, dass die Dekodierung bei keiner Eingabe abstürzt ({{ ref("T10") }}). |

Es gibt drei Ebenen von Tests:

1. **Paket-Test:** Der Test prüft ein Paket allein. Er nutzt die Fakes der Schnittstellen.
2. **Test im Prozess:** Der Test baut den Daemon im Testprozess zusammen: Socket in einem temporären Verzeichnis, Fake-Backend, Skript-UI, gefälschte Uhr. Die meisten Testfälle des Design-Sets laufen auf dieser Ebene, weil sie die gefälschte Uhr brauchen.
3. **Test mit Prozessen:** Der Test baut das Binary einmal je Testlauf in ein temporäres Verzeichnis. Er startet `a38 serve --backend fake --fake-file <datei> --ui auto-approve` mit einer eigenen Konfigurationsdatei und danach `a38 mcp` als Kindprozesse. Er spricht MCP über stdin und stdout von `a38 mcp` ({{ ref("T1") }}). Diese Ebene nutzt die echte Uhr und deshalb kurze Werte in der Konfiguration, zum Beispiel ein Bündelfenster von 50 Millisekunden.

Regeln für einen Test mit Prozessen:

- Die Konfiguration nennt `paths.socket_dir` und `paths.audit_file` in einem temporären Verzeichnis. Der Pfad eines Sockets bleibt unter 103 Byte; unter macOS ist `t.TempDir()` dafür oft zu lang. Der Test legt das Verzeichnis dann unter `/tmp` an.
- Der Test wartet auf den Socket und nicht auf eine feste Zeit.
- Der Test beendet die Kindprozesse in `t.Cleanup`.
- Ein Kennwert im Fake-Backend zeigt, ob ein Secret-Wert in eine Ausgabe gelangt ({{ ref("T8") }}).

## Abhängigkeiten

Das Binary bleibt statisch (`CGO_ENABLED=0`).
Jede Abhängigkeit braucht einen Grund.

| Modul | Version | Zweck | Stand |
|---|---|---|---|
| `go.yaml.in/yaml/v3` | v3.0.6 | Konfigurationsdatei und Fake-Backend | in `go.mod` |
| `golang.org/x/sys` | v0.49.0 | Peer-Daten des Sockets ohne cgo: `LOCAL_PEERCRED` und `LOCAL_PEERPID` unter macOS, `SO_PEERCRED` unter Linux | in `go.mod` |
| `modernc.org/sqlite` | v1.62.0 | Audit-Log; SQLite in reinem Go, ohne cgo | Empfehlung für Issue 17 |
| `github.com/modelcontextprotocol/go-sdk` | v1.8.0 | MCP-Server über stdio; das offizielle Go-SDK | in `go.mod` (Issue 24) |

`go.yaml.in/yaml` ist der gepflegte Nachfolger von `gopkg.in/yaml.v3`.
Die Version v4.0.0 ist eine Woche alt (Stand 2026-11-11).
Die Version v3 ist erprobt und reicht für beide Dateien.

Das MCP-SDK steht seit Issue 24 in `go.mod`. Das SQLite-Modul steht noch nicht darin.
Das Issue, das ein Modul braucht, nimmt es auf und prüft dabei die aktuelle Version.
`modernc.org/sqlite` gehört nur in den Daemon und in `a38 audit`.
Das MCP-SDK gehört nur in `internal/mcp`.

## Was diese Seite nicht entscheidet

- Die Form des Startwerts für den Namen der Sitzung ist in Issue 24 gewählt: das Argument `--session-name` und die Umgebungsvariable `A38_SESSION_NAME` von `a38 mcp`, das Argument hat Vorrang ({{ ref("G19") }}). Wer den Wert je Sitzung setzt, bleibt offen.
- Der Name des Admin-Befehls zum Aufheben der Sperre ist offen ({{ ref("G20") }}). Das Protokoll nennt die Nachricht `fault_clear`. Issue 25 wählt auf der Kommandozeile `a38 fault clear` und `a38 fault status`; Stefan bestätigt den Namen.
- Ob die Freigabe-App die Meldung zeigt, ist offen ({{ ref("G20") }}). In Stufe 1 erhält das Skript der Oberfläche die Meldung.
- Ob ein Agent die Namen der Items über ein Werkzeug findet, ist offen ({{ ref("G6") }}). Das Protokoll hat dafür keine Nachricht. `Backend.Vaults` listet nur Vaults.
- Ob eine Verbindung je Agent die richtige Sitzung ist, ist offen ({{ ref("G2") }}). Der Code setzt {{ ref("D25") }} um.
