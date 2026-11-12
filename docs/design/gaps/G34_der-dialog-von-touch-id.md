---
render_macros: true
id: 'G34'
titel: 'Der Dialog von Touch ID mischt die Sprache des Systems mit englischem Text'
bereich: 'Freigabe'
lauf: '2.0'
stand: '2026-11-12'
order: 6
headless: true
type: Gap
kind: design
status: geschlossen
beleg:
  wer: Stefan
  datum: '2026-11-12'
  quelle: ['#39']
  notiz: 'im Praxistest am 2026-11-12 entschieden: der Text folgt der Sprache des Systems'
register:
  owner: Stefan
---
# G34 — Der Dialog von Touch ID mischt die Sprache des Systems mit englischem Text

{{ design_kopf(page) }}

Der Dialog von Touch ID nennt die App „Passierschein“. macOS baut den Satz aus seinem eigenen Rahmen in der Sprache des Systems und aus dem Text der App. Auf einem deutschen System steht dort: „Passierschein möchte approve 1 credential for project praxistest.“ Der Knopf „Back“ stammt ebenfalls von der App.

Die Sprache der Oberfläche legt die Entwicklung fest (Abnahme des Popups, 2026-11-11).

**Entschieden (Stefan, 2026-11-12):** Der Text der App für diesen Dialog und der Knopf zum Abbrechen folgen der Sprache des Systems. Auf einem deutschen System steht dort: „Passierschein möchte 1 Zugangsdatum für das Projekt praxistest freigeben.“ Der Knopf heißt „Zurück“. Das Popup selbst bleibt englisch.

**Quelle:** Praxistest mit Stefan, 2026-11-12, Issue 39.
