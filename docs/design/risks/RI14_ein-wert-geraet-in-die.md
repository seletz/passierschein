---
render_macros: true
id: 'RI14'
titel: 'Ein Wert gerät in die Argumente eines Kommandos und damit in Prozessliste und Audit-Log'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-15'
order: 4
headless: true
status: offen
wahrscheinlichkeit: mittel
auswirkung: mittel
behandlung: [mindern]
register:
  owner: Stefan
---
# RI14 — Ein Wert gerät in die Argumente eines Kommandos und damit in Prozessliste und Audit-Log

{{ design_kopf(page) }}

**Beschreibung:** Ein Agent setzt einen Wert in die Argumente, etwa einen dev-Wert aus einer MCP-Antwort oder über `sh -c` mit `"$TOKEN"` in einem Aufruf von `curl`. Argumente stehen in der Prozessliste, die unter macOS jeder Benutzer lesen kann. Sie stehen in der Freigabeanfrage und im Audit-Log.

**Bewertung:** Wahrscheinlich bei Werkzeugen, die ein Secret nur als Argument annehmen. Die Auswirkung ist mittel: Das Audit-Log ist eine Datei des Benutzers ({{ ref("RI7") }}).

**Auslöser:** Ein Kommando, dessen Argumente einen Wert enthalten.

**Maßnahme:** Der Daemon liefert nicht aus, wenn Argumente oder Arbeitsverzeichnis einen der ausgelieferten Werte enthalten ({{ ref("D65") }}). Der Eintrag für die Anfrage entsteht vorher und kann einen Wert enthalten, der nicht zur Anfrage gehört. Regel im Skill: nie einen Wert in die Argumente ({{ ref("D68") }}). Eine Expansion in der Shell zur Laufzeit (`"$TOKEN"` in `sh -c`) erkennt der Daemon nicht.

**Revisit:** nach dem ersten Monat mit dem exec-Weg.

**Quelle:** [Issue 91](https://github.com/seletz/passierschein/issues/91).
