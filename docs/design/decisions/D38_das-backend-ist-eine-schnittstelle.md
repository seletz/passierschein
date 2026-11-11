---
render_macros: true
id: 'D38'
titel: 'Das Backend ist eine Schnittstelle mit einem Fake-Backend und einem 1Password-Backend'
bereich: 'Backends'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: gueltig
use_cases: [UC10]
beleg:
  notiz: Vorschlag des Designlaufs, zu bestätigen durch Stefan
  annahme: true
---
# D38 — Das Backend ist eine Schnittstelle mit einem Fake-Backend und einem 1Password-Backend

{{ design_kopf(page) }}

**Entscheidung:** Der Daemon spricht mit dem Vault nur über eine Go-Schnittstelle: Vaults auflisten, Namen eines Items auflösen, Wert eines Feldes lesen. `--backend fake` liest einen Vault aus einer YAML-Datei. `--backend 1password` nutzt das SDK.

**Begründung:** Das Fake-Backend macht den ganzen Ablauf in der CI prüfbar.

**Quelle:** Projektnotiz, Stufe 1.
