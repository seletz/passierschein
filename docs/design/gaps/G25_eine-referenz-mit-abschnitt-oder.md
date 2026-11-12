---
render_macros: true
id: 'G25'
titel: 'Eine Referenz mit Abschnitt oder Attribut lässt sich nicht ausdrücken'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-11'
order: 2
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G25 — Eine Referenz mit Abschnitt oder Attribut lässt sich nicht ausdrücken

{{ design_kopf(page) }}

{{ ref("D27") }} legt die Referenz auf drei Teile fest: Vault, Item und Feld. 1Password kennt Referenzen mit einem Abschnitt und mit einem Attribut. Solche Referenzen lassen sich nicht ausdrücken.

Das betrifft die Stufe 3 ({{ ref("F12") }}). In den Stufen 1 und 2 reicht die Form mit drei Teilen.

Offene Frage:

- Braucht das echte Backend Abschnitte und Attribute, und wie sieht die Referenz dann aus?

Stand 2026-11-12: {{ ref("D52") }} legt für das 1Password-Backend fest: Ein Feld darf in einem Abschnitt liegen, wenn seine Bezeichnung im Item eindeutig ist. Eine Referenz mit Attribut endet mit `not_found`. {{ ref("D52") }} ist eine Annahme der Entwicklung. Offen bleiben die Bestätigung durch Stefan und die Frage, ob ein Agent Attribute wie `otp` braucht.

**Quelle:** Entwicklung von Stufe 1, Issue 16 (Architektur und Paketgerüst), 2026-11-11.
