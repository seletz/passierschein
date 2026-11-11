---
render_macros: true
id: 'UC2'
titel: 'Ein Agent fordert Zugangsdaten im Rahmen eines Grants an und erhält sie ohne Popup'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
stufe: 1
---
# UC2 — Ein Agent fordert Zugangsdaten im Rahmen eines Grants an und erhält sie ohne Popup

{{ design_kopf(page) }}

**Akteure:** Agent, Daemon.

**Auslöser:** Der Agent braucht Zugangsdaten erneut oder braucht weitere Zugangsdaten.

**Vorbedingung:** Die Sitzung hat einen gültigen Grant. Der Grant deckt die Anfrage ab.

**Ablauf:**

1. Der Agent fragt an wie in {{ ref("UC1") }}.
2. Der Daemon prüft die Policy und den Grant ({{ ref("D8") }}, {{ ref("D32") }}).
3. Der Daemon liefert die Werte ohne Freigabeanfrage aus.
4. Der Daemon schreibt die Auslieferung mit dem Verweis auf den Grant in das Audit-Log.

```mermaid
sequenceDiagram
    participant A as Agent
    participant M as a38 mcp
    participant D as Daemon
    participant B as Backend
    A->>M: request_credentials
    M->>D: Anfrage über den Agent-Socket
    D->>D: Policy prüfen
    D->>D: Grant prüfen: deckt die Anfrage ab
    D->>B: Werte lesen
    B-->>D: Werte
    D-->>M: Werte
    M-->>A: MCP-Antwort mit den Werten
    Note over D: Audit-Log: Auslieferung mit Verweis auf den Grant
    Note over A,D: Kein Popup, der Mensch ist nicht beteiligt
```

**Ergebnis:** Der Mensch sieht kein Popup. Die Auslieferung ist im Audit-Log nachvollziehbar.
