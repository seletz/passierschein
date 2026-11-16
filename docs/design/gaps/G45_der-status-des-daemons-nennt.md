---
render_macros: true
id: 'G45'
titel: 'Der Status des Daemons nennt den Zustand des 1Password-Backends nicht je Konto'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-16'
order: 5
headless: true
type: Gap
kind: design
status: geschlossen
beleg:
  datum: '2026-11-16'
  durch: [D78]
  quelle: ['#119']
  notiz: D78 wartet auf die Bestätigung durch Stefan
  annahme: true
register:
  owner: Stefan
---
# G45 — Der Status des Daemons nennt den Zustand des 1Password-Backends nicht je Konto

{{ design_kopf(page) }}

Der Daemon meldet am Admin-Socket einen Zustand für das ganze Backend ({{ ref("F20") }}). Mit mehreren Konten ({{ ref("D73") }}) gilt: Antwortet ein Konto, meldet das Backend `ok`. Lehnt 1Password das Token eines anderen Kontos ab, sieht `a38 doctor` das nicht. Nur das Log des Daemons nennt das Konto. `a38 doctor` prüft je Konto nur, ob die Quelle ein Token hat, nicht, ob 1Password es annimmt.

Ein Zustand je Konto im Status braucht ein neues Feld in der Nachricht `status`. Die Nachricht gehört zu `internal/protocol`; [Issue 88](https://github.com/seletz/passierschein/issues/88) ändert das Protokoll nicht.

Offene Fragen:

- Soll der Status je Konto Zustand und Grund nennen, und soll `a38 doctor` daraus je Konto einen Befund machen?
- Soll ein gesperrtes Konto den Zustand des ganzen Backends auf „gesperrt“ setzen, oder bleibt „ok“, solange ein Konto antwortet?

**Vorschlag:** ein Feld `accounts` im Status mit Name, Zustand und Grund; `a38 doctor` zeigt je Konto einen Befund. Der Zustand des ganzen Backends bleibt „ok“, solange ein Konto antwortet.

**Quelle:** [Issue 88](https://github.com/seletz/passierschein/issues/88), Umsetzung.
