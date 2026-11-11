---
render_macros: true
id: 'F5'
titel: 'Grants: Vergabe, Prüfung, Ablauf und Widerruf'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-12'
order: 2
headless: true
status: aktiv
stufe: 1
umsetzung: vorhanden
implementiert: [D8, D32, D33]
use_cases: [UC2, UC3, UC7]
---
# F5 — Grants: Vergabe, Prüfung, Ablauf und Widerruf

{{ design_kopf(page) }}

Der Daemon vergibt nach einer Freigabe einen Grant, prüft spätere Anfragen gegen die Grants der Sitzung, erkennt Abweichungen und beendet Grants. `a38 grants list` und `a38 grants revoke` nutzen den Admin-Socket.


Setzt um: {{ ref("D8") }}, {{ ref("D32") }}, {{ ref("D33") }}.
