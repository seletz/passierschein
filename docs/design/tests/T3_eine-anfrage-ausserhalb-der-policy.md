---
render_macros: true
id: 'T3'
titel: 'Eine Anfrage außerhalb der Policy oder außerhalb von dev erreicht die Freigabe nicht'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [F3, D12, D24, D28, D47]
---
# T3 — Eine Anfrage außerhalb der Policy oder außerhalb von dev erreicht die Freigabe nicht

{{ design_kopf(page) }}

**Form:** Tabellengetriebener Go-Test mit `--ui script`.

**Fälle:** Vault nicht in der Konfiguration; Vault im Backend vorhanden, aber nicht in der Konfiguration; Vault nicht in der Allowlist des Projekts; Vault mit der Umgebung test; Vault mit der Umgebung prod.

**Erwartung:** Jeder Fall endet mit `not_allowed`. Ein unbekannter Vault und ein vorhandener, nicht genannter Vault ergeben dieselbe Antwort. Das Skript der Freigabe läuft nie. Das Fake-Backend zählt keine Auflösung eines Namens und keinen Lesezugriff. Das Audit-Log enthält je Fall eine Ablehnung.

**Alles oder nichts:** Eine Anfrage mit einer erlaubten und einer nicht erlaubten Referenz endet ganz mit `not_allowed`. Das Fake-Backend zählt auch für die erlaubte Referenz keinen Lesezugriff.

**Scheitert, wenn:** eine Freigabeanfrage entsteht oder ein Wert zurückkommt.


Belegt: {{ ref("F3") }}, {{ ref("D12") }}, {{ ref("D24") }}, {{ ref("D28") }}, {{ ref("D47") }}.
