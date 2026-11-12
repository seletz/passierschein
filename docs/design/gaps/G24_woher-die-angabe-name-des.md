---
render_macros: true
id: 'G24'
titel: 'Woher die Angabe „Name“ des Agents kommt, ist offen'
bereich: 'Grenzen und Identität'
lauf: '2.0'
stand: '2026-11-11'
order: 6
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G24 — Woher die Angabe „Name“ des Agents kommt, ist offen

{{ design_kopf(page) }}

{{ ref("D4") }} und {{ ref("T11") }} sprechen von der Angabe „Name“ des Agents. Die Nachricht `request_credentials` hat nach {{ ref("D41") }} nur Referenzen, Aufgabe und Begründung.

**Annahme der Entwicklung:** `a38 mcp` meldet den Namen des Agent-Werkzeugs einmal beim Aufbau der Verbindung, neben dem Namen der Sitzung ({{ ref("D25") }}). Die Quelle ist die Angabe des MCP-Clients. Der Name gilt als unbestätigt.

Offene Frage:

- Braucht die Freigabeanfrage den Namen des Agent-Werkzeugs neben dem Namen der Sitzung?

Stand 2026-11-12: Die abgenommene Oberfläche der Freigabe-App zeigt bei den Angaben des Agents die Zeile „Agent“ neben dem Namen der Sitzung (PR 46). {{ ref("D4") }} nennt den Namen als Angabe des Agents. Offen bleibt die Bestätigung der Quelle: die Angabe `clientInfo` des MCP-Clients.

**Quelle:** Entwicklung von Stufe 1, Issue 16 (Architektur und Paketgerüst), 2026-11-11.
