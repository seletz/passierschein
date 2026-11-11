---
render_macros: true
id: 'D21'
titel: 'Die Automation ist nicht Teil von a38'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 6
headless: true
status: gueltig
use_cases: [UC1]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Projektnotiz vom 2026-11-11
---
# D21 — Die Automation ist nicht Teil von a38

{{ design_kopf(page) }}

**Entscheidung:** Agents für Analysen, Renovate und Health Checks sind nicht Teil von `a38`. Sie laufen nur lesend über eine eigene Tailscale-Persona in einer VM.

**Begründung:** Dieser Fall gilt als gelöst.

**Quelle:** Projektnotiz, Situation.
