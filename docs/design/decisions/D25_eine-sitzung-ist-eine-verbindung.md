---
render_macros: true
id: 'D25'
titel: 'Eine Sitzung ist eine Verbindung zum Agent-Socket, und der Daemon vergibt ihre Kennung'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 7
headless: true
status: gueltig
use_cases: [UC1, UC2]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Vorschlag des Designlaufs, von Stefan im Review am 2026-11-11 abgenommen
---
# D25 — Eine Sitzung ist eine Verbindung zum Agent-Socket, und der Daemon vergibt ihre Kennung

{{ design_kopf(page) }}

**Entscheidung:** Eine Sitzung ist eine Verbindung eines Prozesses `a38 mcp` zum Agent-Socket. Der Daemon vergibt die Kennung der Sitzung. Der Daemon liest die Herkunft aus den Peer-Daten des Sockets: Prozess-ID, Benutzer, Pfad des Programms und Arbeitsverzeichnis. Aus dem Arbeitsverzeichnis leitet der Daemon das Projekt ab. Der Agent kann die Kennung und die Herkunft nicht setzen.

**Begründung:** Ein Grant braucht einen Träger, den der Agent nicht wählen kann. Eine Verbindung endet mit dem Prozess.

**Name der Sitzung:** Der Mensch braucht zur Zuordnung den Namen der Sitzung aus seinem Agent-Werkzeug, zum Beispiel `beispiel-koordinator`. Diesen Namen findet er in allen Werkzeugen wieder, mit denen er mit den Agents arbeitet. Der Name ist ein Startwert von `a38 mcp`. Die Anmeldung des MCP-Servers legt ihn je Sitzung fest. Das Agent-Werkzeug startet `a38 mcp` außerhalb der Sitzung des Agents; der Agent wählt den Namen deshalb nicht selbst. `a38 mcp` übergibt den Namen beim Aufbau der Verbindung. Das ist kein dichter Schutz: Ein Agent mit weiten Rechten, zum Beispiel in der VM, kann die Anmeldung ändern. Die Freigabeanfrage, das Audit-Log und die TUI zeigen ihn. Der Name ist eine Angabe aus der Zone des Agents und gilt als unbestätigt ({{ ref("D4") }}). Die Kennung des Daemons bleibt der Träger des Grants. Stefan hat das am 2026-11-11 im Review festgelegt.

**Gleicher Name:** Zwei Sitzungen mit demselben Namen sind ein Fehler. Der Daemon lehnt die Anfragen beider Sitzungen mit dem Code `session_name_conflict` ab, solange beide verbunden sind. Stefan hat das am 2026-11-11 im Review festgelegt. Folge: Eine Sitzung kann eine andere blockieren, indem sie deren Namen nennt. Das ist die sichere Richtung; es gibt dann keine Auslieferung.

**Grenze:** Lokal sind die Peer-Daten ein Hinweis und kein Beweis ({{ ref("RI5") }}). Offene Fragen: {{ ref("G1") }}, {{ ref("G2") }}, {{ ref("G19") }}.

**Quelle:** Projektnotiz, Stufe 1: Herkunft über die Peer-Daten des Sockets.
