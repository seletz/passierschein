---
render_macros: true
id: 'G48'
titel: 'Die feste Liste der Programme ohne Aussage erkennt nicht jede Shell und nicht jeden Starter'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-16'
order: 10
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G48 — Die feste Liste der Programme ohne Aussage erkennt nicht jede Shell und nicht jeden Starter

{{ design_kopf(page) }}

{{ ref("D63") }} macht bei einem Programm ohne Aussage aus der Bindung `program` die Bindung `command`. Die Liste ist fest im Code. Seit dem Review von Pull Request 115 vergleicht der Daemon den letzten Teil des aufgelösten Pfads. Er vergleicht ohne Groß- und Kleinschreibung und ohne Versionsendung (`python3.12`, `perl5.34`). Ein Link im Projekt auf `/bin/sh` gilt damit als `sh`.

Eine Liste kann drei Fälle nicht schließen:

- **Kopie unter anderem Namen:** Der Agent kopiert eine Shell ins Projekt und nennt sie `psql`. Der aufgelöste Pfad ist die Kopie. Der Name sagt nichts über den Inhalt.
- **Programme, die die Liste nicht nennt:** `ksh`, `tcsh`, `awk`, `make`, `find` mit `-exec`, `git` mit einem Alias, `ssh`, `docker`, `kubectl exec` und viele weitere führen ein beliebiges Kommando aus. Ein Grant mit der Bindung `program` deckt dann jedes Kommando über diese Programme.
- **Wechsel des Links:** `a38 mcp` startet den Pfad, den der Agent sendet, mit den Links ({{ ref("D61") }}). Der Daemon prüft und zeigt den aufgelösten Pfad. Ändert der Agent den Link zwischen der Prüfung und dem Start, läuft ein anderes Programm.

Die Bindung `program` in test ruht also auf der Annahme, dass der Pfad das Verhalten bestimmt. Diese Annahme gilt nur für Programme mit fester Aufgabe.

Offene Fragen:

- Soll test im Standard die Bindung `command` nutzen, solange keine bessere Prüfung des Programms existiert?
- Soll `a38 mcp` den aufgelösten Pfad starten, den der Daemon geprüft hat? Das braucht ein Feld in `exec_credentials` oder eine Prüfung in `a38 mcp`.
- Soll der Daemon ein Programm im Projektverzeichnis oder ein Programm, das der Benutzer schreiben kann, wie eine Shell behandeln?
- Soll die Konfiguration die Liste erweitern dürfen (nur erweitern, nie kürzen)?

**Quelle:** Unabhängiges Review von [Pull Request 115](https://github.com/seletz/passierschein/pull/115), Befund 1.
