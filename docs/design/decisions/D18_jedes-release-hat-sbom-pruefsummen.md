---
render_macros: true
id: 'D18'
titel: 'Jedes Release hat SBOM, Prüfsummen, Signatur und Build-Provenienz'
bereich: 'Betrieb und Release'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: gueltig
use_cases: [UC11]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Projektnotiz vom 2026-11-11
---
# D18 — Jedes Release hat SBOM, Prüfsummen, Signatur und Build-Provenienz

{{ design_kopf(page) }}

**Entscheidung:** Ein Release entsteht als GitHub-Release. Jedes Release hat eine SBOM je Archiv, Prüfsummen, signierte und notarisierte macOS-Artefakte und eine Build-Provenienz. Die Verteilung läuft zunächst über `mise` mit dem Backend `github:`. Alle Aufgaben sind mise-Tasks. Die Workflows rufen nur mise-Tasks auf.

**Quelle:** Projektnotiz, Entwicklungsregeln. Offene Punkte: {{ ref("G15") }}.
