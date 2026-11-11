---
render_macros: true
id: 'F1'
titel: 'Agent-Socket, Admin-Socket und Sitzungen'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
stufe: 1
umsetzung: zu-bauen
implementiert: [D3, D25, D26, D43, D44]
use_cases: [UC1, UC7]
---
# F1 — Agent-Socket, Admin-Socket und Sitzungen

{{ design_kopf(page) }}

Der Daemon stellt den Agent-Socket und den Admin-Socket bereit. Er nimmt Verbindungen an, liest die Peer-Daten, vergibt die Kennung der Sitzung und prüft jede Nachricht gegen das Schema.

**Abgrenzung:** Die Erkennung von Sitzungen in der VM gehört zu {{ ref("F17") }}.


Setzt um: {{ ref("D3") }}, {{ ref("D25") }}, {{ ref("D26") }}, {{ ref("D43") }}, {{ ref("D44") }}.
