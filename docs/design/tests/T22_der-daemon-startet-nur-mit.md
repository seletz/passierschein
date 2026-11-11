---
render_macros: true
id: 'T22'
titel: 'Der Daemon startet nur mit einer vollständigen Konfiguration der Vaults'
bereich: 'Backends'
lauf: '1.0'
stand: '2026-11-11'
order: 3
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [D12, D47, F15]
---
# T22 — Der Daemon startet nur mit einer vollständigen Konfiguration der Vaults

{{ design_kopf(page) }}

**Form:** Tabellengetriebener Go-Test über Konfigurationsdateien, dazu zwei Fälle mit `--backend fake`.

**Fälle ohne Start:** ein Vault ohne Angabe der Umgebung; eine Umgebung außerhalb von dev, test und prod (`staging`, leer, `Dev`); derselbe Vault zweimal mit verschiedenen Umgebungen.

**Erwartung ohne Start:** Der Exit-Code ist nicht 0. Die Fehlermeldung nennt den Vault. Es entsteht kein Socket.

**Fälle ohne Namensvergleich:** Ein Vault mit dem Namen `beispiel-prod` und der Umgebung dev liefert aus. Ein Vault mit dem Namen `beispiel-dev` und der Umgebung prod endet mit `not_allowed`. Ein Vault `k8s-beispiel` im Fake-Backend, aber nicht in der Konfiguration, endet mit `not_allowed`; das Backend zählt keine Auflösung und keinen Lesezugriff.

**Offen:** Ob ein Vault in der Allowlist eines Projekts, der in der Liste der Vaults fehlt, den Start verhindert.


Belegt: {{ ref("D12") }}, {{ ref("D47") }}, {{ ref("F15") }}.
