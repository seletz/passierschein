---
render_macros: true
id: 'T1'
titel: 'Der Ablauf liefert mit Fake-Backend und automatischer Freigabe den Wert und schreibt das Audit-Log'
bereich: 'Anfrage und Bündelung'
lauf: '2.0'
stand: '2026-11-12'
order: 1
headless: true
status: aktiv
umsetzung: vorhanden
beweist: [F1, F6, F8, F9]
---
# T1 — Der Ablauf liefert mit Fake-Backend und automatischer Freigabe den Wert und schreibt das Audit-Log

{{ design_kopf(page) }}

**Form:** Go-Test, der `a38 serve --backend fake --ui auto-approve` und `a38 mcp` als Prozesse startet.

**Schritte:** Der Test fordert zwei Referenzen aus einem dev-Vault der YAML-Datei an.

**Erwartung:** Die Antwort enthält beide Werte. Das Audit-Log enthält Anfrage, Bündel, Entscheidung, Grant und Auslieferung in dieser Reihenfolge.

**Scheitert, wenn:** ein Wert fehlt oder ein Ereignis im Audit-Log fehlt.


Belegt: {{ ref("F1") }}, {{ ref("F6") }}, {{ ref("F8") }}, {{ ref("F9") }}.
