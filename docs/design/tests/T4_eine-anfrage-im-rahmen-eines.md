---
render_macros: true
id: 'T4'
titel: 'Eine Anfrage im Rahmen eines Grants braucht keine Freigabe, nach der Laufzeit wieder'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [F5, D8]
---
# T4 — Eine Anfrage im Rahmen eines Grants braucht keine Freigabe, nach der Laufzeit wieder

{{ design_kopf(page) }}

**Form:** Go-Test mit einer gefälschten Uhr.

**Schritte:** Freigabe für ein Zugangsdatum. Zweite Anfrage für dasselbe Zugangsdatum. Die Uhr springt hinter die Laufzeit. Dritte Anfrage.

**Erwartung:** Die zweite Anfrage erzeugt keine Freigabeanfrage. Die dritte erzeugt eine. Das Audit-Log enthält den Ablauf des Grants.


Belegt: {{ ref("F5") }}, {{ ref("D8") }}.
