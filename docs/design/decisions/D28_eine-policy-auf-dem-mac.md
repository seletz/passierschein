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

**Alles oder nichts:** Enthält eine Anfrage eine erlaubte und eine nicht erlaubte Referenz, lehnt der Daemon die ganze Anfrage mit `not_allowed` ab. Es gibt keine Teilauslieferung durch die Policy.

**Fehlerhafte Konfiguration eines Projekts:** Nennt ein Projekt einen Vault, der in der Liste der Vaults fehlt ({{ ref("D12") }}), läuft der Daemon weiter. Der Daemon lehnt jede Anfrage dieses Projekts mit dem Code `config_invalid` ab. Der Daemon zeigt dem Menschen ein Popup mit dem Grund, wie bei einem Fehler des Audit-Logs ({{ ref("D36") }}). Andere Projekte arbeiten weiter. Stefan hat das am 2026-11-11 im Review festgelegt.

**Begründung:** Der Mensch sieht nur Anfragen, die überhaupt zulässig sind. Ein einzelnes falsch konfiguriertes Projekt darf den Daemon nicht lahmlegen. Das senkt die Zahl der Popups und begrenzt den Schaden eines gekaperten Agents ({{ ref("RI2") }}).

**Offen:** Format und Ort der Datei ({{ ref("G17") }}).
