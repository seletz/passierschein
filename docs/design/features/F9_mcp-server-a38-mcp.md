---
render_macros: true
id: 'F9'
titel: 'MCP-Server a38 mcp'
bereich: 'Oberflächen'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
stufe: 1
umsetzung: zu-bauen
implementiert: [D2, D27, D41]
use_cases: [UC1, UC2]
---
# F9 — MCP-Server a38 mcp

{{ design_kopf(page) }}

`a38 mcp` ist ein MCP-Server über stdio. Er übersetzt die beiden Werkzeuge in Nachrichten für den Agent-Socket. Er enthält keinen Code für 1Password.


Setzt um: {{ ref("D2") }}, {{ ref("D27") }}, {{ ref("D41") }}.
