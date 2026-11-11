---
render_macros: true
id: 'D7'
titel: 'Ein Bündel braucht genau eine Bestätigung mit Touch ID, mit einem Häkchen je Zugangsdatum'
bereich: 'Freigabe'
lauf: '1.0'
stand: '2026-11-11'
order: 3
headless: true
status: gueltig
use_cases: [UC1, UC4]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Projektnotiz vom 2026-11-11
---
# D7 — Ein Bündel braucht genau eine Bestätigung mit Touch ID, mit einem Häkchen je Zugangsdatum

{{ design_kopf(page) }}

**Entscheidung:** Die Freigabeanfrage zeigt ein Häkchen je Zugangsdatum. Der Mensch bestätigt das Bündel einmal mit Touch ID. Eine zweite Bestätigung gibt es nicht.

**Begründung:** Eine zweite Bestätigung bringt keine neue Information und fördert blindes Bestätigen ({{ ref("RI3") }}).

**Quelle:** Projektnotiz, Ablauf.
