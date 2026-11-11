---
render_macros: true
id: 'UC14'
titel: 'Ein Agent fragt an und der Daemon oder das Backend ist nicht bereit'
bereich: 'Anfrage und Bündelung'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: aktiv
stufe: 1
---
# UC14 — Ein Agent fragt an und der Daemon oder das Backend ist nicht bereit

{{ design_kopf(page) }}

**Akteure:** Agent, Daemon.

**Auslöser:** Der Agent fragt an. Der Daemon läuft nicht, das Backend ist gesperrt, oder die Freigabe-App startet nicht.

**Ablauf:**

1. `a38 mcp` oder der Daemon erkennt den Fehler.
2. Der Agent erhält einen Fehler mit einem stabilen Code: `daemon_unavailable`, `backend_locked`, `backend_unavailable` oder `ui_failed` ({{ ref("D35") }}).
3. Der Daemon schreibt den Fehler in das Audit-Log, wenn er läuft.

**Ergebnis:** Der Daemon liefert nichts aus. Es gibt keinen Ersatzweg. Die Fehlermeldung enthält keinen Secret-Wert.
