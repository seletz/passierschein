---
render_macros: true
id: 'D46'
titel: 'Die Freigabe-App ist ein App-Bundle, das ein macOS-Runner signiert und notarisiert'
bereich: 'Betrieb und Release'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: gueltig
use_cases: [UC11]
beleg:
  notiz: Vorschlag des Designlaufs, zu bestätigen durch Stefan
  annahme: true
---
# D46 — Die Freigabe-App ist ein App-Bundle, das ein macOS-Runner signiert und notarisiert

{{ design_kopf(page) }}

**Entscheidung:** Die Freigabe-App ist ein echtes `.app`-Bundle aus einem Swift-Paket, ohne Xcode-Projekt. Ein Job auf einem macOS-Runner signiert das Bundle mit `codesign`, lässt es mit `notarytool` notarisieren und heftet das Ticket mit `stapler` an. Der Job übergibt das Bundle an den goreleaser-Job auf Linux. goreleaser packt es in die darwin-Archive neben `a38`.

**Begründung:** quill signiert in der veröffentlichten Version nur einzelne Binaries. Ein einzelnes Binary mit eingebetteter `Info.plist` hat für macOS keine Bundle-ID. Die vorhandenen fünf Secrets reichen aus.

**Grenze:** Nichts davon ist mit der echten Developer ID getestet ({{ ref("G5") }}).

**Quelle:** [Spike Signatur der Swift-App](https://github.com/seletz/passierschein/issues/5).
