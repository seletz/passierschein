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
  wer: Stefan
  datum: '2026-11-11'
  notiz: Vorschlag des Designlaufs, von Stefan im Review am 2026-11-11 abgenommen
---
# D33 — Ein Grant endet mit der Laufzeit, der Sitzung, dem Widerruf oder dem Neustart des Daemons

{{ design_kopf(page) }}

**Entscheidung:** Der Daemon hält Grants nur im Arbeitsspeicher. Ein Grant endet, wenn die Laufzeit abläuft, die Verbindung der Sitzung endet, der Mensch widerruft oder der Daemon neu startet.

**Begründung:** Ein Grant, der einen Neustart überlebt, braucht einen geschützten Speicher. Den gibt es lokal nicht. Der Preis ist eine neue Freigabe nach einem Neustart.
