---
render_macros: true
id: 'T16'
titel: 'Ein Grant endet mit der Sitzung, dem Widerruf und dem Neustart des Daemons'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-12'
order: 4
headless: true
status: aktiv
umsetzung: teilweise
beweist: [D33, F5]
---
# T16 — Ein Grant endet mit der Sitzung, dem Widerruf und dem Neustart des Daemons

{{ design_kopf(page) }}

**Form:** Go-Test.

**Fälle:** Die Verbindung der Sitzung endet; `a38 grants revoke`; der Daemon startet neu.

**Erwartung:** Nach jedem Fall braucht dieselbe Anfrage eine neue Freigabe. Das Audit-Log nennt den Grund für das Ende.


Belegt: {{ ref("D33") }}, {{ ref("F5") }}.
