---
render_macros: true
id: 'F4'
titel: 'Freigabe-Schnittstelle mit gefälschten Freigaben für Tests'
bereich: 'Freigabe'
lauf: '2.0'
stand: '2026-11-12'
order: 1
headless: true
status: aktiv
stufe: 1
umsetzung: vorhanden
implementiert: [D30, D35, D40]
use_cases: [UC4, UC10]
---
# F4 — Freigabe-Schnittstelle mit gefälschten Freigaben für Tests

{{ design_kopf(page) }}

Der Daemon spricht mit der Freigabe über eine Schnittstelle: Freigabeanfrage hinein, Entscheidung heraus. Die Varianten `auto-approve`, `auto-deny` und `script` ersetzen die App in Tests. Die Schnittstelle hält die Warteschlange und die Wartezeit.

**Abgrenzung:** Die echte App ist {{ ref("F11") }}.


Setzt um: {{ ref("D30") }}, {{ ref("D35") }}, {{ ref("D40") }}.
