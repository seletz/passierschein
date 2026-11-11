---
render_macros: true
id: 'G17'
titel: 'Format und Ort der Konfiguration sind offen'
bereich: 'Betrieb und Release'
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
# G17 — Format und Ort der Konfiguration sind offen

{{ design_kopf(page) }}

{{ ref("D28") }} braucht eine Datei mit Projekten, erlaubten Vaults und Laufzeiten.

Offene Fragen:

- Welches Format hat die Datei, und wo liegt sie?
- Wie erkennt der Daemon ein Projekt: über den Pfad, über das Git-Remote?
- Lädt der Daemon eine Änderung im Betrieb neu?

**Quelle:** Designlauf 1.
