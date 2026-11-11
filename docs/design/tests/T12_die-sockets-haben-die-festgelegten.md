---
render_macros: true
id: 'T12'
titel: 'Die Sockets haben die festgelegten Rechte, und der Agent-Socket nimmt keine Admin-Befehle an'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 3
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [D43, D44]
---
# T12 — Die Sockets haben die festgelegten Rechte, und der Agent-Socket nimmt keine Admin-Befehle an

{{ design_kopf(page) }}

**Form:** Go-Test.

**Erwartung:** Das Verzeichnis hat `0700`, jeder Socket `0600`. Der Daemon bricht den Start ab, wenn das Verzeichnis weitere Rechte hat. Ein Widerruf und die Liste aller Grants über den Agent-Socket enden mit `invalid_request`.


Belegt: {{ ref("D43") }}, {{ ref("D44") }}.
