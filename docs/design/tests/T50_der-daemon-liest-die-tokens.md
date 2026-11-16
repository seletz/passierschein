---
render_macros: true
id: 'T50'
titel: 'Der Daemon liest die Tokens der Konten nebeneinander und wartet höchstens eine Frist'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-16'
order: 9
headless: true
status: aktiv
umsetzung: vorhanden
beweist: [F25, D79]
---
# T50 — Der Daemon liest die Tokens der Konten nebeneinander und wartet höchstens eine Frist

{{ design_kopf(page) }}

**Form:** Go-Test über das Lesen der Tokens beim Start (`TestOpenAccountsSideBySide` in `cmd/a38`) mit Quellen im Arbeitsspeicher und einem gefälschten Schlüsselbund. Kein echter Schlüsselbund, kein Dialog.

**Schritte:**

1. Zwei Quellen geben ihr Token erst, wenn beide Quellen begonnen haben.
2. Zwei Einträge im Schlüsselbund warten auf einen Dialog; die Frist ist 300 Millisekunden.

**Erwartung:** 1 liefert beide Tokens in der Reihenfolge der Konfiguration. 2 sperrt beide Konten nach etwa einer Frist, nicht nach zwei; die Meldung nennt beide Konten mit der Frist.

**Scheitert, wenn:** der Daemon die Quellen nacheinander liest.
