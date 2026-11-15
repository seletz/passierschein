---
render_macros: true
id: 'D10'
titel: 'Ein Secret außerhalb von dev geht nie als MCP-Antwort an den Agent, sondern als Datei auf einem tmpfs'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-15'
order: 3
headless: true
status: ersetzt
ersetzt_durch: D62
use_cases: [UC5]
beleg:
  datum: '2026-11-15'
  quelle: ['#91']
  notiz: Vorschlag aus Issue 91, zu bestätigen durch Stefan
  annahme: true
---
# D10 — Ein Secret außerhalb von dev geht nie als MCP-Antwort an den Agent, sondern als Datei auf einem tmpfs

{{ design_kopf(page) }}

**Entscheidung:** Ein Secret außerhalb von dev steht nie in einer MCP-Antwort. Der Daemon schreibt den Wert in eine Datei auf einem tmpfs. Die Datei hat eine Laufzeit.

**Begründung:** Der Wert erreicht so nicht den Kontext des Modells.

**Quelle:** Projektnotiz, Ablauf. Die Umsetzung ist offen ({{ ref("G11") }}).

**Hinweis:** Die Entscheidung gilt. Der Praxistest prüft, ob sie sich bewährt (Stefan, 2026-11-11).
