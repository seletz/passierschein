---
render_macros: true
id: 'UC3'
titel: 'Ein Agent fordert Zugangsdaten außerhalb seines Grants an'
bereich: 'Freigabe'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
stufe: 1
---
# UC3 — Ein Agent fordert Zugangsdaten außerhalb seines Grants an

{{ design_kopf(page) }}

**Akteure:** Agent, Daemon, Mensch.

**Auslöser:** Der Agent braucht Zugangsdaten, die sein Grant nicht abdeckt.

**Vorbedingung:** Die Sitzung hat einen gültigen Grant.

**Ablauf:**

1. Der Agent fragt an wie in {{ ref("UC1") }}.
2. Der Daemon stellt fest, dass der Grant die Anfrage nicht abdeckt.
3. Der Daemon startet eine Freigabe. Die Freigabeanfrage ist als Abweichung markiert. Sie nennt den bestehenden Grant und zeigt nur die neuen Zugangsdaten ({{ ref("D8") }}).
4. Der Mensch entscheidet wie in {{ ref("UC1") }}.
5. Bei einer Freigabe entsteht ein weiterer Grant für die Sitzung. Der bestehende Grant bleibt unverändert.

```mermaid
sequenceDiagram
    participant A as Agent
    participant M as a38 mcp
    participant D as Daemon
    participant U as Freigabe-App
    actor H as Mensch
    A->>M: request_credentials
    M->>D: Anfrage über den Agent-Socket
    D->>D: Grant prüfen: deckt die Anfrage nicht ab
    D->>U: Freigabeanfrage, als Abweichung markiert
    U->>H: Popup: bestehender Grant, nur die neuen Zugangsdaten
    alt Freigabe
        H->>U: Auswahl und Touch ID
        U-->>D: Entscheidung: freigegeben
        D->>D: weiteren Grant vergeben
        D-->>M: Werte
        M-->>A: MCP-Antwort mit den Werten
    else Ablehnung oder Zeitablauf
        U-->>D: Entscheidung: abgelehnt
        D-->>M: Fehler denied oder timeout
        M-->>A: Fehler
    end
```

**Ergebnis:** Eine Abweichung fällt dem Menschen auf. Sie sieht anders aus als eine erste Anfrage.
