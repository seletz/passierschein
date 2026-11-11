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
2. Der Mensch legt die Konfiguration an: die Liste der Vaults mit ihrer Umgebung ({{ ref("D12") }}, {{ ref("D47") }}), je Projekt die erlaubten Vaults ({{ ref("D28") }}), die Laufzeit eines Grants ({{ ref("D32") }}), das Bündelfenster und die Obergrenze eines Bündels ({{ ref("D29") }}), die Form des Sitzungsnamens ({{ ref("D25") }}). Format und Ort: {{ ref("G17") }}.
3. Der Mensch richtet den Zugang zu 1Password ein ({{ ref("D39") }}).
4. Der Mensch installiert den LaunchAgent mit `a38 service install`.
5. Der Mensch trägt `a38 mcp` als MCP-Server in seine Agent-Werkzeuge ein. Die Anmeldung legt je Sitzung den Namen der Sitzung fest ({{ ref("D25") }}).
6. Der Mensch prüft die Einrichtung mit `a38 doctor`.

**Ergebnis:** Der Daemon startet mit der Anmeldung. Die Agents erreichen ihn über den Agent-Socket.
