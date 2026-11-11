---
render_macros: true
id: 'RI6'
titel: 'Der Mensch gibt versehentlich frei, oder ein anderer Prozess zeigt ein gefälschtes Popup'
bereich: 'Freigabe'
lauf: '1.0'
stand: '2026-11-11'
order: 3
headless: true
status: offen
wahrscheinlichkeit: niedrig
auswirkung: mittel
behandlung: [mindern]
register:
  owner: Stefan
---
# RI6 — Der Mensch gibt versehentlich frei, oder ein anderer Prozess zeigt ein gefälschtes Popup

{{ design_kopf(page) }}

**Beschreibung:** Ein Popup erscheint während des Tippens. Ein anderer Prozess kann ein ähnliches Fenster zeigen.

**Bewertung:** Touch ID verlangt eine körperliche Handlung. Ein gefälschtes Fenster kann keine Freigabe im Daemon erzeugen, aber den Menschen an falsche Popups gewöhnen.

**Maßnahme:** Kein Standard-Button, Verzögerung, kein Tastaturfokus, nur Touch ID ({{ ref("D31") }}, {{ ref("D45") }}).

**Revisit:** nach der Sichtprüfung ({{ ref("G4") }}).

**Quelle:** kritische Prüfung, Schwachstelle zu Popup und Fokus.
