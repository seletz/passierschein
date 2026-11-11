---
render_macros: true
id: 'G29'
titel: 'Welche Ereignisart das Audit-Log je Ausgang schreibt, ist nicht festgelegt'
bereich: 'Audit'
lauf: '2.0'
stand: '2026-11-11'
order: 5
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G29 — Welche Ereignisart das Audit-Log je Ausgang schreibt, ist nicht festgelegt

{{ design_kopf(page) }}

{{ ref("D36") }} nennt die Ereignisarten. Das Design sagt nicht für jeden Ausgang, welche Einträge entstehen.

**Annahme der Entwicklung:**

| Ausgang | Einträge nach Anfrage und Bündel |
|---|---|
| Freigabe | Entscheidung, Grant, Auslieferung |
| Ablehnung durch den Menschen | Ablehnung, keine Entscheidung |
| Zeitüberschreitung | Ablauf mit dem Zusatz `timeout` |
| Fehler der Freigabe | Fehler |
| Teilauswahl | Entscheidung, Grant, Auslieferung, danach Ablehnung für die nicht angehakten Einträge |
| Ablauf eines Grants | Ablauf; der Daemon schreibt ihn spätestens nach 30 Sekunden oder bei der nächsten Anfrage |

Offene Frage:

- Braucht die Auswertung bei einer Ablehnung zusätzlich einen Eintrag der Ereignisart Entscheidung?

**Quelle:** Entwicklung von Stufe 1, Broker und MCP-Server (Issues 21, 22, 24), 2026-11-11.
