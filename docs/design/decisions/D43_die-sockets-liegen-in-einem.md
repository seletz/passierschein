---
render_macros: true
id: 'D43'
titel: 'Die Sockets liegen in einem Verzeichnis nur für den Benutzer'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 9
headless: true
status: gueltig
use_cases: [UC1]
beleg:
  notiz: Vorschlag des Designlaufs, zu bestätigen durch Stefan
  annahme: true
---
# D43 — Die Sockets liegen in einem Verzeichnis nur für den Benutzer

{{ design_kopf(page) }}

**Entscheidung:** Die Sockets liegen in einem Verzeichnis mit den Rechten `0700`. Jeder Socket hat die Rechte `0600`. Der Daemon prüft die Rechte beim Start und bricht bei einer Abweichung ab.

**Begründung:** Kein anderer Benutzer des Macs erreicht den Daemon.
