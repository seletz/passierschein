---
render_macros: true
id: 'UC5'
titel: 'Ein Agent fordert Zugangsdaten außerhalb von dev an'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: aktiv
stufe: 5
---
# UC5 — Ein Agent fordert Zugangsdaten außerhalb von dev an

{{ design_kopf(page) }}

**Akteure:** Agent, Daemon, Mensch.

**Auslöser:** Der Agent braucht Zugangsdaten aus einem Vault, der nicht zur Umgebung dev gehört.

```mermaid
sequenceDiagram
    autonumber
    participant A as Agent
    participant D as Daemon
    actor H as Mensch
    A->>D: request_credentials<br/>(Vault außerhalb von dev)
    alt MVP
        Note over D: Audit: Ablehnung
        D-->>A: Fehler not_allowed,<br/>ohne Popup
    else ab Stufe 5
        D->>H: Popup, Umgebung hervorgehoben
        H-->>D: Auswahl und Touch ID
        D->>D: Wert in Datei auf tmpfs,<br/>mit Laufzeit
        Note over D: Audit: Auslieferung
        D-->>A: Pfad der Datei, nie der Wert
    end
```

**Ablauf im MVP:** Der Daemon lehnt die Anfrage ohne Freigabeanfrage ab. Der Agent erhält den Code `not_allowed`. Der Daemon schreibt die Ablehnung in das Audit-Log ({{ ref("D24") }}).

**Ablauf ab Stufe 5:**

1. Der Daemon startet eine Freigabe. Die Freigabeanfrage hebt die Umgebung hervor.
2. Nach der Freigabe schreibt der Daemon den Wert in eine Datei auf einem tmpfs. Die Datei hat eine Laufzeit ({{ ref("D10") }}).
3. Der Agent erhält den Pfad der Datei, nie den Wert.

**Ergebnis:** Ein Wert außerhalb von dev steht nie in einer MCP-Antwort.

**Offen:** Der Ablauf mit der Datei ist nicht entworfen ({{ ref("G11") }}).
