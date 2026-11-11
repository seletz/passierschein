---
render_macros: true
id: 'G23'
titel: 'Wo eine Referenz in anderer Schreibweise angeglichen wird, ist offen'
bereich: 'Anfrage und Bündelung'
lauf: '2.0'
stand: '2026-11-11'
order: 3
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G23 — Wo eine Referenz in anderer Schreibweise angeglichen wird, ist offen

{{ design_kopf(page) }}

{{ ref("T24") }} erwartet: Der Agent schreibt eine Referenz in anderer Schreibweise, und die Freigabeanfrage zeigt die Namen des Backends ({{ ref("D27") }}). Die Policy prüft den Vault aber vor dem Backend, und {{ ref("D12") }} verbietet einen Vergleich nach Muster.

**Annahme der Entwicklung:** Der Name des Vaults muss genau dem Namen in der Konfiguration entsprechen. Ein Vault in anderer Schreibweise endet mit `not_allowed`. Das Backend gleicht nur Item und Feld an.

Offene Fragen:

- Gilt die andere Schreibweise nur für Item und Feld?
- Darf ein Agent ein Item über seine Kennung statt über seinen Namen nennen?

**Quelle:** Entwicklung von Stufe 1, Issue 16 (Architektur und Paketgerüst), 2026-11-11.
