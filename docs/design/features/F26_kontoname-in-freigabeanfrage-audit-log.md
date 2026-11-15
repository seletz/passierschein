---
render_macros: true
id: 'F26'
titel: 'Kontoname in Freigabeanfrage, Audit-Log und a38 tui'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-15'
order: 6
headless: true
status: aktiv
stufe: 3
umsetzung: zu-bauen
implementiert: [D74]
use_cases: [UC1, UC6]
---
# F26 — Kontoname in Freigabeanfrage, Audit-Log und a38 tui

{{ design_kopf(page) }}

Die Freigabeanfrage zeigt je Eintrag das Konto vor dem Vault. Das Audit-Log speichert das Konto in jedem Ereignis mit einem Vault, als Teil der Hash-Kette. `a38 tui` zeigt eine Spalte „Konto“. Das braucht ein neues Feld in der Nachricht an die Freigabe-App, eine Änderung der Swift-App und ein neues Feld im Schema des Audit-Logs.

Setzt auf: Freigabe-App ({{ ref("F11") }}), Audit-Log ({{ ref("F7") }}), `a38 tui` ({{ ref("F13") }}), mehrere Konten ({{ ref("F25") }}).

Setzt um: {{ ref("D74") }}.
