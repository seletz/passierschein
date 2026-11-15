---
render_macros: true
id: 'D32'
titel: 'Ein Grant umfasst die angehakten Zugangsdaten, mit einer konfigurierbaren Laufzeit'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-15'
order: 6
headless: true
status: ersetzt
ersetzt_durch: D63
use_cases: [UC2, UC3]
beleg:
  datum: '2026-11-15'
  quelle: ['#91']
  notiz: Vorschlag aus Issue 91, zu bestätigen durch Stefan
  annahme: true
---
# D32 — Ein Grant umfasst die angehakten Zugangsdaten, mit einer konfigurierbaren Laufzeit

{{ design_kopf(page) }}

**Entscheidung:** Ein Grant gilt für eine Sitzung und für genau die Zugangsdaten, die der Mensch angehakt hat. Die Laufzeit ist ein Wert der Konfiguration. Er gilt global für `a38`. Der Standard ist vier Stunden. Der Mensch kann in der Freigabeanfrage eine kürzere Laufzeit wählen. Liefert die Freigabe eine längere Laufzeit als die Konfiguration, vergibt der Daemon keinen Grant und meldet `ui_failed`.

**Begründung:** Der kleinste Umfang ist der sichere Standard.

**Offen:** Die Projektnotiz nennt als Beispiel einen Grant für ein Projekt, einen dev-Vault und 4 h. Das spricht für einen Grant über einen ganzen Vault ({{ ref("G7") }}).
