---
render_macros: true
id: 'G2'
titel: 'Ob eine Verbindung je Agent unter herdr die richtige Sitzung ist, ist offen'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G2 — Ob eine Verbindung je Agent unter herdr die richtige Sitzung ist, ist offen

{{ design_kopf(page) }}

{{ ref("D25") }} setzt die Sitzung mit der Verbindung eines Prozesses `a38 mcp` gleich. Unter herdr laufen viele Agents zur gleichen Zeit.

Offene Fragen:

- Startet jedes Agent-Werkzeug einen eigenen Prozess `a38 mcp`, auch für Unteragenten?
- Ist das Arbeitsverzeichnis ein verlässlicher Hinweis auf das Projekt, auch in einem Worktree?
- Soll ein Grant einen Neustart des Agents überleben?

**Quelle:** Designlauf 1.
