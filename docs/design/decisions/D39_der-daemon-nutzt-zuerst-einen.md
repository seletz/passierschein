---
render_macros: true
id: 'D39'
titel: 'Der Daemon nutzt zuerst einen Service Account, die Desktop-Integration folgt hinter einem Build-Tag'
bereich: 'Backends'
lauf: '1.0'
stand: '2026-11-11'
order: 3
headless: true
status: gueltig
use_cases: [UC11]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Vorschlag des Designlaufs, von Stefan im Review am 2026-11-11 abgenommen
---
# D39 — Der Daemon nutzt zuerst einen Service Account, die Desktop-Integration folgt hinter einem Build-Tag

{{ design_kopf(page) }}

**Entscheidung:** Das 1Password-Backend nutzt zuerst einen Service Account, der nur die dev-Vaults lesen darf. Die Desktop-Integration des SDK ist eine spätere zweite Variante hinter einem Build-Tag.

**Begründung:** Ein Service Account braucht kein CGO. Der statische Build und die Signatur bleiben unverändert. Die Rechte begrenzen den Schaden auf die dev-Vaults. Die Desktop-Integration braucht CGO, gilt für das ganze Konto und hat offene Fehler.

**Grenze:** Die Grundlage ist nur die Doku von 1Password. Nichts davon ist praktisch getestet ({{ ref("G3") }}, {{ ref("RI9") }}).

**Quelle:** [Spike 1Password-Go-SDK](https://github.com/seletz/passierschein/issues/3).
