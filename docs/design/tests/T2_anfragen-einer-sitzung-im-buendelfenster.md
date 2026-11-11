---
render_macros: true
id: 'T2'
titel: 'Anfragen einer Sitzung im Bündelfenster ergeben genau eine Freigabeanfrage'
bereich: 'Anfrage und Bündelung'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [F2, D5]
---
# T2 — Anfragen einer Sitzung im Bündelfenster ergeben genau eine Freigabeanfrage

{{ design_kopf(page) }}

**Form:** Go-Test mit `--ui script`. Das Skript zählt die Freigabeanfragen.

**Schritte:** Eine Sitzung sendet drei einzelne Anfragen im Abstand von 200 Millisekunden.

**Erwartung:** Es entsteht genau eine Freigabeanfrage mit drei Einträgen. Eine vierte Anfrage nach dem Fenster erzeugt eine zweite Freigabeanfrage.

**Scheitert, wenn:** der Daemon je Anfrage eine Freigabeanfrage startet.


Belegt: {{ ref("F2") }}, {{ ref("D5") }}.
