---
render_macros: true
id: 'G18'
titel: 'Ob ein Bündel zu viele Zugangsdaten für eine Freigabeanfrage zeigt, muss der Praxistest zeigen'
bereich: 'Freigabe'
lauf: '1.0'
stand: '2026-11-11'
order: 3
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G18 — Ob ein Bündel zu viele Zugangsdaten für eine Freigabeanfrage zeigt, muss der Praxistest zeigen

{{ design_kopf(page) }}

Ein Bündel kann viele Zugangsdaten in einer Freigabeanfrage zeigen. Der Mensch liest eine lange Liste möglicherweise nicht mehr ({{ ref("RI3") }}, {{ ref("G12") }}). Stefan teilt diesen Einwand. Er hat am 2026-11-11 festgelegt, dass der Praxistest die Antwort gibt und nicht eine Vorab-Regel.

Offene Fragen:

- Ab wie vielen Zugangsdaten liest der Mensch die Liste nicht mehr?
- Braucht ein Bündel eine Obergrenze, oder teilt der Daemon ein großes Bündel in mehrere Freigabeanfragen?
- Fördert die Schaltfläche „Select all“ je Vault das Bestätigen ohne Lesen?

**Quelle:** Spike zur Freigabe-UI, Risiko „Lange Bündel“; Rückmeldung von Stefan zum Design.
