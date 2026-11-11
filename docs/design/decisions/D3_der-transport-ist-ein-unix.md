---
render_macros: true
id: 'D3'
titel: 'Der Transport ist ein Unix-Socket, für die VM derselbe Socket über SSH-RemoteForward'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 3
headless: true
status: gueltig
use_cases: [UC1, UC13]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Projektnotiz vom 2026-11-11
---
# D3 — Der Transport ist ein Unix-Socket, für die VM derselbe Socket über SSH-RemoteForward

{{ design_kopf(page) }}

**Entscheidung:** Der Daemon stellt einen Unix-Socket bereit. Ein lokaler Agent nutzt den Socket direkt. Für die VM leitet SSH denselben Socket mit `RemoteForward` weiter.

**Begründung:** Die VM ist damit nur eine Frage des Transports. Der Daemon braucht keinen Netzwerk-Port und kein gemeinsames Geheimnis.

**Quelle:** Projektnotiz, Design v0 und Stufenplan.
