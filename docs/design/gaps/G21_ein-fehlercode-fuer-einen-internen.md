---
render_macros: true
id: 'G21'
titel: 'Ein Fehlercode für einen internen Fehler des Daemons fehlt'
bereich: 'Anfrage und Bündelung'
lauf: '2.0'
stand: '2026-11-11'
order: 2
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G21 — Ein Fehlercode für einen internen Fehler des Daemons fehlt

{{ design_kopf(page) }}

{{ ref("D35") }} nennt für jeden bekannten Fehler einen Code. Für einen internen Fehler des Daemons, der kein Fehler des Backends, der Freigabe oder des Audit-Logs ist, gibt es keinen Code.

**Annahme der Entwicklung:** Ein solcher Fehler endet mit `daemon_faulted` und löst die globale Sperre aus ({{ ref("D48") }}). Das ist die sichere Richtung.

Offene Frage:

- Bestätigt Stefan diese Annahme, oder braucht der Fall einen eigenen Code ohne Sperre?

**Quelle:** Entwicklung von Stufe 1, Issue 16 (Architektur und Paketgerüst), 2026-11-11.
