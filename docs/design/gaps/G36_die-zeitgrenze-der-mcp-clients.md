---
render_macros: true
id: 'G36'
titel: 'Die Zeitgrenze der MCP-Clients für lange Werkzeugaufrufe ist nicht geprüft'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-15'
order: 2
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G36 — Die Zeitgrenze der MCP-Clients für lange Werkzeugaufrufe ist nicht geprüft

{{ design_kopf(page) }}

`run_with_credentials` wartet auf die Freigabe (bis zu 2 Minuten, {{ ref("D30") }}) und danach auf das Kommando (bis zu `exec.max_timeout`, {{ ref("D64") }}). Ein MCP-Client kann einen Werkzeugaufruf vorher abbrechen.

Offene Fragen:

- Welche Zeitgrenze für einen Werkzeugaufruf hat Claude Code, und welche haben andere Agent-Werkzeuge?
- Was tut `a38 mcp`, wenn der Client den Aufruf abbricht? Vorschlag: die Prozessgruppe beenden und `exec_result` mit Zeitablauf melden.
- Passt der Standard `2m` zu der Grenze des Clients?

**Quelle:** [Issue 91](https://github.com/seletz/passierschein/issues/91). Klärung im Praxistest von 0.3.0.
