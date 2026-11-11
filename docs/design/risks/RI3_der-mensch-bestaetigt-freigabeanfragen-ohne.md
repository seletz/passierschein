---
render_macros: true
id: 'RI3'
titel: 'Der Mensch bestätigt Freigabeanfragen, ohne sie zu lesen'
bereich: 'Freigabe'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: offen
wahrscheinlichkeit: hoch
auswirkung: mittel
behandlung: [mindern]
register:
  owner: Stefan
---
# RI3 — Der Mensch bestätigt Freigabeanfragen, ohne sie zu lesen

{{ design_kopf(page) }}

**Beschreibung:** Bei vielen Agents entstehen viele Freigabeanfragen. Nach einiger Zeit bestätigt der Mensch aus Gewohnheit.

**Bewertung:** Das ist das Ausgangsproblem. Ein Popup mit mehr Text löst es nicht von selbst.

**Maßnahme:** Bündelung ({{ ref("D5") }}). Grants mit Laufzeit ({{ ref("D8") }}). Policy ohne Popup ({{ ref("D28") }}). Verzögerung und kein Standard-Button ({{ ref("D31") }}). Kennzahlen ({{ ref("G12") }}).

**Revisit:** nach vier Wochen Betrieb mit den Zahlen aus dem Audit-Log.

**Quelle:** kritische Prüfung, kritische Schwachstelle zur Freigabemüdigkeit.
