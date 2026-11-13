---
render_macros: true
id: 'G15'
titel: 'Pages, Build-Provenienz und cosign passen nicht ohne Weiteres zu einem privaten Repo'
bereich: 'Betrieb und Release'
lauf: '2.0'
stand: '2026-11-12'
order: 2
headless: true
type: Gap
kind: design
status: geschlossen
beleg:
  wer: Stefan
  datum: '2026-11-12'
  quelle: ['#65']
  notiz: Pages öffentlich von develop (PR 65); Provenienz nur bei öffentlichem Repo; keine Signatur der Prüfsummendatei mit cosign. Alle drei Fragen beantwortet.
register:
  owner: Stefan
---
# G15 — Pages, Build-Provenienz und cosign passen nicht ohne Weiteres zu einem privaten Repo

{{ design_kopf(page) }}

Offene Fragen:

- Eine Site auf GitHub Pages ist öffentlich. Soll die Doku eines privaten Repos öffentlich sein?
- GitHub bietet Atteste für private Repos nur in GitHub Enterprise Cloud an. Reicht die Provenienz ab dem Zeitpunkt, an dem das Repo öffentlich ist?
- Eine schlüssellose Signatur mit cosign schreibt den Namen des Repos in ein öffentliches Log. Soll es eine Signatur der Prüfsummendatei geben?

**Quelle:** Grundgerüst, [Pull Request 6](https://github.com/seletz/passierschein/pull/6).
