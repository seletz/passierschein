---
render_macros: true
id: 'D9'
titel: 'Ein dev-Secret geht als MCP-Antwort an den Agent'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: gueltig
use_cases: [UC1, UC2]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Projektnotiz vom 2026-11-11
---
# D9 — Ein dev-Secret geht als MCP-Antwort an den Agent

{{ design_kopf(page) }}

**Entscheidung:** Ein Secret aus einem dev-Vault geht nach der Freigabe als MCP-Antwort an den Agent. Der Agent darf den Wert kennen. Ein Leck ist akzeptiert.

**Begründung:** Ein dev-Vault enthält nur Zugangsdaten der Entwicklungsumgebung. Sie sind jederzeit rotierbar. Der Vorteil gegenüber einer Datei im Repo: rotierbar, sperrbar, nicht in der Git-Historie, jeder Zugriff protokolliert.

**Folge:** Die kritische Prüfung empfiehlt das Gegenteil. Stefan hat danach bewusst so entschieden ({{ ref("RI1") }}).

**Quelle:** Projektnotiz, Secrets-Modell.
