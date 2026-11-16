---
render_macros: true
id: 'T39'
titel: 'Der Daemon liefert nicht aus, wenn das Kommando einen der Werte enthält'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-15'
order: 13
headless: true
status: aktiv
umsetzung: vorhanden
beweist: [F22, D65]
---
# T39 — Der Daemon liefert nicht aus, wenn das Kommando einen der Werte enthält

{{ design_kopf(page) }}

**Form:** Go-Test mit Fake-Backend und Fake-UI `auto-approve`.

**Schritte:**

1. `run_with_credentials`, bei dem ein Argument den Wert des Fake-Backends enthält.
2. Dasselbe mit dem Wert im Arbeitsverzeichnis.

**Erwartung:** `invalid_request`. Das Kommando startet nicht. Das Audit-Log hat eine Ablehnung mit dem Grund und ohne den Wert.

**Scheitert, wenn:** das Kommando startet oder der Grund den Wert nennt.
