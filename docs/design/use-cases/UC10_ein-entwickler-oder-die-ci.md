---
render_macros: true
id: 'UC10'
titel: 'Ein Entwickler oder die CI testet den Ablauf ohne 1Password und ohne Popup'
bereich: 'Backends'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
stufe: 1
---
# UC10 — Ein Entwickler oder die CI testet den Ablauf ohne 1Password und ohne Popup

{{ design_kopf(page) }}

**Akteure:** Entwickler, CI.

**Auslöser:** Ein Test soll den ganzen Ablauf prüfen.

**Ablauf:**

1. Der Test startet `a38 serve --backend fake --ui auto-approve` mit einer YAML-Datei als Vault ({{ ref("D38") }}, {{ ref("D40") }}).
2. Der Test startet `a38 mcp` und sendet Anfragen.
3. Der Test prüft die Antworten und das Audit-Log.
4. Der Test wiederholt den Ablauf mit `--ui auto-deny` und `--ui script`.

**Ergebnis:** Bündelung, Laufzeit, Abweichungen und Audit-Log sind ohne 1Password und ohne Mensch prüfbar.
