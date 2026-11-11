---
render_macros: true
id: 'T8'
titel: 'Ein Secret-Wert erscheint nie in Log, Audit-Log, Fehlermeldung oder Freigabeanfrage'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-12'
order: 3
headless: true
status: aktiv
umsetzung: vorhanden
beweist: [D34, D6]
---
# T8 — Ein Secret-Wert erscheint nie in Log, Audit-Log, Fehlermeldung oder Freigabeanfrage

{{ design_kopf(page) }}

**Form:** Go-Test mit einem Kennwert im Fake-Backend.

**Schritte:** Der Test durchläuft Freigabe, Ablehnung, Zeitüberschreitung und Backend-Fehler. Er sammelt die Ausgabe des Daemons, die Audit-Datei, jede Fehlermeldung und jede Freigabeanfrage.

**Erwartung:** Der Kennwert steht nur in der MCP-Antwort des freigegebenen Falls.

**Kein Cache:** Der Test ändert den Wert im Fake-Backend zwischen zwei Auslieferungen unter einem Grant. Die zweite Antwort enthält den neuen Wert. Der Zähler der Lesezugriffe steigt um eins.

**Scheitert, wenn:** der Kennwert an einer anderen Stelle steht.


Belegt: {{ ref("D34") }}, {{ ref("D6") }}.
