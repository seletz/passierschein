---
render_macros: true
id: 'F15'
titel: 'Installation als LaunchAgent und Konfiguration'
bereich: 'Betrieb und Release'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
stufe: 3
umsetzung: zu-bauen
implementiert: [D28, D42]
use_cases: [UC11]
---
# F15 — Installation als LaunchAgent und Konfiguration

{{ design_kopf(page) }}

`a38 service install` richtet den LaunchAgent ein. Der Daemon liest die Konfiguration beim Start und prüft sie. Ein Fehler in der Konfiguration bricht den Start ab.


Setzt um: {{ ref("D28") }}, {{ ref("D42") }}.
