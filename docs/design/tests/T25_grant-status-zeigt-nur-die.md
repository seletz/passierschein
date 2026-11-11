---
render_macros: true
id: 'T25'
titel: 'grant_status zeigt nur die Grants der eigenen Sitzung und nie einen Wert'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 6
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [D41, F9]
---
# T25 — grant_status zeigt nur die Grants der eigenen Sitzung und nie einen Wert

{{ design_kopf(page) }}

**Form:** Go-Test mit zwei Sitzungen.

**Erwartung:** `grant_status` zeigt die Grants der eigenen Sitzung mit der Restlaufzeit. Es zeigt keinen Grant der anderen Sitzung und keinen Wert. Nach dem Ablauf und nach dem Widerruf zeigt es den Grant nicht mehr.


Belegt: {{ ref("D41") }}, {{ ref("F9") }}.
