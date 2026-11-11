---
render_macros: true
id: 'UC13'
titel: 'Ein Agent in der VM fordert Zugangsdaten an'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
stufe: 5
---
# UC13 — Ein Agent in der VM fordert Zugangsdaten an

{{ design_kopf(page) }}

**Akteure:** Agent in der VM, Daemon auf dem Mac, Mensch.

**Auslöser:** Der Agent läuft in der Linux-VM und braucht Zugangsdaten.

**Vorbedingung:** Die VM erreicht den Agent-Endpunkt des Daemons. Der Transport ist offen: SSH-`RemoteForward`, ein eigener Port-Forward, Tailscale oder die Host-Anbindung der VM ({{ ref("D3") }}, {{ ref("G1") }}).

**Ablauf:** wie {{ ref("UC1") }}. Die geprüften Fakten nennen die VM statt des lokalen Prozesses. Welche Kennung der VM der Daemon prüfen kann, hängt vom Transport ab ({{ ref("G1") }}).

**Ergebnis:** Die VM enthält kein Werkzeug von 1Password ({{ ref("D2") }}).

**Offen:** Wie der Daemon die VM und die Sitzung erkennt ({{ ref("G1") }}). Was geschieht, wenn der Mac nicht erreichbar ist ({{ ref("G9") }}).
