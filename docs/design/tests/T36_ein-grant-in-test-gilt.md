---
render_macros: true
id: 'T36'
titel: 'Ein Grant in test gilt nur für das freigegebene Programm, und in prod braucht jedes Kommando eine Freigabe'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-15'
order: 12
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [F22, D63]
---
# T36 — Ein Grant in test gilt nur für das freigegebene Programm, und in prod braucht jedes Kommando eine Freigabe

{{ design_kopf(page) }}

**Form:** Go-Test mit Fake-Backend, Fake-UI `script` und Fake-Uhr.

**Schritte:**

1. test: `run_with_credentials` mit `/usr/bin/true`, danach mit `/usr/bin/true` und anderen Argumenten, danach mit `/usr/bin/env`.
2. test: `sh -c` mit zwei verschiedenen Texten.
3. test: Uhr 31 Minuten weiter, dasselbe Kommando.
4. prod (eingeschaltet): dasselbe Kommando zweimal.
5. dev: `request_credentials`, danach `run_with_credentials` mit denselben Referenzen.

**Erwartung:**

1. Erstes und drittes Kommando zeigen eine Freigabeanfrage, das zweite nicht.
2. Jedes Kommando zeigt eine Freigabeanfrage.
3. Neue Freigabeanfrage.
4. Zwei Freigabeanfragen; `grant_status` zeigt keinen Grant.
5. Eine Freigabeanfrage.

**Scheitert, wenn:** ein Grant ein anderes Programm abdeckt, `sh` als Programm einen Grant wiederverwendet oder prod einen Grant wiederverwendet.
