---
render_macros: true
id: 'T11'
titel: 'Die Kennung der Sitzung und die geprüften Fakten kommen vom Daemon'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [D4, D25]
---
# T11 — Die Kennung der Sitzung und die geprüften Fakten kommen vom Daemon

{{ design_kopf(page) }}

**Form:** Go-Test mit `--ui script`.

**Schritte:** Der Agent sendet in seinen Angaben einen falschen Prozessnamen, ein falsches Projekt und eine fremde Sitzungskennung.

**Erwartung:** Die geprüften Fakten der Freigabeanfrage nennen den wirklichen Prozess und das wirkliche Arbeitsverzeichnis. Die Angaben des Agents stehen nur im Abschnitt der Angaben. Ein Grant einer anderen Sitzung gilt nicht.


Belegt: {{ ref("D4") }}, {{ ref("D25") }}.
