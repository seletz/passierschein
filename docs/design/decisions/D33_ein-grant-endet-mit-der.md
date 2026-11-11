---
render_macros: true
id: 'D33'
titel: 'Ein Grant endet mit der Laufzeit, der Sitzung, dem Widerruf oder dem Neustart des Daemons'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 7
headless: true
status: gueltig
use_cases: [UC7, UC2]
beleg:
  notiz: Vorschlag des Designlaufs, zu bestätigen durch Stefan
  annahme: true
---
# D33 — Ein Grant endet mit der Laufzeit, der Sitzung, dem Widerruf oder dem Neustart des Daemons

{{ design_kopf(page) }}

**Entscheidung:** Der Daemon hält Grants nur im Arbeitsspeicher. Ein Grant endet, wenn die Laufzeit abläuft, die Verbindung der Sitzung endet, der Mensch widerruft oder der Daemon neu startet.

**Begründung:** Ein Grant, der einen Neustart überlebt, braucht einen geschützten Speicher. Den gibt es lokal nicht. Der Preis ist eine neue Freigabe nach einem Neustart.
