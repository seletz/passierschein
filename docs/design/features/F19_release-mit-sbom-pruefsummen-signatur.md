---
render_macros: true
id: 'F19'
titel: 'Release mit SBOM, Prüfsummen, Signatur und Doku'
bereich: 'Betrieb und Release'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: aktiv
stufe: 1
umsetzung: teilweise
implementiert: [D18, D46]
use_cases: [UC11]
---
# F19 — Release mit SBOM, Prüfsummen, Signatur und Doku

{{ design_kopf(page) }}

Der Release-Workflow baut die Archive, die SBOMs und die Prüfsummen, signiert und notarisiert die darwin-Binaries und veröffentlicht die Doku. Das Grundgerüst enthält diesen Teil schon. Die Signatur der Freigabe-App fehlt noch.


Setzt um: {{ ref("D18") }}, {{ ref("D46") }}.
