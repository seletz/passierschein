---
render_macros: true
id: 'RI12'
titel: 'Ein Bündel vergrößert den Schaden einer einzelnen Freigabe'
bereich: 'Anfrage und Bündelung'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: offen
wahrscheinlichkeit: mittel
auswirkung: niedrig
behandlung: [mindern]
register:
  owner: Stefan
---
# RI12 — Ein Bündel vergrößert den Schaden einer einzelnen Freigabe

{{ design_kopf(page) }}

**Beschreibung:** Ein Agent fragt aus Vorsicht mehr an, als er braucht. Ein gekaperter Agent bläht das Bündel auf. Der Mensch liest ein langes Bündel nicht gründlich.

**Bewertung:** Die Policy begrenzt das Bündel auf die dev-Vaults des Projekts.

**Maßnahme:** Obergrenze je Bündel ({{ ref("D29") }}). Kein Häkchen vorab gesetzt ({{ ref("D31") }}). Abweichung sichtbar ({{ ref("D8") }}).

**Revisit:** mit den Zahlen aus dem Audit-Log ({{ ref("G12") }}).

**Quelle:** kritische Prüfung, Schwachstelle zur Bündelung.
