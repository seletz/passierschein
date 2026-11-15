---
render_macros: true
id: 'G37'
titel: 'Ein Kommando mit Eingabe über stdin oder ein interaktives Kommando ist nicht entworfen'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-15'
order: 5
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G37 — Ein Kommando mit Eingabe über stdin oder ein interaktives Kommando ist nicht entworfen

{{ design_kopf(page) }}

{{ ref("D61") }} startet das Kommando mit leerem stdin. Manche Werkzeuge lesen ein Secret nur über stdin oder fragen interaktiv, zum Beispiel `docker login --password-stdin`.

Offene Fragen:

- Welche Kommandos der Agenten brauchen stdin? Das zeigt der Einsatz mit echten Projekten.
- Soll `env` auch „Wert nach stdin“ erlauben, etwa mit einem reservierten Namen?
- Bleibt ein interaktives Kommando ausgeschlossen?

Bis zur Klärung hilft `sh -c` mit einer Umleitung aus der Variable. Das braucht eine Freigabe je Kommando ({{ ref("D63") }}).

**Quelle:** [Issue 91](https://github.com/seletz/passierschein/issues/91).
