---
render_macros: true
id: 'D8'
titel: 'Ein Grant gilt für eine Sitzung und hat eine Laufzeit'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: gueltig
use_cases: [UC2, UC3]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Projektnotiz vom 2026-11-11
---
# D8 — Ein Grant gilt für eine Sitzung und hat eine Laufzeit

{{ design_kopf(page) }}

**Entscheidung:** Eine Freigabe erzeugt einen Grant für die Sitzung. Der Grant hat eine Laufzeit. Eine spätere Anfrage im Rahmen des Grants braucht kein Popup. Eine Anfrage außerhalb des Grants ist eine Abweichung und sieht in der Freigabeanfrage anders aus.

**Begründung:** Wenige, unterscheidbare Freigabeanfragen wirken gegen blindes Bestätigen ({{ ref("RI3") }}).

**Quelle:** Projektnotiz, Ablauf.
