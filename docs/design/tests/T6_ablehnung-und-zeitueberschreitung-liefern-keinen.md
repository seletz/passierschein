---
render_macros: true
id: 'T6'
titel: 'Ablehnung und Zeitüberschreitung liefern keinen Wert und einen stabilen Code'
bereich: 'Freigabe'
lauf: '2.0'
stand: '2026-11-12'
order: 2
headless: true
status: aktiv
umsetzung: vorhanden
beweist: [F4, F10]
---
# T6 — Ablehnung und Zeitüberschreitung liefern keinen Wert und einen stabilen Code

{{ design_kopf(page) }}

**Form:** Go-Test mit `--ui auto-deny`, mit einem Skript, das nie antwortet, und mit einem Skript, das scheitert.

**Fälle:** Ablehnung; keine Antwort; das Skript endet mit einem Fehler; das Skript schreibt kein JSON; das Skript schreibt `approved` mit einem Exit-Code ungleich 0.

**Erwartung:** Der Agent erhält `denied`, `timeout` oder `ui_failed`. Die Antwort enthält keinen Wert. Es entsteht kein Grant. Das Fake-Backend zählt keinen Lesezugriff auf einen Wert. Das Audit-Log enthält je Fall einen Eintrag mit der Ereignisart und dem Code.


Belegt: {{ ref("F4") }}, {{ ref("F10") }}.
