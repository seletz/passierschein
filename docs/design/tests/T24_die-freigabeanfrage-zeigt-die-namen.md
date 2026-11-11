---
render_macros: true
id: 'T24'
titel: 'Die Freigabeanfrage zeigt die Namen des Backends und nicht den Text des Agents'
bereich: 'Anfrage und Bündelung'
lauf: '1.0'
stand: '2026-11-11'
order: 5
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [D27, F9]
---
# T24 — Die Freigabeanfrage zeigt die Namen des Backends und nicht den Text des Agents

{{ design_kopf(page) }}

**Form:** Tabellengetriebener Go-Test mit `--ui script`. Das Skript speichert die Freigabeanfrage.

**Fälle und Erwartung:**

- Der Agent schreibt die Referenz in anderer Schreibweise. Die Freigabeanfrage zeigt die Namen aus der YAML-Datei des Fake-Backends.
- Ungültige Referenzen enden mit `invalid_request`: `op://vault/item` ohne Feld; ein Segment zu viel; ein anderes Schema; ein leeres Segment.

- Ein unbekanntes Item und ein unbekanntes Feld in einem erlaubten Vault enden mit `not_found`, ohne Freigabeanfrage. Ein Vault außerhalb der Policy endet mit `not_allowed`.


Belegt: {{ ref("D27") }}, {{ ref("F9") }}.
