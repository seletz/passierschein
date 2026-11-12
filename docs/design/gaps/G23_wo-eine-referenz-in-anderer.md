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

Stand 2026-11-12: {{ ref("D52") }} beantwortet beide Fragen für das 1Password-Backend: Das Backend vergleicht Vault, Item und Feld genau, ohne Angleichung, und das Segment des Items ist nie eine Kennung. {{ ref("D52") }} ist eine Annahme der Entwicklung. Offen bleibt die Bestätigung durch Stefan. Damit gibt es auch für Item und Feld keine andere Schreibweise mehr; der Fall aus {{ ref("T24") }} tritt mit dem echten Backend nicht auf.

**Quelle:** Entwicklung von Stufe 1, Issue 16 (Architektur und Paketgerüst), 2026-11-11.
