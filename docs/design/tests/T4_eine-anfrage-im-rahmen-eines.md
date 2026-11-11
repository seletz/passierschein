---
render_macros: true
id: 'T4'
titel: 'Eine Anfrage im Rahmen eines Grants braucht keine Freigabe, nach der Laufzeit wieder'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [F5, D8, D32]
---
# T4 — Eine Anfrage im Rahmen eines Grants braucht keine Freigabe, nach der Laufzeit wieder

{{ design_kopf(page) }}

**Form:** Go-Test mit einer gefälschten Uhr.

**Schritte:** Freigabe für ein Zugangsdatum. Zweite Anfrage für dasselbe Zugangsdatum. Die Uhr springt hinter die Laufzeit. Dritte Anfrage.

**Erwartung:** Die zweite Anfrage erzeugt keine Freigabeanfrage. Die dritte erzeugt eine. Das Audit-Log enthält den Ablauf des Grants.

**Fälle zur Laufzeit:** Ohne Wert in der Konfiguration gilt der Grant kurz vor vier Stunden und endet nach vier Stunden. Ein Wert von 30 Minuten gilt. Ein Wert von zwölf Stunden gilt; es gibt keine feste Obergrenze. Das Skript wählt 15 Minuten bei einem Wert von vier Stunden: Der Grant endet nach 15 Minuten, und der Eintrag im Audit-Log nennt 15 Minuten. Das Skript liefert 0, einen negativen Wert oder Text: kein Grant, Code `ui_failed`. Ein Wert der Konfiguration von 0, ein negativer Wert oder Text: Der Daemon startet nicht.

**Offen:** Das Ergebnis, wenn die Freigabe eine längere Laufzeit als die Konfiguration liefert, ist nicht entschieden. Der Vorschlag ist: kein Grant.


Belegt: {{ ref("F5") }}, {{ ref("D8") }}, {{ ref("D32") }}.
