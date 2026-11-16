---
render_macros: true
id: 'D76'
titel: 'Laufzeit 0 heißt in jeder Umgebung „nur dieses Kommando“'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-16'
order: 13
headless: true
status: gueltig
use_cases: [UC15]
beleg:
  wer: Stefan
  datum: '2026-11-16'
  quelle: ['#115']
---
# D76 — Laufzeit 0 heißt in jeder Umgebung „nur dieses Kommando“

{{ design_kopf(page) }}

**Entscheidung:** Die Laufzeit 0 heißt in jeder Umgebung „nur dieses Kommando, einmal“. Setzt die Konfiguration `environments.dev.grant_lifetime` oder `environments.test.grant_lifetime` auf `0s`, fragt der Daemon den Menschen bei jedem Kommando. Die Entscheidung ergänzt {{ ref("D63") }} und {{ ref("D65") }}. Sie schließt {{ ref("G47") }}.

- **Freigabeanfrage:** Ist die kürzeste Laufzeit der Umgebungen einer Anfrage 0, hat das Objekt `exec` die Art `once` mit der Bindung `command` und ohne Wahl der Laufzeit; `grant.ttl_seconds` ist 0. Das gilt für dev, test und prod. Die Freigabe-App zeigt dann „nur dieses Kommando, einmal“ wie in prod.
- **Grant:** Der Daemon vergibt einen Grant für genau eine Auslieferung ({{ ref("D63") }}, Laufzeit 0). `grant_status` zeigt ihn nicht.
- **MCP-Weg:** Die Antwort von `request_credentials` hat kein Kommando. Bei der Laufzeit 0 in dev antwortet der Daemon deshalb mit `exec_required`, wenn dev den exec-Weg erlaubt, sonst mit `not_allowed`. Der Agent nutzt dann `run_with_credentials`.
- **Grenzen:** Die Konfiguration erlaubt `0s` bis `24h` wie bisher ({{ ref("D66") }}). Eine Laufzeit größer als 0 hat in dev und test weiter die Art `grant`. prod behält die feste Laufzeit 0.

**Begründung:**

- Eine Konfiguration mit `0s` in dev oder test lud, aber keine Freigabe gelang ({{ ref("G47") }}). Der Mensch wählt `0s`, wenn er jedes Kommando sehen will. Dieselbe Art wie in prod erfüllt diesen Wunsch ohne neue Art.
- Ein Grant für genau eine Auslieferung passt nur zum exec-Weg. Beim MCP-Weg geht der Wert in die Antwort an den Agent und bleibt dort; „einmal“ hätte dort keine Wirkung.

**Quelle:** Aussage von Stefan am 2026-11-16 zu {{ ref("G47") }}: „Lücken wie empfohlen“; die Empfehlung war die Art `once` auch für dev und test bei Laufzeit 0. [Pull Request 115](https://github.com/seletz/passierschein/pull/115).
