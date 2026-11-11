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
  notiz: Vorschlag des Designlaufs, zu bestätigen durch Stefan
  annahme: true
---
# D25 — Eine Sitzung ist eine Verbindung zum Agent-Socket, und der Daemon vergibt ihre Kennung

{{ design_kopf(page) }}

**Entscheidung:** Eine Sitzung ist eine Verbindung eines Prozesses `a38 mcp` zum Agent-Socket. Der Daemon vergibt die Kennung der Sitzung. Der Daemon liest die Herkunft aus den Peer-Daten des Sockets: Prozess-ID, Benutzer, Pfad des Programms und Arbeitsverzeichnis. Aus dem Arbeitsverzeichnis leitet der Daemon das Projekt ab. Der Agent kann die Kennung und die Herkunft nicht setzen.

**Begründung:** Ein Grant braucht einen Träger, den der Agent nicht wählen kann. Eine Verbindung endet mit dem Prozess.

**Grenze:** Lokal sind die Peer-Daten ein Hinweis und kein Beweis ({{ ref("RI5") }}). Offene Fragen: {{ ref("G1") }}, {{ ref("G2") }}.

**Quelle:** Projektnotiz, Stufe 1: Herkunft über die Peer-Daten des Sockets.
