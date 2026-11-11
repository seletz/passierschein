---
render_macros: true
id: 'D10'
titel: 'Ein Secret außerhalb von dev geht nie als MCP-Antwort an den Agent, sondern als Datei auf einem tmpfs'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 3
headless: true
status: gueltig
use_cases: [UC5]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Projektnotiz vom 2026-11-11
---
# D10 — Ein Secret außerhalb von dev geht nie als MCP-Antwort an den Agent, sondern als Datei auf einem tmpfs

{{ design_kopf(page) }}

**Entscheidung:** Ein Secret außerhalb von dev steht nie in einer MCP-Antwort. Der Daemon schreibt den Wert in eine Datei auf einem tmpfs. Die Datei hat eine Laufzeit.

**Begründung:** Der Wert erreicht so nicht den Kontext des Modells.

**Quelle:** Projektnotiz, Ablauf. Die Umsetzung ist offen ({{ ref("G11") }}).
