---
render_macros: true
id: 'UC8'
titel: 'Der Mensch prüft die Integrität des Audit-Logs'
bereich: 'Audit'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: aktiv
stufe: 1
---
# UC8 — Der Mensch prüft die Integrität des Audit-Logs

{{ design_kopf(page) }}

**Akteure:** Mensch.

**Auslöser:** Der Mensch will wissen, ob das Audit-Log unverändert ist.

**Ablauf:**

1. Der Mensch ruft `a38 audit verify` auf.
2. Der Befehl berechnet die Hash-Kette über alle Einträge neu ({{ ref("D36") }}).
3. Der Befehl meldet den ersten Eintrag, dessen Hash nicht passt.

**Ergebnis:** Exit-Code 0 bei einer unversehrten Kette, sonst ein Fehler mit der Nummer des Eintrags.

**Grenze:** Die Prüfung erkennt eine Änderung. Sie erkennt keine Kette, die jemand vollständig neu berechnet hat ({{ ref("D37") }}).
