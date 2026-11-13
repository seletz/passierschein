---
render_macros: true
id: 'F16'
titel: 'Hook gegen den direkten Aufruf von op'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-12'
order: 3
headless: true
status: aktiv
stufe: 4
umsetzung: teilweise
implementiert: [D15]
use_cases: [UC12]
---
# F16 — Hook gegen den direkten Aufruf von op

{{ design_kopf(page) }}

Ein Hook für die Agent-Werkzeuge erkennt den Aufruf von `op` und verweist auf `a38`.


Setzt um: {{ ref("D15") }}.
