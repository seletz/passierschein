---
render_macros: true
id: 'G39'
titel: 'Das Design nennt keine Antwort des Daemons auf exec_result und keine Meldung eines gescheiterten Starts'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-16'
order: 6
headless: true
type: Gap
kind: design
status: geschlossen
beleg:
  wer: Stefan
  datum: '2026-11-16'
  quelle: ['#95', '#115']
redaktion:
  - {datum: '2026-11-16', issue: '#99'}
register:
  owner: Stefan
---
# G39 — Das Design nennt keine Antwort des Daemons auf exec_result und keine Meldung eines gescheiterten Starts

{{ design_kopf(page) }}

{{ ref("D61") }} nennt drei neue Nachrichten am Agent-Socket: `request_exec`, `exec_credentials` und `exec_result`. Der Entwurf lässt zwei Fragen offen.

- **Antwort auf `exec_result`:** Das Protokoll kennt nur Anfrage und Antwort mit Korrelations-ID. Der Client von `a38 mcp` wartet auf jede Anfrage auf eine Antwort. Das Design nennt keine Antwort des Daemons auf `exec_result`.
- **Gescheiterter Start:** `a38 mcp` meldet dem Agent `exec_failed`, wenn es das Kommando nach der Auslieferung nicht startet ({{ ref("D61") }}). Das Design sagt nicht, ob `a38 mcp` diesen Fall dem Daemon meldet. Das Audit-Log zeigt dann eine Auslieferung ohne Ausführung.

Ein dritter Punkt folgt aus dem Protokoll: Ein Schlüssel einer Nachricht besteht nur aus Kleinbuchstaben ({{ ref("F1") }}). Die Abbildung `env` mit Namen wie `PGPASSWORD` steht deshalb auf dem Draht als Liste von Paaren aus Name und Referenz, nicht als JSON-Objekt. Das MCP-Werkzeug behält das JSON-Objekt aus {{ ref("D61") }}.

**Vorschlag (umgesetzt in Issue 95, zu bestätigen durch Stefan):**

- Der Daemon beantwortet `exec_result` mit der neuen Nachricht `exec_recorded`. Sie trägt nur die Korrelations-ID. Ein Fehler kommt als `error`.
- `exec_result` trägt ein eigenes Feld `request_id` mit der ID der Anfrage `request_exec`.
- `exec_result` hat ein optionales Feld `error` mit dem Code `exec_failed`. Ist es gesetzt, sind alle Felder des Ergebnisses leer. Der Daemon schreibt dafür das Ereignis „Ausführung“ mit dem Vermerk „nicht gestartet“ ({{ ref("D65") }}).

Offene Fragen:

- Bestätigt Stefan den Vorschlag, oder soll `exec_result` ohne Antwort bleiben?
- Soll ein Fehler des Audit-Logs bei `exec_result` `a38 mcp` erreichen, oder nur das Log des Daemons?

**Ergebnis (Stefan, 2026-11-16):** Stefan bestätigt den Vorschlag. `exec_result` erhält die Antwort `exec_recorded`; ein gescheiterter Start kommt als Feld `error` mit dem Code `exec_failed`.

**Quelle:** [Issue 95](https://github.com/seletz/passierschein/issues/95), Umsetzung des Protokolls für den exec-Weg.
