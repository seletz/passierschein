---
render_macros: true
id: 'D1'
titel: 'Nur der Daemon hat Zugriff auf 1Password, über das Go-SDK und nie über op'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: gueltig
use_cases: [UC1, UC11]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Projektnotiz vom 2026-11-11
---
# D1 — Nur der Daemon hat Zugriff auf 1Password, über das Go-SDK und nie über op

{{ design_kopf(page) }}

**Entscheidung:** `a38 serve` ist der einzige Bestandteil mit Zugriff auf 1Password. Der Daemon nutzt das Go-SDK von 1Password. Der Daemon ruft das Kommandozeilenwerkzeug `op` nie auf.

**Begründung:** Ein einziger Ort für den Zugriff ist prüfbar. Ein Aufruf von `op` erzeugt einen Dialog ohne Kontext und eine Autorisierung, die andere Prozesse mitnutzen können.

**Quelle:** Projektnotiz, Design v0.
