---
render_macros: true
id: 'D32'
titel: 'Ein Grant umfasst die angehakten Zugangsdaten, mit einer konfigurierbaren Laufzeit'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 6
headless: true
status: gueltig
use_cases: [UC2, UC3]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Vorschlag des Designlaufs, von Stefan im Review am 2026-11-11 abgenommen
---
# D32 — Ein Grant umfasst die angehakten Zugangsdaten, mit einer konfigurierbaren Laufzeit

{{ design_kopf(page) }}

**Entscheidung:** Ein Grant gilt für eine Sitzung und für genau die Zugangsdaten, die der Mensch angehakt hat. Die Laufzeit ist ein Wert der Konfiguration. Er gilt global für `a38`. Der Standard ist vier Stunden. Der Mensch kann in der Freigabeanfrage eine kürzere Laufzeit wählen. Liefert die Freigabe eine längere Laufzeit als die Konfiguration, vergibt der Daemon keinen Grant und meldet `ui_failed`.

**Begründung:** Der kleinste Umfang ist der sichere Standard.

**Offen:** Die Projektnotiz nennt als Beispiel einen Grant für ein Projekt, einen dev-Vault und 4 h. Das spricht für einen Grant über einen ganzen Vault ({{ ref("G7") }}).
