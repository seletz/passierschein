---
render_macros: true
id: 'T34'
titel: 'Zeitgrenze, Ausgabegrenze und verbotene Variablennamen begrenzen das Kommando'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-15'
order: 10
headless: true
status: aktiv
umsetzung: vorhanden
beweist: [F21, D64]
---
# T34 — Zeitgrenze, Ausgabegrenze und verbotene Variablennamen begrenzen das Kommando

{{ design_kopf(page) }}

**Form:** Go-Test in `a38 mcp` mit Fake-Daemon.

**Schritte:**

1. Ein Kommando, das länger läuft als `timeout_seconds` und einen Kindprozess startet.
2. Ein Kommando mit 200 KiB Ausgabe bei `output_limit` 64 KiB.
3. `timeout_seconds` über `exec.max_timeout`.
4. `env` mit `PATH`, `DYLD_INSERT_LIBRARIES`, `A38_CONFIG` und `kleinbuchstaben`.
5. `cwd` außerhalb des Projekts.

**Erwartung:**

1. `timed_out: true`. Kein Prozess der Prozessgruppe lebt danach.
2. `stdout_truncated: true`; die Ausgabe ist das Ende mit 64 KiB.
3. bis 5. `invalid_request` ohne Start. In Schritt 5 lehnt auch der Daemon selbst ab, wenn `a38 mcp` die Prüfung auslässt (`not_allowed`).

**Scheitert, wenn:** ein Prozess weiterläuft, die Ausgabe größer ist als die Grenze oder ein verbotener Name durchkommt.
