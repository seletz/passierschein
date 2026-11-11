---
render_macros: true
id: 'RI7'
titel: 'Ein Prozess des Benutzers ändert das Audit-Log unbemerkt'
bereich: 'Audit'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: offen
wahrscheinlichkeit: niedrig
auswirkung: mittel
behandlung: [akzeptieren]
beleg:
  notiz: Annahme zu bestätigen durch Stefan
  annahme: true
register:
  owner: Stefan
---
# RI7 — Ein Prozess des Benutzers ändert das Audit-Log unbemerkt

{{ design_kopf(page) }}

**Beschreibung:** Wer die Datei ändern kann, kann die Hash-Kette neu berechnen oder das Ende abschneiden.

**Bewertung:** Das Audit-Log dient der Auswertung, nicht dem Beweis ({{ ref("D37") }}).

**Maßnahme:** Keine im MVP. Möglich ist später ein Anker außerhalb des Macs.

**Revisit:** wenn `a38` für mehr als eine Person gelten soll.

**Quelle:** kritische Prüfung, Schwachstelle zur Integrität des Logs.
