---
render_macros: true
id: 'F3'
titel: 'Policy-Prüfung vor der Freigabe'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
stufe: 1
umsetzung: zu-bauen
implementiert: [D12, D24, D28, D47]
use_cases: [UC1, UC5]
---
# F3 — Policy-Prüfung vor der Freigabe

{{ design_kopf(page) }}

Der Daemon prüft jede Referenz: Der Vault ist für `a38` sichtbar, das Projekt der Sitzung darf den Vault nutzen, der Vault gehört zu dev. Eine Anfrage, die eine Regel verletzt, erreicht den Menschen nicht.


Setzt um: {{ ref("D12") }}, {{ ref("D24") }}, {{ ref("D28") }}, {{ ref("D47") }}.
