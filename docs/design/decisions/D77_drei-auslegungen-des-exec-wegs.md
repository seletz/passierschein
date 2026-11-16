---
render_macros: true
id: 'D77'
titel: 'Drei Auslegungen des exec-Wegs im Daemon'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-16'
order: 14
headless: true
status: gueltig
use_cases: [UC15]
beleg:
  wer: Stefan
  datum: '2026-11-16'
  quelle: ['#115']
---
# D77 — Drei Auslegungen des exec-Wegs im Daemon

{{ design_kopf(page) }}

**Entscheidung:** Der Daemon legt drei Stellen des exec-Wegs so aus. Die Entscheidung ergänzt {{ ref("D61") }}, {{ ref("D63") }} und {{ ref("D65") }}.

- **Zu viele Variablen:** Nennt `request_exec` mehr Variablen als `bundle.max_entries`, antwortet der Daemon mit `bundle_too_large`. Ein Kommando bildet ein eigenes Bündel ({{ ref("D61") }}); die Grenze eines Bündels gilt deshalb auch hier.
- **Weg eines Eintrags:** Ein Eintrag des Audit-Logs ohne Kommando gehört zum Weg `mcp`. Nur Einträge des exec-Wegs tragen das Kommando mit dem Weg `exec` ({{ ref("D65") }}). Ältere Einträge bleiben damit ohne Umbau lesbar.
- **Bindung eines Grants aus `request_credentials`:** Ein Grant aus `request_credentials` hat immer die Bindung `none`. In dev deckt er deshalb auch `run_with_credentials` ({{ ref("D63") }}, Grant aus dem anderen Weg).

**Begründung:** Die drei Stellen waren im Design offen. Die Umsetzung in Issue 99 wählte die Auslegung mit der kleinsten neuen Regel. Stefan hat sie bestätigt.

**Quelle:** Aussage von Stefan am 2026-11-16 zu [Pull Request 115](https://github.com/seletz/passierschein/pull/115), [Issue 99](https://github.com/seletz/passierschein/issues/99).
