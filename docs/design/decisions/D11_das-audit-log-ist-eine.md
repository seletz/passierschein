---
render_macros: true
id: 'D11'
titel: 'Das Audit-Log ist eine SQLite-Datei mit Hash-Kette'
bereich: 'Audit'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: gueltig
use_cases: [UC6, UC8]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Projektnotiz vom 2026-11-11
---
# D11 — Das Audit-Log ist eine SQLite-Datei mit Hash-Kette

{{ design_kopf(page) }}

**Entscheidung:** Der Daemon schreibt jeden Grant, jede Auslieferung und jeden Fehlschlag in eine SQLite-Datei. Jeder Eintrag enthält den Hash des vorigen Eintrags.

**Begründung:** Die Freigaben sind auswertbar. Eine Änderung ist erkennbar.

**Quelle:** Projektnotiz, Design v0. Einzelheiten: {{ ref("D36") }}, {{ ref("D37") }}.
