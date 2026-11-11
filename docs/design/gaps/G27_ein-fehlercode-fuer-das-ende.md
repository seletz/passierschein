---
render_macros: true
id: 'G27'
titel: 'Ein Fehlercode für das Ende der Sitzung während einer Anfrage fehlt'
bereich: 'Anfrage und Bündelung'
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
# G27 — Ein Fehlercode für das Ende der Sitzung während einer Anfrage fehlt

{{ design_kopf(page) }}

Endet die Verbindung einer Sitzung, während ihre Anfrage läuft, braucht die Anfrage ein Ergebnis. {{ ref("D35") }} hat dafür keinen Code.

**Annahme der Entwicklung:** Die Anfrage endet mit `denied`. Das Audit-Log enthält einen Eintrag der Ereignisart Ablauf mit dem Zusatz `session_ended`. Der Daemon zeigt für eine beendete Sitzung keine Freigabeanfrage und vergibt keinen Grant ({{ ref("D33") }}).

Offene Frage:

- Braucht der Fall einen eigenen Code?

**Quelle:** Entwicklung von Stufe 1, Broker und MCP-Server (Issues 21, 22, 24), 2026-11-11.
