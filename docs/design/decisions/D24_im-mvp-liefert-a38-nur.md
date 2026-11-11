---
render_macros: true
id: 'D24'
titel: 'Im MVP liefert a38 nur aus dev-Vaults aus'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 4
headless: true
status: gueltig
use_cases: [UC5]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Vorschlag des Designlaufs, von Stefan im Review am 2026-11-11 abgenommen
---
# D24 — Im MVP liefert a38 nur aus dev-Vaults aus

{{ design_kopf(page) }}

**Entscheidung:** Bis zur Stufe 5 lehnt der Daemon jede Anfrage außerhalb von dev ohne Freigabeanfrage ab. Die Auslieferung als Datei ({{ ref("D10") }}) entsteht mit der VM.

**Begründung:** Lokal läuft der Agent unter demselben Benutzer. Eine Datei auf einem tmpfs schützt den Wert dort nicht. Die Ablehnung ist einfacher und sicherer als ein halber Schutz.

**Folge:** Stefan braucht im MVP für Zugangsdaten außerhalb von dev weiterhin den bisherigen Weg.
