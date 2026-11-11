---
render_macros: true
id: 'RI11'
titel: 'Ein ausgeliefertes Secret lässt sich nicht zurückholen'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: offen
wahrscheinlichkeit: hoch
auswirkung: niedrig
behandlung: [mindern]
register:
  owner: Stefan
---
# RI11 — Ein ausgeliefertes Secret lässt sich nicht zurückholen

{{ design_kopf(page) }}

**Beschreibung:** Ein Widerruf beendet den Grant. Der Wert bleibt beim Agent.

**Bewertung:** Das gilt für jede Auslieferung. In dev ist der Schaden klein.

**Maßnahme:** Rotation in 1Password ({{ ref("G13") }}). Das Audit-Log nennt, welche Sitzung welche Zugangsdaten erhalten hat ({{ ref("D36") }}).

**Revisit:** mit jeder Erweiterung über dev hinaus.

**Quelle:** kritische Prüfung, Schwachstelle zu Widerruf und Rotation.
