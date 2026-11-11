---
render_macros: true
id: 'G11'
titel: 'Der Ablauf der Auslieferung als Datei auf einem tmpfs ist nicht entworfen'
bereich: 'Grants und Auslieferung'
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
# G11 — Der Ablauf der Auslieferung als Datei auf einem tmpfs ist nicht entworfen

{{ design_kopf(page) }}

{{ ref("D10") }} legt das Ziel fest. Der Weg fehlt.

Offene Fragen:

- Wer schreibt die Datei in der VM? Der Daemon läuft auf dem Mac.
- Wer löscht die Datei nach der Laufzeit?
- Wie verhindert der Entwurf, dass der Agent die Datei liest und den Wert in seinen Kontext holt?

**Quelle:** Projektnotiz, Ablauf Schritt 5.
