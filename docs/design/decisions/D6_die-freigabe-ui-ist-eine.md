---
render_macros: true
id: 'D6'
titel: 'Die Freigabe-UI ist eine eigene Swift-App ohne Zugriff auf Secrets'
bereich: 'Freigabe'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: gueltig
use_cases: [UC1]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Projektnotiz vom 2026-11-11
---
# D6 — Die Freigabe-UI ist eine eigene Swift-App ohne Zugriff auf Secrets

{{ design_kopf(page) }}

**Entscheidung:** Die Freigabe-UI ist eine Swift-App ohne Dock-Icon. Der Daemon startet die App. Der Daemon übergibt die Freigabeanfrage als JSON auf stdin. Die App gibt die Entscheidung als JSON auf stdout aus. Die App hat keinen Zugriff auf 1Password und erhält keinen Secret-Wert.

**Begründung:** Eine Freigabe braucht ein natives Fenster und Touch ID. Die App bleibt klein und prüfbar, weil sie keine Secrets kennt.

**Quelle:** Projektnotiz, Design v0.
