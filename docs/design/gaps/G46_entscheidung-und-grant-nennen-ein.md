---
render_macros: true
id: 'G46'
titel: 'Entscheidung und Grant nennen ein Kommando mit einem Wert, bevor der Daemon die Werte prüft'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-16'
order: 8
headless: true
type: Gap
kind: design
status: geschlossen
beleg:
  wer: Stefan
  datum: '2026-11-16'
  quelle: ['#115']
redaktion:
  - {datum: '2026-11-16', issue: '#99'}
register:
  owner: Stefan
---
# G46 — Entscheidung und Grant nennen ein Kommando mit einem Wert, bevor der Daemon die Werte prüft

{{ design_kopf(page) }}

{{ ref("D65") }} legt die Reihenfolge fest: Der Daemon liest die Werte und prüft danach, ob Argumente oder Arbeitsverzeichnis einen Wert enthalten. Erst danach schreibt er den Eintrag für die Auslieferung. Dieselbe Entscheidung verlangt das Kommando in den Ereignissen Anfrage, Grant und Auslieferung. {{ ref("RI14") }} nennt nur den Eintrag für die Anfrage als Ort, an dem ein Wert stehen kann.

Ein Wert darf erst nach dem Grant gelesen werden ({{ ref("D36") }}, {{ ref("D63") }}). Die Ereignisse Entscheidung und Grant entstehen deshalb vor der Prüfung. Setzt der Agent einen Wert in ein Argument, steht der Wert auch in diesen zwei Einträgen. Die Freigabeanfrage zeigt das Kommando ebenfalls; der Mensch sieht den Wert dort.

**Umsetzung in Issue 99 (zu bestätigen durch Stefan):**

- Anfrage, Entscheidung und Grant tragen das Kommando, wie der Agent es sendet.
- Nach dem Fund schreibt der Daemon die Ablehnung mit dem Grund `invalid_request` und ohne das Kommando. Das Ende eines Grants für genau eine Auslieferung trägt ebenfalls kein Kommando.
- Ein Grant aus dieser Freigabe bleibt bestehen (dev, test). Er deckt nur Kommandos nach seiner Bindung.

Offene Fragen:

- Akzeptiert Stefan den Wert in Entscheidung und Grant als Teil von {{ ref("RI14") }}?
- Oder sollen Entscheidung und Grant das Kommando ohne Argumente nennen und nur die Anfrage das ganze Kommando?
- Soll ein Grant nach einem Fund enden, weil das Kommando den Wert in der Prozessliste zeigen würde?

**Ergebnis (Stefan, 2026-11-16):** Stefan akzeptiert den Wert in Entscheidung und Grant als bekannte Grenze von {{ ref("RI14") }}. Die Umsetzung aus Issue 99 bleibt. Die Referenz des Audit-Logs und die Anleitung zu `run_with_credentials` nennen die Grenze. Der Skill verbietet einen Wert im Kommando.

**Quelle:** [Issue 99](https://github.com/seletz/passierschein/issues/99), Umsetzung des Brokers für den exec-Weg.
