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

**Form:** Go-Test mit `--ui script` und einer gefälschten Uhr. Das Skript zählt die Freigabeanfragen. Das Bündelfenster kommt aus der Konfiguration des Tests.

**Schritte:** Eine Sitzung sendet drei einzelne Anfragen im Abstand von 200 Millisekunden.

**Erwartung:** Es entsteht genau eine Freigabeanfrage mit drei Einträgen. Eine vierte Anfrage nach dem Fenster erzeugt eine zweite Freigabeanfrage.

**Weitere Fälle:** Eine Anfrage kurz vor dem Ende des Fensters gehört zum Bündel; eine Anfrage kurz danach öffnet ein neues Bündel. Eine späte Anfrage verlängert das Fenster nicht. Eine Anfrage, die ein Grant abdeckt, öffnet kein Fenster. Zwei Sitzungen im selben Fenster ergeben zwei Bündel.

**Scheitert, wenn:** der Daemon je Anfrage eine Freigabeanfrage startet.


Belegt: {{ ref("F2") }}, {{ ref("D5") }}.
