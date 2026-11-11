---
render_macros: true
id: 'T18'
titel: 'Ein Release hat je Archiv eine SBOM, eine vollständige Prüfsummendatei und signierte darwin-Binaries'
bereich: 'Betrieb und Release'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
umsetzung: teilweise
beweist: [F19, D18]
---
# T18 — Ein Release hat je Archiv eine SBOM, eine vollständige Prüfsummendatei und signierte darwin-Binaries

{{ design_kopf(page) }}

**Form:** mise-Tasks `release:sbom:check` und `release:signatures:check` in der CI und im Workflow für den signierten Snapshot.

**Erwartung:** Jedes Archiv hat eine CycloneDX-SBOM. Die Prüfsummendatei nennt jedes Archiv und jede SBOM. Jedes darwin-Binary trägt eine Developer-ID-Signatur mit Hardened Runtime.

**Umsetzung heute:** Die Prüfung der SBOMs läuft. Die Prüfung der Signatur braucht die Secrets im Repo.


Belegt: {{ ref("F19") }}, {{ ref("D18") }}.
