---
render_macros: true
id: 'F24'
titel: 'Hook op-guard mit --strict und Skill für den exec-Weg'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-15'
order: 5
headless: true
status: aktiv
stufe: 4
umsetzung: zu-bauen
implementiert: [D67, D68]
use_cases: [UC16]
---
# F24 — Hook op-guard mit --strict und Skill für den exec-Weg

{{ design_kopf(page) }}

`a38 hook op-guard` nennt in der Meldung `run_with_credentials` und `request_credentials` und kennt den Schalter `--strict`. Der Skill aus `a38 llm --skill` erhält die Teile für den exec-Weg. Die Anleitung zum Hook empfiehlt die Einrichtung im Benutzerbereich.

Setzt auf: Hook ({{ ref("F16") }}), Skill ({{ ref("F14") }}).

Setzt um: {{ ref("D67") }}, {{ ref("D68") }}.
