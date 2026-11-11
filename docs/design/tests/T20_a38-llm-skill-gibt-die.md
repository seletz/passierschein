---
render_macros: true
id: 'T20'
titel: 'a38 llm --skill gibt die eingebettete Anleitung aus und nennt jeden Fehlercode'
bereich: 'Oberflächen'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [F14, D22]
---
# T20 — a38 llm --skill gibt die eingebettete Anleitung aus und nennt jeden Fehlercode

{{ design_kopf(page) }}

**Form:** Go-Test.

**Erwartung:** Die Ausgabe ist gleich der eingebetteten Datei. Die Anleitung nennt jedes MCP-Werkzeug und jeden Fehlercode aus dem Code.

**Scheitert, wenn:** ein neuer Fehlercode in der Anleitung fehlt.


Belegt: {{ ref("F14") }}, {{ ref("D22") }}.
