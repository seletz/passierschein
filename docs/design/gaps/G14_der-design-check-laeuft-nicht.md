---
render_macros: true
id: 'G14'
titel: 'Der Design-Check läuft nicht in der CI'
bereich: 'Arbeitsweise'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G14 — Der Design-Check läuft nicht in der CI

{{ design_kopf(page) }}

`dg-register` liegt in einem privaten Repo einer anderen Organisation. Die CI dieses Repos hat keinen Zugriff. `mise run design:check` läuft nur lokal.

Offene Fragen:

- Bekommt das Repo ein Token für den Download?

**Quelle:** Grundgerüst, [Issue 1](https://github.com/seletz/passierschein/issues/1).
