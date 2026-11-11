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

```mermaid
sequenceDiagram
    autonumber
    actor H as Mensch
    participant D as Daemon
    participant A as Agent
    H->>D: a38 grants list<br/>(Admin-Socket)
    D-->>H: Grants
    H->>D: a38 grants revoke ID
    D->>D: Grant beenden,<br/>Audit-Log schreiben
    D-->>H: bestätigt
    A->>D: nächste Anfrage
    Note over D: braucht eine neue Freigabe
```

**Ablauf:**

1. Der Mensch ruft `a38 grants list` auf.
2. Der Mensch ruft `a38 grants revoke <id>` auf.
3. Der Befehl spricht mit dem Daemon über den Admin-Socket ({{ ref("D44") }}).
4. Der Daemon beendet den Grant und schreibt den Widerruf in das Audit-Log.

**Ergebnis:** Die nächste Anfrage der Sitzung braucht eine neue Freigabe. Ein schon ausgelieferter Wert bleibt beim Agent ({{ ref("RI11") }}).
