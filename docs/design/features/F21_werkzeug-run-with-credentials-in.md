---
render_macros: true
id: 'F21'
titel: 'Werkzeug run_with_credentials in a38 mcp mit Maskierung und Grenzen'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-15'
order: 4
headless: true
status: aktiv
stufe: 4
umsetzung: zu-bauen
implementiert: [D61, D64]
use_cases: [UC15]
---
# F21 — Werkzeug run_with_credentials in a38 mcp mit Maskierung und Grenzen

{{ design_kopf(page) }}

`a38 mcp` bietet das Werkzeug `run_with_credentials`. Es sucht das Programm, sendet `request_exec`, startet das Kommando mit den Werten in der Umgebung, maskiert und kürzt die Ausgabe, beendet das Kommando nach der Zeitgrenze und meldet `exec_result` an den Daemon. Das Werkzeug gibt dem Agent nie einen Wert.

Setzt auf: dem MCP-Server ({{ ref("F9") }}) und dem Protokoll des Agent-Sockets ({{ ref("F1") }}). Neu im Protokoll: `request_exec`, `exec_credentials`, `exec_result` und die Codes `exec_required` und `exec_failed`.

Setzt um: {{ ref("D61") }}, {{ ref("D64") }}.
