---
render_macros: true
id: 'G44'
titel: 'Was eine Teilauswahl der Variablen bei einem Kommando bedeutet'
bereich: 'Freigabe'
lauf: '2.0'
stand: '2026-11-16'
order: 7
headless: true
type: Gap
kind: design
status: geschlossen
beleg:
  wer: Stefan
  datum: '2026-11-16'
  quelle: ['#112']
register:
  owner: Stefan
---
# G44 — Was eine Teilauswahl der Variablen bei einem Kommando bedeutet

{{ design_kopf(page) }}

Der Entwurf in {{ ref("D65") }} zeigt je Variable ein Häkchen. {{ ref("D31") }} verlangt, dass kein Häkchen vorab gesetzt ist. {{ ref("D7") }} kennt ein Häkchen je Eintrag und eine Bestätigung. Das Design sagt nicht, was eine Freigabe bedeutet, die bei einem Kommando nur einen Teil der Variablen nennt.

Ein Kommando braucht jeden Wert aus `env`. `a38 mcp` startet das Kommando nur mit allen Werten ({{ ref("D61") }}). Eine Teilauswahl liefert also entweder ein Kommando ohne einen Wert, oder sie gibt nichts frei.

**Vorschlag (umgesetzt in Issue 98, zu bestätigen durch Stefan):**

- Die Freigabe-App aktiviert die Bestätigung erst, wenn jede Variable angehakt ist. Der Knopf lautet vorher „Select every variable“.
- Der Daemon nimmt eine Freigabe eines Kommandos nur an, wenn sie jeden Eintrag nennt. Jede andere Freigabe endet mit `ui_failed` (`approval.ValidateDecision`).
- Die Häkchen bleiben. Der Mensch bestätigt so jedes Zugangsdatum einzeln, wie bei {{ ref("D7") }}.

Offene Fragen:

- Bestätigt Stefan den Vorschlag?
- Oder soll ein Kommando gar keine Häkchen haben und nur eine Bestätigung für alle Variablen?

**Quelle:** [Issue 98](https://github.com/seletz/passierschein/issues/98), Umsetzung der Freigabeanfrage für ein Kommando.
