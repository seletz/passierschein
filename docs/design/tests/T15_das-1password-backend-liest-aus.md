---
render_macros: true
id: 'T15'
titel: 'Das 1Password-Backend liest aus einem Test-Vault und meldet den gesperrten Zustand'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-12'
order: 2
headless: true
status: aktiv
umsetzung: teilweise
beweist: [F12, D39]
---
# T15 — Das 1Password-Backend liest aus einem Test-Vault und meldet den gesperrten Zustand

{{ design_kopf(page) }}

**Form:** Manueller Test nach Rücksprache mit Stefan. Kein Lauf in der CI.

**Schritte:** Ein Test-Vault mit neutralen Einträgen. Der Daemon löst Namen auf und liest ein Feld. Danach derselbe Ablauf mit einem ungültigen Zugang.

**Erwartung:** Namen und Wert stimmen. Der ungültige Zugang endet mit `backend_unavailable` oder `backend_locked`. Ein Vault außerhalb der Rechte ist nicht sichtbar.


Belegt: {{ ref("F12") }}, {{ ref("D39") }}.
