---
render_macros: true
id: 'T13'
titel: 'Der linux-Build enthält keinen Code für 1Password'
bereich: 'Backends'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [D2, D42]
---
# T13 — Der linux-Build enthält keinen Code für 1Password

{{ design_kopf(page) }}

**Form:** Prüfung in der CI.

**Schritte:** `go list -deps` für den linux-Build.

**Erwartung:** Die Liste enthält kein Paket des 1Password-SDK und kein Paket des 1Password-Backends.


Belegt: {{ ref("D2") }}, {{ ref("D42") }}.
