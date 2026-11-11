---
render_macros: true
id: 'T6'
titel: 'Ablehnung und Zeitüberschreitung liefern keinen Wert und einen stabilen Code'
bereich: 'Freigabe'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [F4, F10]
---
# T6 — Ablehnung und Zeitüberschreitung liefern keinen Wert und einen stabilen Code

{{ design_kopf(page) }}

**Form:** Go-Test mit `--ui auto-deny` und mit einem Skript, das nie antwortet.

**Erwartung:** Der Agent erhält `denied` oder `timeout`. Die Antwort enthält keinen Wert. Es entsteht kein Grant. Das Fake-Backend zählt keinen Lesezugriff auf einen Wert.


Belegt: {{ ref("F4") }}, {{ ref("F10") }}.
