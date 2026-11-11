---
render_macros: true
id: 'UC7'
titel: 'Der Mensch widerruft einen Grant'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 3
headless: true
status: aktiv
stufe: 1
---
# UC7 — Der Mensch widerruft einen Grant

{{ design_kopf(page) }}

**Akteure:** Mensch, Daemon.

**Auslöser:** Der Mensch will einen Grant vor dem Ende der Laufzeit beenden.

**Ablauf:**

1. Der Mensch ruft `a38 grants list` auf.
2. Der Mensch ruft `a38 grants revoke <id>` auf.
3. Der Befehl spricht mit dem Daemon über den Admin-Socket ({{ ref("D44") }}).
4. Der Daemon beendet den Grant und schreibt den Widerruf in das Audit-Log.

```mermaid
sequenceDiagram
    actor H as Mensch
    participant C as a38 grants
    participant D as Daemon
    participant A as Agent
    H->>C: a38 grants list
    C->>D: Liste über den Admin-Socket
    D-->>C: Grants
    H->>C: a38 grants revoke ID
    C->>D: Widerruf über den Admin-Socket
    D->>D: Grant beenden, Audit-Log schreiben
    D-->>C: bestätigt
    A->>D: nächste Anfrage der Sitzung
    Note over D: braucht eine neue Freigabe
```

**Ergebnis:** Die nächste Anfrage der Sitzung braucht eine neue Freigabe. Ein schon ausgelieferter Wert bleibt beim Agent ({{ ref("RI11") }}).
