---
render_macros: true
id: 'UC6'
titel: 'Der Mensch wertet Freigaben und Auslieferungen aus'
bereich: 'Audit'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
stufe: 3
---
# UC6 — Der Mensch wertet Freigaben und Auslieferungen aus

{{ design_kopf(page) }}

**Akteure:** Mensch.

**Auslöser:** Der Mensch will wissen, welche Agents welche Zugangsdaten erhalten haben.

**Ablauf:**

1. Der Mensch startet `a38 tui`.
2. Die TUI liest das Audit-Log nur lesend.
3. Der Mensch filtert nach Projekt, Agent, Zugangsdaten, Zeit und Ergebnis.
4. Die TUI zeigt Kennzahlen: Freigabeanfragen pro Tag, Anteil der Ablehnungen, Abweichungen ({{ ref("G12") }}).

**Ergebnis:** Der Mensch erkennt Muster und passt die Policy an.

**Hinweis:** Ab Stufe 1 gibt es `a38 audit list` als einfache Liste auf der Kommandozeile.
