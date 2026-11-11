---
render_macros: true
id: 'D28'
titel: 'Eine Policy auf dem Mac legt je Projekt die erlaubten Vaults fest'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 5
headless: true
status: gueltig
use_cases: [UC1, UC11]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Vorschlag des Designlaufs, von Stefan im Review am 2026-11-11 abgenommen
---
# D28 — Eine Policy auf dem Mac legt je Projekt die erlaubten Vaults fest

{{ design_kopf(page) }}

**Entscheidung:** Eine Konfigurationsdatei auf dem Mac nennt je Projekt die erlaubten Vaults. Der Daemon prüft jede Anfrage vor der Freigabe. Eine Anfrage außerhalb der Policy erreicht den Menschen nicht. Der Agent erhält den Code `not_allowed`. Der Daemon schreibt die Ablehnung in das Audit-Log.

**Begründung:** Der Mensch sieht nur Anfragen, die überhaupt zulässig sind. Das senkt die Zahl der Popups und begrenzt den Schaden eines gekaperten Agents ({{ ref("RI2") }}).

**Offen:** Format und Ort der Datei ({{ ref("G17") }}).
