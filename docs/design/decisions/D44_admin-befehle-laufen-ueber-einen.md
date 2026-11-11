---
render_macros: true
id: 'D44'
titel: 'Admin-Befehle laufen über einen eigenen Socket, den niemand in die VM weiterleitet'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 10
headless: true
status: gueltig
use_cases: [UC7]
beleg:
  notiz: Vorschlag des Designlaufs, zu bestätigen durch Stefan
  annahme: true
---
# D44 — Admin-Befehle laufen über einen eigenen Socket, den niemand in die VM weiterleitet

{{ design_kopf(page) }}

**Entscheidung:** Der Daemon stellt zwei Sockets bereit. Der Agent-Socket nimmt Anfragen der Agents an. Der Admin-Socket nimmt Widerruf und die Liste aller Grants an. Nur der Agent-Socket geht in die VM.

**Begründung:** Ein Agent in der VM kann keinen Grant ändern und keine fremden Grants sehen.

**Grenze:** Lokal erreicht ein Agent desselben Benutzers beide Sockets ({{ ref("RI5") }}).
