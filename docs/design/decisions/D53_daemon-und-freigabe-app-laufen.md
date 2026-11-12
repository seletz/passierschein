---
render_macros: true
id: 'D53'
titel: 'Daemon und Freigabe-App laufen nur auf macOS, Linux ist nur Ziel für den Client'
bereich: 'Betrieb und Release'
lauf: '2.0'
stand: '2026-11-12'
order: 4
headless: true
status: gueltig
use_cases: [UC13, UC11]
beleg:
  wer: Stefan
  datum: '2026-11-12'
  notiz: im Gespräch am 2026-11-12 festgelegt
---
# D53 — Daemon und Freigabe-App laufen nur auf macOS, Linux ist nur Ziel für den Client

{{ design_kopf(page) }}

**Entscheidung:** Der Daemon `a38 serve` und die Freigabe-App „Passierschein“ haben genau eine Zielplattform: macOS. Linux ist nur Zielplattform für den Client, also für `a38 mcp` in der VM des Agents ({{ ref("UC13") }}, {{ ref("F17") }}).

**Begründung:** Touch ID und der macOS-Schlüsselbund gibt es nur auf dem Mac. 1Password gibt es auch unter Linux, aber ohne diese beiden Bausteine fehlt die Freigabe durch den Menschen ({{ ref("D7") }}) und der Schutz des Tokens.

**Folgen:**

- Das Release enthält für Linux nur das Programm `a38` ohne das 1Password-Backend und ohne die Freigabe-App ({{ ref("D42") }}, {{ ref("D2") }}).
- Unter Linux startet `a38 serve` nur mit dem Fake-Backend. Das dient der Entwicklung und der CI ({{ ref("D38") }}, {{ ref("D40") }}). Es ist kein unterstützter Betrieb.
- Ein Daemon unter Linux ist kein Ziel des Projekts. Eine Freigabe-Oberfläche für Linux entsteht nicht.

**Quelle:** Stefan, 2026-11-12, im Gespräch zum Token im Schlüsselbund.
