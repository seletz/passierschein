---
render_macros: true
id: 'UC11'
titel: 'Der Mensch richtet den Daemon ein und legt die Policy fest'
bereich: 'Betrieb und Release'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
stufe: 3
---
# UC11 — Der Mensch richtet den Daemon ein und legt die Policy fest

{{ design_kopf(page) }}

**Akteure:** Mensch.

**Auslöser:** Der Mensch nimmt `a38` auf seinem Mac in Betrieb.

**Ablauf:**

1. Der Mensch installiert `a38` und die Freigabe-App mit `mise`.
2. Der Mensch legt die Konfiguration an: Projekte, erlaubte Vaults, Laufzeiten ({{ ref("D28") }}, {{ ref("G17") }}).
3. Der Mensch richtet den Zugang zu 1Password ein ({{ ref("D39") }}).
4. Der Mensch installiert den LaunchAgent mit `a38 service install`.
5. Der Mensch trägt `a38 mcp` als MCP-Server in seine Agent-Werkzeuge ein.

**Ergebnis:** Der Daemon startet mit der Anmeldung. Die Agents erreichen ihn über den Agent-Socket.
