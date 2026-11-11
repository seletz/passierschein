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
beweist: [F3, D24, D28]
---
# T3 — Eine Anfrage außerhalb der Policy oder außerhalb von dev erreicht die Freigabe nicht

{{ design_kopf(page) }}

**Form:** Tabellengetriebener Go-Test mit `--ui script`.

**Fälle:** Vault ohne `k8s` im Namen; Vault nicht in der Allowlist des Projekts; Vault mit dem Segment `k8s-test`; Vault mit dem Segment `k8s`.

**Erwartung:** Jeder Fall endet mit `not_allowed`. Das Skript der Freigabe läuft nie. Das Audit-Log enthält je Fall eine Ablehnung.

**Scheitert, wenn:** eine Freigabeanfrage entsteht oder ein Wert zurückkommt.


Belegt: {{ ref("F3") }}, {{ ref("D24") }}, {{ ref("D28") }}.
