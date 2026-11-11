---
render_macros: true
id: 'T26'
titel: 'Ein Fehler beim Schreiben des Audit-Logs verhindert die Auslieferung und meldet sich dem Menschen'
bereich: 'Audit'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [D36, D35, F7]
---
# T26 — Ein Fehler beim Schreiben des Audit-Logs verhindert die Auslieferung und meldet sich dem Menschen

{{ design_kopf(page) }}

**Form:** Go-Test mit `--backend fake` und `--ui script`. Die Audit-Datei ist nicht schreibbar, oder der Test schaltet einen Schreibfehler ein.

**Fälle:** Fehler beim Eintrag der Anfrage; Fehler beim Eintrag des Grants nach der Freigabe; Fehler beim Eintrag der Auslieferung unter einem bestehenden Grant.

**Erwartung:** Der Agent erhält `audit_failed`. Die Antwort enthält keinen Wert. Es entsteht kein Grant. Das Fake-Backend zählt nach dem Fehler keinen Lesezugriff auf einen Wert. Das Skript der Oberfläche erhält genau eine Meldung über den Fehler. Das Log des Daemons nennt den Fehler ohne Secret-Wert.

**Offen:** Form und Aktionen der Meldung ({{ ref("G20") }}).


Belegt: {{ ref("D36") }}, {{ ref("D35") }}, {{ ref("F7") }}.
