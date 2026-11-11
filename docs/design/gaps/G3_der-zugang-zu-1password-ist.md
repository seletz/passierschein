---
render_macros: true
id: 'G3'
titel: 'Der Zugang zu 1Password ist nur aus der Doku bekannt und nicht praktisch getestet'
bereich: 'Backends'
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
# G3 — Der Zugang zu 1Password ist nur aus der Doku bekannt und nicht praktisch getestet

{{ design_kopf(page) }}

Der Spike hat auf Anweisung kein 1Password benutzt. {{ ref("D39") }} ruht deshalb auf der Doku.

Offene Fragen:

- Wie viele Vaults kann ein Service Account lesen? Es gibt etwa 25 dev-Vaults. Die Rechte eines Service Accounts sind nach dem Anlegen unveränderlich.
- Wo liegt das Token auf dem Mac: im Schlüsselbund, in einer Datei?
- Reichen die Grenzen für Anfragen pro Stunde?
- Wie verhält sich die Desktop-Integration unter einem LaunchAgent, und welchen Text zeigt ihr Dialog?
- Kann das SDK Namen von Items lesen, ohne Werte zu lesen?

**Quelle:** [Spike 1Password-Go-SDK](https://github.com/seletz/passierschein/issues/3), Abschnitt Offene Punkte.
