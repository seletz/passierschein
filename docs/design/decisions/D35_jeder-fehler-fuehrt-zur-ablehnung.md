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
  wer: Stefan
  datum: '2026-11-11'
  notiz: Vorschlag des Designlaufs, von Stefan im Review am 2026-11-11 abgenommen
---
# D35 — Jeder Fehler führt zur Ablehnung mit einem stabilen Fehlercode

{{ design_kopf(page) }}

**Entscheidung:** Bei jedem Fehler liefert der Daemon nichts aus. Der Agent erhält einen Fehler mit einem stabilen Code: `denied`, `timeout`, `not_allowed`, `bundle_too_large`, `daemon_unavailable`, `backend_locked`, `backend_unavailable`, `ui_failed`, `audit_failed`, `daemon_faulted`, `config_invalid`, `session_name_conflict`, `not_found`, `invalid_request`. Es gibt keinen Ersatzweg.

**Begründung:** Ein Agent mit einer unklaren Meldung sucht einen Umweg ({{ ref("RI8") }}). Ein stabiler Code erlaubt eine klare Regel im Skill.
