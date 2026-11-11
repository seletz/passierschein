---
render_macros: true
id: 'D42'
titel: 'a38 ist ein Binary mit Unterbefehlen, und der linux-Build enthält kein 1Password-Backend'
bereich: 'Oberflächen'
lauf: '1.0'
stand: '2026-11-11'
order: 4
headless: true
status: gueltig
use_cases: [UC13, UC11]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Vorschlag des Designlaufs, von Stefan im Review am 2026-11-11 abgenommen
---
# D42 — a38 ist ein Binary mit Unterbefehlen, und der linux-Build enthält kein 1Password-Backend

{{ design_kopf(page) }}

**Entscheidung:** `a38` ist ein Binary mit den Unterbefehlen `serve`, `mcp`, `tui`, `llm`, `audit`, `grants`, `service` und `doctor`. Das 1Password-Backend liegt hinter einem Build-Tag. Der linux-Build enthält das Backend und das SDK nicht.

**Begründung:** Ein Binary ist einfach zu verteilen. Die VM bekommt trotzdem keinen Code für 1Password ({{ ref("D2") }}).
