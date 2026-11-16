---
render_macros: true
id: 'T38'
titel: 'Freigabeanfrage und Audit-Log nennen das Kommando und enthalten nie einen Wert oder die Ausgabe'
bereich: 'Freigabe'
lauf: '2.0'
stand: '2026-11-15'
order: 5
headless: true
status: aktiv
umsetzung: vorhanden
beweist: [F23, D65]
---
# T38 — Freigabeanfrage und Audit-Log nennen das Kommando und enthalten nie einen Wert oder die Ausgabe

{{ design_kopf(page) }}

**Form:** Go-Test mit Fake-Backend und Fake-UI `script`. Das Skript speichert die Freigabeanfrage. Ein zweiter Test prüft die Darstellung in der Freigabe-App mit einer festen Freigabeanfrage.

**Schritte:**

1. `run_with_credentials` für test mit einem Kommando, dessen Ausgabe einen festen Text enthält.
2. Freigabeanfrage aus dem Skript lesen.
3. Alle Einträge des Audit-Logs der Sitzung lesen.

**Erwartung:**

1. Die Freigabeanfrage enthält Umgebung, Pfad, Argumente, Arbeitsverzeichnis, Namen der Variablen und die Namen des Backends.
2. Das Audit-Log hat Anfrage, Entscheidung, Grant, Auslieferung mit Weg `exec` und Kommando sowie Ausführung mit Exit-Code.
3. Weder die Freigabeanfrage noch ein Eintrag enthält den Wert oder den festen Text der Ausgabe.
4. `a38 audit verify` meldet eine gültige Kette.

**Scheitert, wenn:** ein Feld fehlt oder ein Wert oder ein Teil der Ausgabe erscheint.
