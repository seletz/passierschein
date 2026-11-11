---
render_macros: true
id: 'D16'
titel: 'Die Entwicklung läuft lokal zuerst, mit gefälschtem Backend und gefälschter Freigabe'
bereich: 'Arbeitsweise'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: gueltig
use_cases: [UC10]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Projektnotiz vom 2026-11-11
---
# D16 — Die Entwicklung läuft lokal zuerst, mit gefälschtem Backend und gefälschter Freigabe

{{ design_kopf(page) }}

**Entscheidung:** Die Stufen sind: 1 lokal mit Fake-Backend und Fake-UI, 2 Freigabe-App, 3 echtes 1Password-Backend, 4 Hook, 5 VM.

**Begründung:** Der Ablauf ist früh und in der CI prüfbar. Die VM ändert nur den Transport.

**Quelle:** Projektnotiz, Stufenplan.
