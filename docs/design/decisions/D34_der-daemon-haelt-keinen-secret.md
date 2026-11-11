---
render_macros: true
id: 'D34'
titel: 'Der Daemon hält keinen Secret-Wert vor und schreibt ihn nie in ein Log'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 8
headless: true
status: gueltig
use_cases: [UC1, UC14]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Vorschlag des Designlaufs, von Stefan im Review am 2026-11-11 abgenommen
---
# D34 — Der Daemon hält keinen Secret-Wert vor und schreibt ihn nie in ein Log

{{ design_kopf(page) }}

**Entscheidung:** Der Daemon liest einen Secret-Wert erst für die Auslieferung. Er hält den Wert nicht in einem Cache. Ein Secret-Wert steht nie im Log, im Audit-Log, in einer Fehlermeldung oder in der Freigabeanfrage. Das Audit-Log speichert die Referenz.

**Begründung:** Der Daemon soll bei einem Fehler möglichst wenig preisgeben ({{ ref("RI4") }}).

**Quelle:** Projektnotiz, Entwicklungsregeln: Secret-Werte nie in Log, Fehlermeldungen oder Testausgaben.
