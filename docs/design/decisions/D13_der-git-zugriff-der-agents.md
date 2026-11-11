---
render_macros: true
id: 'D13'
titel: 'Der Git-Zugriff der Agents läuft über eine eigene Identität und nicht über a38'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 4
headless: true
status: gueltig
use_cases: [UC13]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Projektnotiz vom 2026-11-11
---
# D13 — Der Git-Zugriff der Agents läuft über eine eigene Identität und nicht über a38

{{ design_kopf(page) }}

**Entscheidung:** Die Agents nutzen für Git ein eigenes GitHub-Konto mit einem eigenen SSH-Schlüssel in der VM. `a38` vermittelt keine Zugangsdaten für Git.

**Begründung:** Die Absicherung liegt bei GitHub: nur nötige Repos, kein Admin, Branch-Schutz mit Review durch einen Menschen.

**Quelle:** Projektnotiz, Ziele.
