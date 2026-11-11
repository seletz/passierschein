---
render_macros: true
id: 'D22'
titel: 'Die Anleitung für Agents liegt im Binary'
bereich: 'Oberflächen'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: gueltig
use_cases: [UC9]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Projektnotiz vom 2026-11-11
---
# D22 — Die Anleitung für Agents liegt im Binary

{{ design_kopf(page) }}

**Entscheidung:** Die Anleitung für Agents ist mit `go:embed` Teil des Binarys. `a38 llm --skill` gibt sie aus.

**Begründung:** Der Skill passt immer zur installierten Version.

**Quelle:** Projektnotiz, Entwicklungsregeln.
