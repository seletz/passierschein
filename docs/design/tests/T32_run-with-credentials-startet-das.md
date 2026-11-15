---
render_macros: true
id: 'T32'
titel: 'run_with_credentials startet das Kommando mit dem Wert in der Umgebung und gibt nur Exit-Code und Ausgabe zurück'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-15'
order: 8
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [F21, D61, UC15]
---
# T32 — run_with_credentials startet das Kommando mit dem Wert in der Umgebung und gibt nur Exit-Code und Ausgabe zurück

{{ design_kopf(page) }}

**Form:** Go-Test mit Fake-Backend und Fake-UI `auto-approve`. Ein kleines Testprogramm liest eine Umgebungsvariable und gibt ihre Länge und einen festen Text aus.

**Vorbedingung:** Der Vault `beispiel.app.k8s-dev` ist dev. Das Fake-Backend kennt `op://beispiel.app.k8s-dev/datenbank-dev/password`.

**Schritte:**

1. `run_with_credentials` mit dem Testprogramm und `env` `{"DB_PASSWORD": "op://beispiel.app.k8s-dev/datenbank-dev/password"}`.
2. Dasselbe mit einem Testprogramm, das mit Exit-Code 3 endet.
3. Dasselbe mit einem Programm, das es nicht gibt.

**Erwartung:**

1. Das Ergebnis hat `exit_code` 0, die richtige Länge in `stdout` und kein `isError`. Kein Byte der MCP-Antwort enthält den Wert.
2. Das Ergebnis hat `exit_code` 3 und kein `isError`.
3. `isError` mit `invalid_request`. Der Daemon hat keine Anfrage erhalten.

**Scheitert, wenn:** der Wert in der MCP-Antwort steht, das Kommando den Wert nicht erhält oder ein Exit-Code ungleich 0 als Fehler von `a38` kommt.
