---
render_macros: true
id: 'T28'
titel: 'Eine Störung des Daemons sperrt die Auslieferung für alle Sitzungen, bis sie behoben ist'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 7
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [D48, D36]
---
# T28 — Eine Störung des Daemons sperrt die Auslieferung für alle Sitzungen, bis sie behoben ist

{{ design_kopf(page) }}

**Form:** Go-Test mit `--backend fake`, `--ui script` und zwei Sitzungen in zwei Projekten. Der Test schaltet einen Schreibfehler des Audit-Logs ein und wieder aus.

**Schritte und Erwartung:**

- Sitzung A hat einen gültigen Grant. Der Schreibfehler tritt bei einer Anfrage von Sitzung B ein. B erhält `audit_failed`.
- Danach endet jede Anfrage von A und von B mit `daemon_faulted`, auch im Rahmen des Grants von A. Es entsteht keine Freigabeanfrage. Das Fake-Backend zählt keinen Lesezugriff.
- Das Skript der Oberfläche erhält genau eine Meldung über die Störung, nicht eine je Anfrage.
- `a38 grants list`, `a38 grants revoke` und `a38 doctor` arbeiten während der Sperre. `a38 doctor` nennt die Störung.
- Der Test behebt die Ursache. Die Sperre bleibt: Jede Anfrage endet weiter mit `daemon_faulted`.
- Der Admin-Befehl zum Aufheben läuft über den Admin-Socket. Danach arbeiten beide Sitzungen wieder. Das Audit-Log enthält Beginn und Ende der Sperre.
- Der Admin-Befehl bei bestehender Ursache hebt die Sperre nicht auf und nennt den Befund.
- Der Befehl zum Aufheben über den Agent-Socket endet mit `invalid_request`.
- Ein Neustart des Daemons bei bestehender Ursache führt wieder in die Sperre.
- Die fehlerhafte Konfiguration eines Projekts sperrt nur dieses Projekt und löst keine globale Sperre aus.


Belegt: {{ ref("D48") }}, {{ ref("D36") }}.
