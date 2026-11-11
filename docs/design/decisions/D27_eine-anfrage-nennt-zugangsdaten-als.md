---
render_macros: true
id: 'D27'
titel: 'Eine Anfrage nennt Zugangsdaten als Referenz aus Vault, Item und Feld'
bereich: 'Anfrage und Bündelung'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: gueltig
use_cases: [UC1]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Vorschlag des Designlaufs, von Stefan im Review am 2026-11-11 abgenommen
---
# D27 — Eine Anfrage nennt Zugangsdaten als Referenz aus Vault, Item und Feld

{{ design_kopf(page) }}

**Entscheidung:** Der Agent nennt jedes Zugangsdatum als Referenz der Form `op://<vault>/<item>/<feld>`. Der Daemon löst die Referenz im Backend auf. Die Freigabeanfrage zeigt die Namen, die das Backend liefert, nicht den Text des Agents.

**Begründung:** Die Form ist in 1Password üblich. Der Mensch sieht geprüfte Namen.

**Offen:** Wie der Agent die Namen der Items findet ({{ ref("G6") }}).
