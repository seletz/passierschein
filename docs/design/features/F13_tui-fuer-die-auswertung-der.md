---
render_macros: true
id: 'F13'
titel: 'TUI für die Auswertung der Freigaben'
bereich: 'Audit'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: aktiv
stufe: 3
umsetzung: zu-bauen
implementiert: [D11, D36]
use_cases: [UC6]
---
# F13 — TUI für die Auswertung der Freigaben

{{ design_kopf(page) }}

`a38 tui` liest das Audit-Log nur lesend. Es filtert nach Projekt, Agent, Zugangsdatum, Zeit und Ergebnis und zeigt Kennzahlen.


Setzt um: {{ ref("D11") }}, {{ ref("D36") }}.
