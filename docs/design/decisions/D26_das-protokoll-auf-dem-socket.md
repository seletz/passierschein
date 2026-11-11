---
render_macros: true
id: 'D26'
titel: 'Das Protokoll auf dem Socket hat ein festes, versioniertes Schema'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 8
headless: true
status: gueltig
use_cases: [UC1, UC14]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Vorschlag des Designlaufs, von Stefan im Review am 2026-11-11 abgenommen
---
# D26 — Das Protokoll auf dem Socket hat ein festes, versioniertes Schema

{{ design_kopf(page) }}

**Entscheidung:** Jede Nachricht ist ein JSON-Objekt auf einer Zeile mit einer Versionsnummer. Der Daemon kennt eine feste Menge von Nachrichtentypen. Der Daemon lehnt unbekannte Typen, unbekannte Felder, eine falsche Version und zu große Nachrichten ab. Es gibt kein generisches RPC.

**Begründung:** Der Daemon nimmt Eingaben aus einer nicht vertrauten Zone an und hält den Zugriff auf 1Password ({{ ref("RI4") }}). Eine kleine, feste Oberfläche ist prüfbar.
