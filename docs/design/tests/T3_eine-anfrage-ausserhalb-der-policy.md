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

**Fälle:** Vault nicht in der Konfiguration; Vault nicht in der Allowlist des Projekts; Vault mit der Umgebung test; Vault mit der Umgebung prod; Vault ohne Angabe der Umgebung (der Daemon startet nicht).

**Erwartung:** Jeder Fall endet mit `not_allowed`. Das Skript der Freigabe läuft nie. Das Audit-Log enthält je Fall eine Ablehnung.

**Scheitert, wenn:** eine Freigabeanfrage entsteht oder ein Wert zurückkommt.


Belegt: {{ ref("F3") }}, {{ ref("D24") }}, {{ ref("D28") }}.
