---
render_macros: true
id: 'G35'
titel: 'Ob der exec-Weg in der VM den Wert in die VM bringen darf, ist offen'
bereich: 'Grenzen und Identität'
lauf: '2.0'
stand: '2026-11-15'
order: 7
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G35 — Ob der exec-Weg in der VM den Wert in die VM bringen darf, ist offen

{{ design_kopf(page) }}

Der exec-Weg ({{ ref("D61") }}) startet das Kommando in `a38 mcp`. In der VM läuft `a38 mcp` in der VM. Der Wert gelangt dann in die VM, in die Umgebung eines Prozesses dort. Ein gekaperter Agent in der VM liest ihn dort, etwa über `/proc/<pid>/environ`.

Für den lokalen Einsatz ist das kein Unterschied zu heute ({{ ref("D60") }}). Für die VM ist es die Frage, ob test und prod die VM überhaupt erreichen dürfen.

Offene Fragen:

- Darf ein Wert aus test oder prod in die VM, oder nur aus dev?
- Braucht die VM eine Ausführung außerhalb der Zone des Agents, zum Beispiel unter einem eigenen Benutzer in der VM oder auf dem Mac?
- Prüft der Daemon in der VM etwas, das er lokal nicht prüfen kann, etwa Pfade im Projekt?

Der Vertrag des Werkzeugs bleibt bei jeder Antwort gleich. Nur der Ort der Ausführung ändert sich.

**Quelle:** [Issue 91](https://github.com/seletz/passierschein/issues/91), Frage 1.
