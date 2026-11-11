---
render_macros: true
id: 'RI2'
titel: 'Ein gekaperter Agent liefert eine plausible Begründung, und der Mensch gibt frei'
bereich: 'Freigabe'
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
# RI2 — Ein gekaperter Agent liefert eine plausible Begründung, und der Mensch gibt frei

{{ design_kopf(page) }}

**Beschreibung:** Name, Aufgabe und Begründung stammen vom Agent. Eine Prompt Injection kann genau den Text erzeugen, den der Mensch erwartet.

**Bewertung:** Prompt Injection ist ein bekannter Angriff. Die Auswirkung ist auf die erlaubten dev-Vaults des Projekts begrenzt.

**Maßnahme:** Geprüfte Fakten getrennt von den Angaben ({{ ref("D4") }}). Policy vor der Freigabe ({{ ref("D28") }}). Kleinster Umfang des Grants ({{ ref("D32") }}).

**Revisit:** mit der Stufe 5 und mit jeder Erweiterung über dev hinaus.

**Quelle:** kritische Prüfung, kritische Schwachstelle zur Identität.
