---
render_macros: true
id: 'G49'
titel: 'exec_result ohne Auslieferung und verworfene offene Auslieferungen haben keine Grenze'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-16'
order: 11
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G49 — exec_result ohne Auslieferung und verworfene offene Auslieferungen haben keine Grenze

{{ design_kopf(page) }}

Der Daemon ordnet `exec_result` einer Auslieferung über die ID zu, die `a38 mcp` für `request_exec` wählt. Eine Sitzung hält höchstens 64 Auslieferungen, die auf ihr `exec_result` warten. Seit dem Review von Pull Request 115 lehnt der Daemon ein `request_exec` mit der ID einer wartenden Auslieferung ab (`invalid_request`). Zwei Lücken bleiben:

- **`exec_result` ohne Auslieferung:** Jedes solche `exec_result` schreibt einen Fehler in das Audit-Log. Es gibt keine Grenze. Ein fehlerhafter oder bösartiger Client füllt das Audit-Log und den Speicherplatz.
- **Verworfene Auslieferung:** Wartet die 65. Auslieferung, verwirft der Daemon die älteste ohne Eintrag. Das Audit-Log zeigt dann eine Auslieferung ohne Ausführung und ohne Grund. Ihr `exec_result` endet später mit `invalid_request`.

**Vorschlag (zu bestätigen durch Stefan):**

- Je Sitzung schreibt der Daemon höchstens 10 Fehler für `exec_result` ohne Auslieferung. Danach zählt er nur und schreibt beim Ende der Sitzung einen Eintrag mit der Zahl. Die Antwort bleibt `invalid_request`.
- Warten 64 Auslieferungen, lehnt der Daemon ein weiteres `request_exec` vor der Freigabe ab (`invalid_request`, Text „too many commands wait for their result“). Er verwirft keine Auslieferung.
- Endet eine Sitzung mit wartenden Auslieferungen, schreibt der Daemon je Auslieferung ein Ereignis Ausführung mit dem Vermerk „kein Ergebnis“.

Offene Fragen:

- Bestätigt Stefan die Grenze von 10 Fehlern je Sitzung und die Ablehnung statt des Verwerfens?
- Braucht das Audit-Log den Vermerk „kein Ergebnis“, oder genügt die Auslieferung ohne Ausführung?

**Quelle:** Unabhängiges Review von [Pull Request 115](https://github.com/seletz/passierschein/pull/115), Befund 4.
