---
render_macros: true
id: 'D32'
titel: 'Ein Grant umfasst die angehakten Zugangsdaten, mit einer Laufzeit von vier Stunden'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 6
headless: true
status: gueltig
use_cases: [UC2, UC3]
beleg:
  notiz: Vorschlag des Designlaufs, zu bestätigen durch Stefan
  annahme: true
---
# D32 — Ein Grant umfasst die angehakten Zugangsdaten, mit einer Laufzeit von vier Stunden

{{ design_kopf(page) }}

**Entscheidung:** Ein Grant gilt für eine Sitzung und für genau die Zugangsdaten, die der Mensch angehakt hat. Die Laufzeit ist vier Stunden. Der Mensch kann in der Freigabeanfrage eine kürzere Laufzeit wählen. Die Obergrenze ist acht Stunden.

**Begründung:** Der kleinste Umfang ist der sichere Standard.

**Offen:** Die Projektnotiz nennt als Beispiel einen Grant für ein Projekt, einen dev-Vault und 4 h. Das spricht für einen Grant über einen ganzen Vault ({{ ref("G7") }}).
