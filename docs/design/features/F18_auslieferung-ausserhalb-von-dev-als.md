---
render_macros: true
id: 'F18'
titel: 'Auslieferung außerhalb von dev als Datei auf einem tmpfs'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-15'
order: 4
headless: true
status: verworfen
stufe: 5
umsetzung: zu-bauen
implementiert: [D10]
use_cases: [UC5]
beleg:
  datum: '2026-11-15'
  notiz: ersetzt durch F21 und F22, D62 ersetzt D10 (Issue 91), zu bestätigen durch Stefan
  annahme: true
---
# F18 — Auslieferung außerhalb von dev als Datei auf einem tmpfs

{{ design_kopf(page) }}

Der Daemon liefert einen Wert außerhalb von dev als Datei mit Laufzeit aus. Der Agent erhält nur den Pfad.


Setzt um: {{ ref("D10") }}.
