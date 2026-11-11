---
render_macros: true
id: 'D30'
titel: 'Es gibt höchstens eine Freigabeanfrage zur gleichen Zeit, und sie läuft nach zwei Minuten ab'
bereich: 'Freigabe'
lauf: '1.0'
stand: '2026-11-11'
order: 4
headless: true
status: gueltig
use_cases: [UC4]
beleg:
  notiz: Vorschlag des Designlaufs, zu bestätigen durch Stefan
  annahme: true
---
# D30 — Es gibt höchstens eine Freigabeanfrage zur gleichen Zeit, und sie läuft nach zwei Minuten ab

{{ design_kopf(page) }}

**Entscheidung:** Der Daemon zeigt höchstens eine Freigabeanfrage zur gleichen Zeit. Weitere Bündel warten in einer Warteschlange. Eine Freigabeanfrage ohne Entscheidung läuft nach zwei Minuten ab. Der Ablauf gilt als Ablehnung mit dem Code `timeout`.

**Begründung:** Mehrere Popups zur gleichen Zeit fördern blindes Bestätigen. Ein Agent darf nicht unbegrenzt warten ({{ ref("RI8") }}).
