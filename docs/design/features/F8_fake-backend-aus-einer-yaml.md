---
render_macros: true
id: 'F8'
titel: 'Fake-Backend aus einer YAML-Datei'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-12'
order: 1
headless: true
status: aktiv
stufe: 1
umsetzung: vorhanden
implementiert: [D38, D16]
use_cases: [UC10]
---
# F8 — Fake-Backend aus einer YAML-Datei

{{ design_kopf(page) }}

Das Fake-Backend liest Vaults, Items und Felder aus einer YAML-Datei. Es kann die Zustände „gesperrt“ und „nicht erreichbar“ nachstellen. Die Testdaten sind neutral.


Setzt um: {{ ref("D38") }}, {{ ref("D16") }}.
