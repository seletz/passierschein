---
render_macros: true
id: 'G5'
titel: 'Die Signatur der Freigabe-App ist nicht mit der echten Developer ID getestet'
bereich: 'Betrieb und Release'
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
# G5 — Die Signatur der Freigabe-App ist nicht mit der echten Developer ID getestet

{{ design_kopf(page) }}

{{ ref("D46") }} ruht auf der Doku und auf einem Versuch mit einer Ad-hoc-Signatur.

Offene Fragen:

- Besteht das Bundle die Notarisierung, und bleibt das angeheftete Ticket in einem tar-Archiv erhalten?
- Wie legt `mise` mit dem Backend `github:` ein Archiv mit Binary und Bundle ab, und wie findet der Daemon die App?
- Prüft der Daemon vor dem Start die Signatur der App?
- Welche Swift-Version nutzt der macOS-Runner, und wie legt `mise.toml` sie fest?

**Quelle:** [Spike Signatur der Swift-App](https://github.com/seletz/passierschein/issues/5), Abschnitt Offene Punkte.
