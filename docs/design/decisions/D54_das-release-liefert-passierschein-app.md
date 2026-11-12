---
render_macros: true
id: 'D54'
titel: 'Das Release liefert Passierschein.app mit a38 als eigenes ZIP, gebaut nach goreleaser'
bereich: 'Betrieb und Release'
lauf: '2.0'
stand: '2026-11-12'
order: 5
headless: true
status: gueltig
use_cases: [UC11]
beleg:
  notiz: Vorschlag aus der Entwicklung von Issue 68, zu bestätigen durch Stefan; die Signatur ist erst im signierten Snapshot erprobt
  annahme: true
---
# D54 — Das Release liefert Passierschein.app mit a38 als eigenes ZIP, gebaut nach goreleaser

{{ design_kopf(page) }}

**Entscheidung:**

1. Das Release enthält für macOS ein eigenes Archiv `passierschein-app-<tag>-darwin-universal.zip` mit `Passierschein.app`. Das Bundle enthält die Freigabe-App und `a38` als `Contents/MacOS/a38` ({{ ref("D50") }}). Beide Programme sind Universal-Binaries (arm64 und x86_64).
2. `a38` im Bundle ist das darwin-Binary desselben goreleaser-Laufs. Der Job prüft Prüfsumme, Version und Commit.
3. Der macOS-Job läuft nach goreleaser. goreleaser legt das Release als Entwurf an. Der macOS-Job signiert das Bundle von innen nach außen mit `codesign`, lässt es notarisieren, heftet das Ticket an und lädt das ZIP, seine SBOM und die erweiterte Prüfsummendatei in den Entwurf. Ein letzter Job prüft die Dateien und veröffentlicht den Entwurf.
4. `codesign` signiert `a38` im Bundle neu, mit dem Bezeichner `io.github.seletz.passierschein.a38`. Die darwin-Archive behalten das einzelne `a38` mit der Signatur von quill.
5. Ein Release ohne die fünf Secrets scheitert. Pull Requests und `develop` bauen das Bundle mit einer Ad-hoc-Signatur.

**Abweichung von {{ ref("D46") }}:** {{ ref("D46") }} legt das Bundle neben `a38` in die darwin-Archive und lässt den macOS-Job vor goreleaser laufen. Mit `a38` im Bundle braucht der macOS-Job aber das `a38` aus goreleaser. Die Reihenfolge kehrt sich deshalb um. Ein Bundle in den tar-Archiven verdoppelte `a38` und brauchte zwei Archive je Architektur. Signatur mit `codesign`, Notarisierung und Ticket bleiben wie in {{ ref("D46") }}.

**Begründung:** Das ZIP mit `ditto` ist der Weg von Apple und erhält Signatur und Ticket. Ein Bundle für beide Architekturen ist ein Download für jeden Mac. Der Entwurf verhindert ein halb fertiges Release. Eine neue Signatur von `a38` mit `codesign` gibt einen Weg für Ad-hoc und Developer ID, einen festen Bezeichner und eine Signatur über beide Teile des Universal-Binarys.

**Grenze:** Der erste Lauf mit der echten Developer ID ist offen ({{ ref("G5") }}). Das einzelne `a38` aus dem tar-Archiv und `a38` im Bundle haben verschiedene Bezeichner.

**Quelle:** [Issue 68](https://github.com/seletz/passierschein/issues/68), Vorschlag aus der Entwicklung.
