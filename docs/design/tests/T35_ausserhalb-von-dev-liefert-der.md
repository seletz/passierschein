---
render_macros: true
id: 'T35'
titel: 'Außerhalb von dev liefert der Daemon nur über den exec-Weg und nie als MCP-Antwort'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-15'
order: 11
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [F22, D62, UC15]
---
# T35 — Außerhalb von dev liefert der Daemon nur über den exec-Weg und nie als MCP-Antwort

{{ design_kopf(page) }}

**Form:** Go-Test mit Fake-Backend und Fake-UI `script`.

**Vorbedingung:** Vaults `beispiel.app.k8s-dev` (dev), `beispiel.app.k8s-test` (test), `beispiel.app.k8s-prod` (prod). Konfiguration im Standard.

**Schritte:**

1. `request_credentials` für den test-Vault.
2. `request_credentials` für dev und test in einer Anfrage.
3. `run_with_credentials` für den prod-Vault.
4. Dasselbe mit `environments.prod.delivery: [exec]`.

**Erwartung:**

1. `exec_required`; die Fake-UI erhält keine Freigabeanfrage.
2. `exec_required` für die ganze Anfrage.
3. `not_allowed` ohne Freigabeanfrage.
4. Freigabeanfrage und Start des Kommandos.

**Scheitert, wenn:** ein Wert aus test oder prod in einer MCP-Antwort steht oder prod ohne Einschalten ausliefert.
