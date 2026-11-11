---
render_macros: true
id: 'T23'
titel: 'Ein Grant umfasst genau die angehakten Zugangsdaten'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 5
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [D32, D7, F5]
---
# T23 — Ein Grant umfasst genau die angehakten Zugangsdaten

{{ design_kopf(page) }}

**Form:** Go-Test mit `--ui script`. Das Skript hakt eine Auswahl an.

**Fälle und Erwartung:**

- Bündel mit drei Einträgen, das Skript hakt zwei an: Die Antwort enthält zwei Werte. Der dritte Eintrag endet mit `denied`.
- Eine spätere Anfrage für den dritten Eintrag erzeugt eine Freigabeanfrage, die als Abweichung markiert ist und nur diesen Eintrag enthält.
- Freigabe ohne Häkchen: `denied`, kein Grant.
- Das Skript hakt einen Eintrag an, der nicht in der Anfrage steht: kein Grant, Code `ui_failed`.


Belegt: {{ ref("D32") }}, {{ ref("D7") }}, {{ ref("F5") }}.
