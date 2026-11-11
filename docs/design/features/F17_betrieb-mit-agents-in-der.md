---
render_macros: true
id: 'F17'
titel: 'Betrieb mit Agents in der VM'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: aktiv
stufe: 5
umsetzung: zu-bauen
implementiert: [D2, D3, D13, D14]
use_cases: [UC13]
---
# F17 — Betrieb mit Agents in der VM

{{ design_kopf(page) }}

Die VM erreicht den Agent-Endpunkt des Daemons. Der Transport ist offen ({{ ref("G1") }}). Der Daemon erkennt die VM und die Sitzung. Die Freigabeanfrage nennt beide als geprüfte Fakten.


Setzt um: {{ ref("D2") }}, {{ ref("D3") }}, {{ ref("D13") }}, {{ ref("D14") }}.
