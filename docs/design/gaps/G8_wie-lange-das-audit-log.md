---
render_macros: true
id: 'G8'
titel: 'Wie lange das Audit-Log Einträge behält, ist offen'
bereich: 'Audit'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G8 — Wie lange das Audit-Log Einträge behält, ist offen

{{ design_kopf(page) }}

Die Begründung eines Agents kann vertrauliche Inhalte enthalten, zum Beispiel aus einem Ticket. Eine Hash-Kette verträgt das Löschen einzelner Einträge nicht.

Offene Fragen:

- Wie lange bleiben Einträge erhalten?
- Löscht `a38` ganze Zeiträume und beginnt die Kette mit einem Anker neu?
- Kürzt der Daemon die Begründung auf eine feste Länge?

**Quelle:** kritische Prüfung, Hinweis zum Datenschutz im Log.
