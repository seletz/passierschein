---
render_macros: true
id: 'F2'
titel: 'Bündelung der Anfragen einer Sitzung'
bereich: 'Anfrage und Bündelung'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
stufe: 1
umsetzung: zu-bauen
implementiert: [D5, D29]
use_cases: [UC1]
---
# F2 — Bündelung der Anfragen einer Sitzung

{{ design_kopf(page) }}

Der Daemon sammelt die Anfragen einer Sitzung im Bündelfenster und begrenzt die Größe des Bündels. Aus einem Bündel entsteht genau eine Freigabeanfrage.


Setzt um: {{ ref("D5") }}, {{ ref("D29") }}.
