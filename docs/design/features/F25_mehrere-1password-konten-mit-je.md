---
render_macros: true
id: 'F25'
titel: 'Mehrere 1Password-Konten mit je einem Token im Daemon, in den Befehlen und in a38 doctor'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-16'
order: 3
headless: true
status: aktiv
stufe: 3
umsetzung: vorhanden
implementiert: [D70, D71, D72, D73, D75, D78, D79]
use_cases: [UC11, UC1]
---
# F25 — Mehrere 1Password-Konten mit je einem Token im Daemon, in den Befehlen und in a38 doctor

{{ design_kopf(page) }}

Der Daemon liefert aus Vaults mehrerer 1Password-Konten. Die Konfiguration nennt die Konten mit Name und Token-Quelle; jeder Vault nennt sein Konto. `a38 config onepassword-token` legt je Konto einen Eintrag im Schlüsselbund an, zeigt ihn und löscht ihn. Der Daemon liest beim Start jedes Token einmal. Das 1Password-Backend hält einen Client je Konto und leitet jede Referenz über den Vault an das Konto. Ein Konto ohne Token sperrt nur seine Vaults. `a38 doctor` und `a38 config vault list` zeigen das Konto. Eine Konfiguration mit einem Token lädt nicht; die Meldung nennt den Umbau.

Setzt auf: 1Password-Backend ({{ ref("F12") }}), Konfiguration ({{ ref("F15") }}), `a38 doctor` ({{ ref("F20") }}).

Setzt um: {{ ref("D70") }}, {{ ref("D71") }}, {{ ref("D72") }}, {{ ref("D73") }}, {{ ref("D75") }}; aus {{ ref("D74") }} den Teil für `a38 doctor` und `a38 config vault list`.
