---
render_macros: true
id: 'F14'
titel: 'Skill für Agents aus dem Binary'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-12'
order: 2
headless: true
status: aktiv
stufe: 1
umsetzung: vorhanden
implementiert: [D22, D5]
use_cases: [UC9]
---
# F14 — Skill für Agents aus dem Binary

{{ design_kopf(page) }}

`a38 llm --skill` gibt die eingebettete Anleitung aus. Die Anleitung nennt die Werkzeuge, die Fehlercodes und die Regeln für den Umgang mit Werten.


Setzt um: {{ ref("D22") }}, {{ ref("D5") }}.
