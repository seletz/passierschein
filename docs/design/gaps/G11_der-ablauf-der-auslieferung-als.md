---
render_macros: true
id: 'G11'
titel: 'Der Ablauf der Auslieferung als Datei auf einem tmpfs ist nicht entworfen'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-15'
order: 2
headless: true
type: Gap
kind: design
status: geschlossen
beleg:
  datum: '2026-11-15'
  durch: [D62]
  quelle: ['#91']
  notiz: Vorschlag aus Issue 91, zu bestätigen durch Stefan
  annahme: true
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
