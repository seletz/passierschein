---
render_macros: true
id: 'UC13'
titel: 'Ein Agent in der VM fordert Zugangsdaten über den weitergeleiteten Socket an'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
stufe: 5
---
# UC13 — Ein Agent in der VM fordert Zugangsdaten über den weitergeleiteten Socket an

{{ design_kopf(page) }}

**Akteure:** Agent in der VM, Daemon auf dem Mac, Mensch.

**Auslöser:** Der Agent läuft in der Linux-VM und braucht Zugangsdaten.

**Vorbedingung:** Der Mensch ist per SSH mit der VM verbunden. Die Verbindung leitet den Agent-Socket in die VM weiter ({{ ref("D3") }}).

**Ablauf:** wie {{ ref("UC1") }}. Die geprüften Fakten nennen die VM und die SSH-Sitzung statt des lokalen Prozesses.

**Ergebnis:** Die VM enthält kein Werkzeug von 1Password ({{ ref("D2") }}).

**Offen:** Wie der Daemon die VM und die Sitzung erkennt ({{ ref("G1") }}). Was geschieht, wenn der Mac nicht erreichbar ist ({{ ref("G9") }}).
