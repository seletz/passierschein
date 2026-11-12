---
render_macros: true
id: 'F12'
titel: '1Password-Backend über das Go-SDK'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-12'
order: 2
headless: true
status: aktiv
stufe: 3
umsetzung: vorhanden
implementiert: [D1, D12, D39]
use_cases: [UC1, UC11]
---
# F12 — 1Password-Backend über das Go-SDK

{{ design_kopf(page) }}

Das Backend löst Namen auf und liest Werte über das Go-SDK. Es meldet die Zustände „gesperrt“ und „nicht erreichbar“ als eigene Fehler.


Setzt um: {{ ref("D1") }}, {{ ref("D12") }}, {{ ref("D39") }}.
