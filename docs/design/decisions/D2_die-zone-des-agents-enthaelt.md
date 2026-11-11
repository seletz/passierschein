---
render_macros: true
id: 'D2'
titel: 'Die Zone des Agents enthält keine Werkzeuge von 1Password'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: gueltig
use_cases: [UC13, UC1]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Projektnotiz vom 2026-11-11
---
# D2 — Die Zone des Agents enthält keine Werkzeuge von 1Password

{{ design_kopf(page) }}

**Entscheidung:** In der VM gibt es kein `op`, kein SDK und kein Token eines Service Accounts. `a38 mcp` enthält keinen Code für 1Password und spricht nur mit dem Agent-Socket. `a38 mcp` gilt als nicht vertraut, weil es in der Zone des Agents läuft.

**Begründung:** Was in der Zone des Agents nicht existiert, kann der Agent nicht missbrauchen.

**Quelle:** Projektnotiz, Ziele.
