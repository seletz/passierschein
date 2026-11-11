---
render_macros: true
id: 'F10'
titel: 'Stabile Fehlercodes und Ablehnung bei jedem Fehler'
bereich: 'Anfrage und Bündelung'
lauf: '2.0'
stand: '2026-11-12'
order: 2
headless: true
status: aktiv
stufe: 1
umsetzung: vorhanden
implementiert: [D35]
use_cases: [UC14, UC4]
---
# F10 — Stabile Fehlercodes und Ablehnung bei jedem Fehler

{{ design_kopf(page) }}

Jeder Fehlerweg endet mit einem der festen Codes. Kein Fehlerweg liefert einen Wert aus. Keine Fehlermeldung enthält einen Secret-Wert.


Setzt um: {{ ref("D35") }}.
