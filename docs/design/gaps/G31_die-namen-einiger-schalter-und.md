---
render_macros: true
id: 'G31'
titel: 'Die Namen einiger Schalter und Befehle stammen aus der Entwicklung'
bereich: 'Betrieb und Release'
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
# G31 — Die Namen einiger Schalter und Befehle stammen aus der Entwicklung

{{ design_kopf(page) }}

Die Entwicklung hat Namen gewählt, die das Design nicht nennt:

| Name | Zweck |
|---|---|
| `a38 serve --fake-file <datei>` | YAML-Datei des Fake-Backends ({{ ref("D38") }}) |
| `a38 serve --ui-script <programm>` | Programm für `--ui script` ({{ ref("D40") }}) |
| `a38 mcp --session-name <name>` und `A38_SESSION_NAME` | Name der Sitzung ({{ ref("D25") }}, {{ ref("G19") }}); das Argument hat Vorrang |
| `a38 fault status` und `a38 fault clear` | Sperre ansehen und aufheben ({{ ref("D48") }}, {{ ref("G20") }}) |

Offene Frage:

- Bestätigt Stefan diese Namen?

**Quelle:** Entwicklung von Stufe 1, Broker und MCP-Server (Issues 21, 22, 24), 2026-11-11.
