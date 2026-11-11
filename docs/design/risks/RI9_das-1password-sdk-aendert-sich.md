---
render_macros: true
id: 'RI9'
titel: 'Das 1Password-SDK ändert sich oder verhält sich anders als die Doku'
bereich: 'Backends'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: offen
wahrscheinlichkeit: mittel
auswirkung: mittel
behandlung: [mindern]
register:
  owner: Stefan
---
# RI9 — Das 1Password-SDK ändert sich oder verhält sich anders als die Doku

{{ design_kopf(page) }}

**Beschreibung:** Das SDK hat die Hauptversion 0. Die Desktop-Integration hat offene Fehler. Der Entwurf ruht auf der Doku ({{ ref("G3") }}).

**Bewertung:** Eine Änderung der Schnittstelle ist bei einer Hauptversion 0 normal.

**Maßnahme:** Das Backend liegt hinter einer eigenen Schnittstelle ({{ ref("D38") }}). Service Account zuerst ({{ ref("D39") }}). Praktischer Test vor der Stufe 3 ({{ ref("T15") }}).

**Revisit:** nach dem praktischen Test.

**Quelle:** [Spike 1Password-Go-SDK](https://github.com/seletz/passierschein/issues/3).
