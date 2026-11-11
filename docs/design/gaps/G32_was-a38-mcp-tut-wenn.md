---
render_macros: true
id: 'G32'
titel: 'Was a38 mcp tut, wenn der Daemon die Verbindung beendet, ist nicht festgelegt'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-11'
order: 1
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G32 — Was a38 mcp tut, wenn der Daemon die Verbindung beendet, ist nicht festgelegt

{{ design_kopf(page) }}

Beendet der Daemon die Verbindung zu `a38 mcp`, zum Beispiel bei einem Neustart, sagt das Design nicht, was `a38 mcp` tut ({{ ref("D25") }}, {{ ref("D35") }}).

**Annahme der Entwicklung:** Der Aufruf endet mit `daemon_unavailable`. `a38 mcp` beendet sich nicht. Der nächste Aufruf baut eine neue Verbindung auf. Eine neue Verbindung ist eine neue Sitzung; die Grants der alten Sitzung gelten nicht mehr ({{ ref("D33") }}).

Offene Frage:

- Soll `a38 mcp` sich stattdessen beenden, damit das Agent-Werkzeug den Abbruch sieht?

**Quelle:** Entwicklung von Stufe 1, Broker und MCP-Server (Issues 21, 22, 24), 2026-11-11.
