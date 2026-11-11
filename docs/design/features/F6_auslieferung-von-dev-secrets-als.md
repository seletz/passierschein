---
render_macros: true
id: 'F6'
titel: 'Auslieferung von dev-Secrets als MCP-Antwort'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 3
headless: true
status: aktiv
stufe: 1
umsetzung: zu-bauen
implementiert: [D9, D34]
use_cases: [UC1, UC2]
---
# F6 — Auslieferung von dev-Secrets als MCP-Antwort

{{ design_kopf(page) }}

Der Daemon liest die freigegebenen Werte im Backend und gibt sie als Antwort zurück. Der Daemon hält die Werte danach nicht vor.


Setzt um: {{ ref("D9") }}, {{ ref("D34") }}.
