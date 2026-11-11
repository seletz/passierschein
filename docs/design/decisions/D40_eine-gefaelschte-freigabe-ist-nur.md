---
render_macros: true
id: 'D40'
titel: 'Eine gefälschte Freigabe ist nur mit dem Fake-Backend erlaubt'
bereich: 'Freigabe'
lauf: '1.0'
stand: '2026-11-11'
order: 6
headless: true
status: gueltig
use_cases: [UC10]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Vorschlag des Designlaufs, von Stefan im Review am 2026-11-11 abgenommen
---
# D40 — Eine gefälschte Freigabe ist nur mit dem Fake-Backend erlaubt

{{ design_kopf(page) }}

**Entscheidung:** `--ui` kennt die Werte `app`, `auto-approve`, `auto-deny` und `script`. Der Daemon startet mit einem echten Backend nur mit `--ui app`. Jede andere Kombination bricht den Start mit einem Fehler ab.

**Begründung:** `auto-approve` mit dem echten Backend gäbe jedem Prozess jedes dev-Secret ohne Freigabe.
