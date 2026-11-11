---
render_macros: true
id: 'T17'
titel: 'Ein gesperrtes oder nicht erreichbares Backend führt zu einem Fehlercode ohne Ersatzweg'
bereich: 'Anfrage und Bündelung'
lauf: '2.0'
stand: '2026-11-12'
order: 3
headless: true
status: aktiv
umsetzung: vorhanden
beweist: [F10, D35]
---
# T17 — Ein gesperrtes oder nicht erreichbares Backend führt zu einem Fehlercode ohne Ersatzweg

{{ design_kopf(page) }}

**Form:** Go-Test mit den Fehlerzuständen des Fake-Backends.

**Erwartung:** Der Agent erhält `backend_locked` oder `backend_unavailable`. Ohne laufenden Daemon meldet `a38 mcp` `daemon_unavailable`. Kein Fall liefert einen Wert. Das Audit-Log enthält je Fall mit laufendem Daemon einen Eintrag der Ereignisart Fehler mit dem Code.


Belegt: {{ ref("F10") }}, {{ ref("D35") }}.
