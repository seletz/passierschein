---
render_macros: true
id: 'T10'
titel: 'Der Daemon lehnt ungültige Nachrichten ab und bleibt stabil'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [D26, F1]
---
# T10 — Der Daemon lehnt ungültige Nachrichten ab und bleibt stabil

{{ design_kopf(page) }}

**Form:** Tabellengetriebener Go-Test und ein Fuzz-Test.

**Fälle:** unbekannter Typ; unbekanntes Feld; falsche Version; kein JSON; Nachricht über der Größengrenze; abgebrochene Verbindung mitten in der Nachricht.

**Erwartung:** Jeder Fall endet mit `invalid_request` oder dem Ende der Verbindung. Der Daemon bedient danach andere Sitzungen weiter. Der Fuzz-Test findet keinen Absturz.


Belegt: {{ ref("D26") }}, {{ ref("F1") }}.
