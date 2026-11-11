---
render_macros: true
id: 'D41'
titel: 'Der MCP-Server bietet zwei Werkzeuge: request_credentials und grant_status'
bereich: 'Oberflächen'
lauf: '1.0'
stand: '2026-11-11'
order: 3
headless: true
status: gueltig
use_cases: [UC1, UC2]
beleg:
  notiz: Vorschlag des Designlaufs, zu bestätigen durch Stefan
  annahme: true
---
# D41 — Der MCP-Server bietet zwei Werkzeuge: request_credentials und grant_status

{{ design_kopf(page) }}

**Entscheidung:** `a38 mcp` bietet `request_credentials` (Liste von Referenzen, Aufgabe, Begründung) und `grant_status` (Grants der eigenen Sitzung ohne Werte). Die Beschreibung von `request_credentials` bittet um gebündelte Anfragen. Ein Fehler kommt als Ergebnis mit `isError` und dem stabilen Code.

**Begründung:** Wenige Werkzeuge halten die Oberfläche klein.

**Offen:** Ein Werkzeug zum Finden von Items ({{ ref("G6") }}).
