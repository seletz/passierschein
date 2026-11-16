---
render_macros: true
id: 'G43'
titel: 'D68 nennt drei Sätze des Skills nicht, die mit dem exec-Weg falsch werden'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-15'
order: 3
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G43 — D68 nennt drei Sätze des Skills nicht, die mit dem exec-Weg falsch werden

{{ design_kopf(page) }}

{{ ref("D68") }} ersetzt zwei Regeln und einen Satz des Skills und sagt: „Die übrigen Teile bleiben, wie sie sind.“ Drei weitere Sätze des Skills widersprechen aber dem exec-Weg:

- „If the human agrees, you get the value.“ Mit `run_with_credentials` erhält der Agent den Wert nie.
- „You use two MCP tools of the server `a38`: `request_credentials` and `grant_status`.“ Es gibt jetzt drei Werkzeuge ({{ ref("D61") }}).
- Beim Code `not_allowed`: „In this version it also means that the vault is not a dev vault.“ Dieser Satz gehört zur ersetzten Regel „In this version a38 delivers only credentials of dev vaults“. Für einen Vault von test oder prod antwortet `request_credentials` jetzt mit `exec_required`.

Ein Agent, der diese Sätze liest, wählt den falschen Weg oder deutet `not_allowed` falsch.

**Vorschlag (umgesetzt in Issue 101, zu bestätigen durch Stefan):**

- Der erste Satz lautet: „If the human agrees, a38 runs your command with the value, or you get the value.“
- Der zweite Satz nennt drei Werkzeuge: `run_with_credentials`, `request_credentials` und `grant_status`.
- Der Satz beim Code `not_allowed` entfällt.

Offene Fragen:

- Bestätigt Stefan die drei Änderungen?

**Quelle:** [Issue 101](https://github.com/seletz/passierschein/issues/101), Umsetzung von {{ ref("D68") }}.
