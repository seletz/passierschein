---
render_macros: true
id: 'D35'
titel: 'Jeder Fehler führt zur Ablehnung mit einem stabilen Fehlercode'
bereich: 'Anfrage und Bündelung'
lauf: '1.0'
stand: '2026-11-11'
order: 4
headless: true
status: gueltig
use_cases: [UC14, UC4]
beleg:
  notiz: Vorschlag des Designlaufs, zu bestätigen durch Stefan
  annahme: true
---
# D35 — Jeder Fehler führt zur Ablehnung mit einem stabilen Fehlercode

{{ design_kopf(page) }}

**Entscheidung:** Bei jedem Fehler liefert der Daemon nichts aus. Der Agent erhält einen Fehler mit einem stabilen Code: `denied`, `timeout`, `not_allowed`, `bundle_too_large`, `daemon_unavailable`, `backend_locked`, `backend_unavailable`, `ui_failed`, `invalid_request`. Es gibt keinen Ersatzweg.

**Begründung:** Ein Agent mit einer unklaren Meldung sucht einen Umweg ({{ ref("RI8") }}). Ein stabiler Code erlaubt eine klare Regel im Skill.
