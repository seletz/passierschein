---
render_macros: true
id: 'D23'
titel: 'Das MVP umfasst die Stufen 1 bis 3'
bereich: 'Arbeitsweise'
lauf: '1.0'
stand: '2026-11-11'
order: 5
headless: true
status: gueltig
use_cases: [UC1, UC11]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Vorschlag des Designlaufs, von Stefan im Review am 2026-11-11 abgenommen
---
# D23 — Das MVP umfasst die Stufen 1 bis 3

{{ design_kopf(page) }}

**Entscheidung:** Das MVP ist erreicht, wenn `a38` lokal mit der Freigabe-App und dem echten 1Password-Backend läuft. Das sind die Stufen 1 bis 3. Der Hook (Stufe 4) und die VM (Stufe 5) folgen danach.

**Begründung:** Die Projektnotiz sagt zu Stufe 3: „Ab hier löst es lokal schon das Ausgangsproblem.“ Das Ausgangsproblem ist die Freigabe ohne Kontext.

**Folge:** Das MVP isoliert nicht ({{ ref("RI5") }}).
