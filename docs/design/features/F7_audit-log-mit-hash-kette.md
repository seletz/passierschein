---
render_macros: true
id: 'F7'
titel: 'Audit-Log mit Hash-Kette, Liste und Prüfung'
bereich: 'Audit'
lauf: '2.0'
stand: '2026-11-12'
order: 1
headless: true
status: aktiv
stufe: 1
umsetzung: vorhanden
implementiert: [D11, D36, D37]
use_cases: [UC8, UC6]
---
# F7 — Audit-Log mit Hash-Kette, Liste und Prüfung

{{ design_kopf(page) }}

Der Daemon schreibt jedes Ereignis in die SQLite-Datei, sobald es eintritt, auch jeden Fehlschlag. Bei einem Fehler beim Schreiben liefert der Daemon nichts aus und meldet den Fehler dem Menschen. `a38 audit list` zeigt die Einträge. `a38 audit verify` prüft die Kette.


Setzt um: {{ ref("D11") }}, {{ ref("D36") }}, {{ ref("D37") }}.
