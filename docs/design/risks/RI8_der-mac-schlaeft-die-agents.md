---
render_macros: true
id: 'RI8'
titel: 'Der Mac schläft, die Agents warten und suchen Umwege'
bereich: 'Anfrage und Bündelung'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: offen
wahrscheinlichkeit: mittel
auswirkung: mittel
behandlung: [mindern]
register:
  owner: Stefan
---
# RI8 — Der Mac schläft, die Agents warten und suchen Umwege

{{ design_kopf(page) }}

**Beschreibung:** Ohne Daemon und ohne Mensch gibt es keine Freigabe. Ein Agent mit einer unklaren Fehlermeldung sucht einen anderen Weg zu den Zugangsdaten.

**Bewertung:** Für die tägliche Arbeit am Mac ist das selten. Für eine Maschine, die immer läuft, ist es der Normalfall ({{ ref("G9") }}).

**Maßnahme:** Stabile Fehlercodes ({{ ref("D35") }}). Wartezeit der Freigabe ({{ ref("D30") }}). Regel im Skill: nach einer Ablehnung stoppen ({{ ref("D22") }}).

**Revisit:** mit der Stufe 5.

**Quelle:** kritische Prüfung, Schwachstelle zur Verfügbarkeit.
