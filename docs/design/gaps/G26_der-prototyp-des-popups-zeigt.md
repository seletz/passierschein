---
render_macros: true
id: 'G26'
titel: 'Der Prototyp des Popups zeigt Fälle, die das Design ausschließt'
bereich: 'Freigabe'
lauf: '2.0'
stand: '2026-11-11'
order: 4
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G26 — Der Prototyp des Popups zeigt Fälle, die das Design ausschließt

{{ design_kopf(page) }}

Der Prototyp aus dem Spike zur Freigabe-UI zeigt zwei Fälle, die das Design ausschließt:

- Eine Anfrage außerhalb von dev erreicht im MVP nie die Freigabe ({{ ref("D24") }}, {{ ref("D28") }}).
- Der Prototyp nennt als Umfang eines Grants das Projekt und die Vaults. Nach {{ ref("D32") }} umfasst ein Grant genau die angehakten Zugangsdaten.

**Annahme der Entwicklung:** Die Schnittstelle zur Freigabe führt den Umfang nur als Anzeige weiter. Die Abweichung „außerhalb des Umfangs“ entfällt.

Das betrifft die Stufe 2 ({{ ref("F11") }}).

Offene Frage:

- Welchen Text zeigt die Freigabe-App zum Umfang des Grants?

**Quelle:** Entwicklung von Stufe 1, Issue 16 (Architektur und Paketgerüst), 2026-11-11.
