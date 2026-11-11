---
render_macros: true
id: 'UC12'
titel: 'Ein Agent ruft op direkt auf und erhält einen Hinweis'
bereich: 'Oberflächen'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: aktiv
stufe: 4
---
# UC12 — Ein Agent ruft op direkt auf und erhält einen Hinweis

{{ design_kopf(page) }}

**Akteure:** Agent.

**Auslöser:** Ein Agent will `op` direkt aufrufen.

**Ablauf:**

1. Ein Hook im Agent-Werkzeug erkennt den Aufruf.
2. Der Hook blockiert den Aufruf und nennt `a38` als Weg.

**Ergebnis:** Der Agent nutzt `a38`. Der Hook ist ein Hinweis und kein Schutz ({{ ref("D15") }}).
