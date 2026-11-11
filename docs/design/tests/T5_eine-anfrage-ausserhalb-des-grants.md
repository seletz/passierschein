---
render_macros: true
id: 'T5'
titel: 'Eine Anfrage außerhalb des Grants ist in der Freigabeanfrage als Abweichung markiert'
bereich: 'Freigabe'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [F5, D8]
---
# T5 — Eine Anfrage außerhalb des Grants ist in der Freigabeanfrage als Abweichung markiert

{{ design_kopf(page) }}

**Form:** Go-Test mit `--ui script`. Das Skript speichert die Freigabeanfrage.

**Schritte:** Freigabe für ein Zugangsdatum. Anfrage für ein anderes Zugangsdatum in derselben Sitzung.

**Erwartung:** Die zweite Freigabeanfrage nennt den bestehenden Grant und enthält nur den neuen Eintrag.


Belegt: {{ ref("F5") }}, {{ ref("D8") }}.
