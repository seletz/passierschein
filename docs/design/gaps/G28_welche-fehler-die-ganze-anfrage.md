---
render_macros: true
id: 'G28'
titel: 'Welche Fehler die ganze Anfrage beenden, ist nicht festgelegt'
bereich: 'Anfrage und Bündelung'
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
# G28 — Welche Fehler die ganze Anfrage beenden, ist nicht festgelegt

{{ design_kopf(page) }}

{{ ref("D28") }} legt für die Policy „alles oder nichts“ fest. Für die anderen Fehler sagt das Design nicht, ob sie die ganze Anfrage beenden.

**Annahme der Entwicklung:** Eine Ablehnung, eine Zeitüberschreitung, ein Fehler der Freigabe, ein Fehler des Audit-Logs, ein Fehler des Backends und das Ende der Sitzung beenden die ganze Anfrage, auch für Einträge unter einem Grant. Nur zwei Ergebnisse gelten je Eintrag: `denied` für nicht angehakte Einträge einer Teilauswahl ({{ ref("UC4") }}) und `bundle_too_large` ({{ ref("D29") }}).

Offene Frage:

- Soll eine Anfrage die Einträge unter einem gültigen Grant trotzdem liefern, wenn der Mensch die neuen Einträge ablehnt?

**Quelle:** Entwicklung von Stufe 1, Broker und MCP-Server (Issues 21, 22, 24), 2026-11-11.
