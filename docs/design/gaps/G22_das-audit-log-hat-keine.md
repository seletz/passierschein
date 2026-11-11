---
render_macros: true
id: 'G22'
titel: 'Das Audit-Log hat keine Ereignisart für Beginn und Ende der globalen Sperre'
bereich: 'Audit'
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
# G22 — Das Audit-Log hat keine Ereignisart für Beginn und Ende der globalen Sperre

{{ design_kopf(page) }}

{{ ref("D48") }} verlangt Beginn und Ende der globalen Sperre im Audit-Log. {{ ref("D36") }} kennt neun Ereignisarten und keine für die Sperre.

**Annahme der Entwicklung:** Beginn und Ende sind Einträge der Ereignisart Fehler mit dem Zusatz `fault_begin` und `fault_end`.

Offene Frage:

- Reicht das für die Auswertung, oder braucht die Sperre eine eigene Ereignisart?

**Quelle:** Entwicklung von Stufe 1, Issue 16 (Architektur und Paketgerüst), 2026-11-11.
