---
render_macros: true
id: 'D15'
titel: 'Der Hook gegen den direkten Aufruf von op ist ein Hinweis und kein Schutz'
bereich: 'Oberflächen'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: gueltig
use_cases: [UC12]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Projektnotiz vom 2026-11-11
---
# D15 — Der Hook gegen den direkten Aufruf von op ist ein Hinweis und kein Schutz

{{ design_kopf(page) }}

**Entscheidung:** Ein Hook in den Agent-Werkzeugen erkennt den direkten Aufruf von `op` und verweist auf `a38`. Der Hook gilt nicht als Schutzmaßnahme.

**Begründung:** Ein Agent führt beliebige Befehle aus und umgeht einen Hook leicht. Den Schutz liefert erst die VM ohne 1Password.

**Quelle:** Projektnotiz, Stufenplan.
