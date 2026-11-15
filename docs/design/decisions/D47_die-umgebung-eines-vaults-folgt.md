---
render_macros: true
id: 'D47'
titel: 'Die Konfiguration nennt die Umgebung jedes Vaults'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-15'
order: 4
headless: true
status: gueltig
use_cases: [UC15, UC1]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Neufassung, von Stefan im Review am 2026-11-11 abgenommen
---
# D47 — Die Konfiguration nennt die Umgebung jedes Vaults

{{ design_kopf(page) }}

**Entscheidung:** Die Konfiguration nennt zu jedem Vault seine Umgebung: dev, test oder prod. Der Daemon leitet die Umgebung nie aus dem Namen des Vaults ab. Ein Vault ohne Angabe der Umgebung ist ein Fehler der Konfiguration. Der Daemon startet dann nicht.

**Begründung:** Die Zuordnung ist eine Policy des Menschen. Ein Vergleich von Zeichenketten kann einen Vault falsch einordnen. Eine fehlende Angabe darf nie zu dev führen.

**Verlauf:** Die erste Fassung leitete die Umgebung aus einem Segment des Namens ab. Stefan hat sie am 2026-11-11 abgelehnt und diese Fassung abgenommen.
