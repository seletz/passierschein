---
title: Release
description: "Ablauf eines Release, Jobs, die 11 Dateien eines Release mit der App Passierschein, SBOM, Prüfsummen, Signatur und Notarisierung, Provenienz und Veröffentlichung der Doku."
render_macros: true
tags:
  - Thema/Release
---
# Release

## Ablauf

1. Ein Mensch führt `mise run release` auf dem Branch `develop` aus.
2. Der Task fragt nach der Art der Erhöhung: Patch, Minor oder Major.
3. Der Task legt den Tag `vX.Y.Z` an und schiebt ihn zu GitHub.
4. Der Workflow `release.yml` startet mit dem Tag.
5. Die Jobs bauen, signieren und prüfen alle Dateien in einen Entwurf des Release.
6. Der letzte Job veröffentlicht den Entwurf, wenn jede Datei da ist.

Das Release veröffentlicht die Doku nicht.
Die Doku erscheint nach jedem Push auf `develop` ([Doku auf GitHub Pages](#doku-auf-github-pages)).

Das erste Release ist `v0.1.0` (2026-11-12): <https://github.com/seletz/passierschein/releases/tag/v0.1.0>.
Die App ist mit der Developer ID signiert, notarisiert und hat ein angeheftetes Ticket.

Die Versionen folgen [SemVer](https://semver.org/lang/de/). Der Text des
Release entsteht aus den Commits seit dem letzten Tag. Die Commits folgen
[Conventional Commits](https://www.conventionalcommits.org/de/).

## Jobs des Release-Workflows

| Job | Runner | Inhalt |
|---|---|---|
| `release` | `ubuntu-latest` | `mise run check`, `release:secrets:check`, goreleaser (`release:publish`) mit quill für das darwin-`a38`, `release:sbom:check`, `release:signatures:check`. goreleaser legt das Release als Entwurf an. Die darwin-Archive, ihre SBOMs und die Prüfsummendatei gehen als Workflow-Artefakt an den nächsten Job. |
| `app` | `xcode-27` (macOS) | `release:app`, `release:app:sign`, `release:app:notarize`, `release:app:check`, `release:app:package`, `release:sbom:check`, `release:app:upload`. Das ZIP der App, seine SBOM und die erweiterte Prüfsummendatei kommen in den Entwurf. |
| `publish` | `ubuntu-latest` | `release:assets:check` prüft, dass der Entwurf jede Datei hat und die Prüfsummendatei jede Datei nennt. Danach folgen die Build-Provenienz und `release:finalize`. Erst dieser Task veröffentlicht den Entwurf. |

Gründe für diese Reihenfolge ({{ ref("D54") }}):

- Das Bundle enthält `a38` ({{ ref("D50") }}). Der macOS-Job braucht deshalb das `a38` aus goreleaser und läuft danach.
- Der Entwurf verhindert ein halb fertiges Release. Scheitert ein Job, bleibt der Entwurf unveröffentlicht. Ein neuer Lauf ersetzt den Entwurf (`replace_existing_draft`).
- goreleaser und quill bleiben auf Linux. Nur Bau, Signatur und Notarisierung der App kosten macOS-Minuten.

Fehlt eines der fünf Secrets, scheitert der Job `release` vor goreleaser.
Die Meldung nennt jedes fehlende Secret.
Pull Requests und `develop` bauen das Bundle im Workflow `macos.yml` mit einer Ad-hoc-Signatur.

## Artefakte

Ein Release enthält 11 Dateien: vier tar-Archive mit vier SBOMs (je eine je Plattform), das ZIP der App mit seiner SBOM und die Prüfsummendatei.
Die Tabelle zeigt die Muster:

| Datei | Inhalt |
|---|---|
| `passierschein-<tag>-<os>-<arch>.tar.gz` | Binary `a38`, `LICENSE`, `README.md` |
| `passierschein-<tag>-<os>-<arch>.cdx.json` | SBOM des Binarys |
| `passierschein-app-<tag>-darwin-universal.zip` | `Passierschein.app` mit der Freigabe-App und `a38`, signiert, notarisiert, mit angeheftetem Ticket |
| `passierschein-app-<tag>-darwin-universal.cdx.json` | SBOM des Bundles |
| `checksums-<tag>.txt` | SHA-256-Prüfsummen aller Archive und SBOMs |

Plattformen der tar-Archive: `darwin` und `linux`, je `amd64` und `arm64`.
Linux erhält nur `a38` ({{ ref("D53") }}).

## App im Release

Aufbau des Bundles:

```text
Passierschein.app/
  Contents/
    Info.plist
    MacOS/
      Passierschein   Freigabe-App, Universal-Binary (arm64, x86_64)
      a38             Go-Programm, Universal-Binary (arm64, x86_64)
    Resources/
      AppIcon.icns    App-Icon aus app/Icon/render-icon.swift
    _CodeSignature/
      CodeResources
    CodeResources     angeheftetes Ticket
```

- `Info.plist`: Bundle-ID `io.github.seletz.passierschein`, Version `X.Y.Z` aus dem Tag, Build-Nummer aus der Zahl der Commits, Icon `AppIcon`, `LSUIElement`, mindestens macOS 15.
- `AppIcon.icns` entsteht mit `scripts/app-icon.sh` aus der Quelle im Repo ([App-Icon](85-freigabe-app.md#app-icon)). Die Signatur des Bundles schließt die Datei ein. `release:app:check` prüft das Icon ({{ ref("T42") }}).
- `Passierschein` entsteht mit `swift build -c release --arch arm64 --arch x86_64`. Die tar-Archive liefern `a38` für beide Architekturen; ein Bundle für beide passt zu jedem Mac.
- `a38` entsteht mit `lipo -create` aus den beiden darwin-Binaries desselben goreleaser-Laufs. Der Task prüft die Prüfsummen der Archive, die Version und den Commit beider Binaries.
- Der Task `release:app:check` prüft auch, dass `Contents/MacOS` nur diese zwei Dateien enthält.

## SBOM

Eine SBOM (Software Bill of Materials) ist die Stückliste einer Software.
Jedes Archiv hat eine SBOM im Format CycloneDX ({{ ref("D18") }}).
`cyclonedx-gomod` erzeugt die SBOM der tar-Archive aus dem ausgelieferten Binary.
Die SBOM des App-ZIP nennt das Bundle als Hauptkomponente.
Ihre Komponenten sind die Freigabe-App und `a38` mit SHA-256 und die Go-Module von `a38`.
Die Freigabe-App hat keine fremden Swift-Pakete.
Der Task `release:sbom:check` bricht ab, wenn eine SBOM fehlt.

## Signatur und Notarisierung

Zwei Wege signieren mit derselben Developer ID und denselben fünf Secrets:

| Datei | Werkzeug | Runner | Ticket |
|---|---|---|---|
| `a38` in den darwin-tar-Archiven | quill in goreleaser (`notarize.macos`) | Linux | nicht anheftbar, macOS holt es online |
| `Passierschein.app` mit `a38` | `codesign`, `notarytool`, `stapler` | macOS | angeheftet |

Signatur des Bundles (`release:app:sign`):

1. Der Task dekodiert `MACOS_SIGN_P12` in eine Datei mit Modus `0600` unter `$RUNNER_TEMP`.
2. Er legt einen temporären Schlüsselbund mit einem Zufallspasswort an und importiert das Zertifikat. Nur `codesign` darf den Schlüssel nutzen. Der Schlüsselbund der Anmeldung bleibt unberührt.
3. Er signiert zuerst `Contents/MacOS/a38`, dann das Bundle. Beide mit `--options runtime`, `--timestamp` und festem Bezeichner: `io.github.seletz.passierschein.a38` und `io.github.seletz.passierschein`. Es gibt keine Berechtigungen (entitlements), also auch kein `get-task-allow`.
4. `codesign --verify --deep --strict` prüft das Ergebnis.
5. Ein `trap` löscht Schlüsselbund und Dateien und stellt die Suchliste der Schlüsselbunde wieder her.

`codesign` signiert `a38` im Bundle neu, auch wenn quill es schon signiert hat.
Gründe: ein Werkzeug und ein Bezeichner für das ganze Bundle, eine Signatur über beide Teile des Universal-Binarys, und derselbe Weg für die Ad-hoc-Signatur in Pull Requests.
Das `a38` im Bundle ist deshalb nicht byte-gleich mit dem `a38` im tar-Archiv.
Die SBOM des ZIP nennt den SHA-256 des Bundle-`a38`.

Notarisierung (`release:app:notarize`):

1. Der Task dekodiert `MACOS_NOTARY_KEY` in eine Datei mit Modus `0600`.
2. `ditto -c -k --keepParent` packt das Bundle.
3. `xcrun notarytool submit --wait --timeout 30m` reicht es ein und wartet.
4. Der Task gibt nur die Kennung der Einreichung, den Status und die Befunde des Protokolls aus.
5. `xcrun stapler staple` heftet das Ticket an, `xcrun stapler validate` prüft es.

`release:app:check` prüft danach: Developer-ID-Kette bis `Apple Root CA`, Hardened Runtime, Zeitstempel, dieselbe Team-ID für App und `a38`, die Anforderung „Developer ID des Teams“, das Urteil `source=Notarized Developer ID` von `spctl -t exec` und das Ticket.
`release:app:package` packt das ZIP, packt es zur Probe aus und prüft die Kopie noch einmal.

Kein Skript nutzt `set -x`.
Die Secrets gehen nur über Umgebungsvariablen in den Schritt, der sie braucht.

Secrets einrichten, mit dem signierten Snapshot testen und ein Release prüfen: Anleitung [Release signieren und prüfen](../how-to/60-release-signieren-und-pruefen.md).

## Signierter Snapshot

Der Workflow `signed-snapshot.yml` startet nur von Hand.
Er veröffentlicht nichts und hat nur Leserechte.

- Job `snapshot` (Linux): `release:secrets:check`, ein signierter goreleaser-Snapshot, `release:signatures:check`.
- Job `app` (macOS): dieselben Schritte wie der Job `app` des Release, ohne Upload in ein Release.

Die Ergebnisse hängen sieben Tage als Workflow-Artefakte am Lauf: `signed-snapshot-darwin` und `signed-snapshot-app`.

## Prüfung im Daemon

Ist `a38` selbst mit einer Developer ID signiert, prüft `a38 serve` die Signatur der App vor jedem Start.
Die App muss die Developer ID desselben Teams und die Hardened Runtime tragen.
Ist `a38` ad-hoc signiert oder unsigniert, ist die Prüfung aus.
Einzelheiten: [Freigabe-App](85-freigabe-app.md#signatur-der-app).

## Build-Provenienz

Der Workflow erzeugt eine Build-Provenienz für jede Datei aus der
Prüfsummendatei. Die Provenienz ist ein GitHub-Attest nach SLSA (Supply-chain
Levels for Software Artifacts).

GitHub bietet Atteste für private Repos nur in GitHub Enterprise Cloud an.
Der Schritt läuft deshalb nur, wenn das Repo öffentlich ist.

Eine Signatur der Prüfsummendatei mit cosign gibt es nicht: Eine schlüssellose Signatur schreibt den Namen des Repos in ein öffentliches Log.
Die Developer ID signiert die Programme ([Grundgerüst, PR 6](https://github.com/seletz/passierschein/pull/6), Punkte 3 und 4).

## Doku auf GitHub Pages

Die Doku steht unter <https://seletz.github.io/passierschein/>.

Nur der Workflow `pages.yml` veröffentlicht sie, nach jedem Push auf `develop` (Stefan, 2026-11-12).
Der Release-Workflow veröffentlicht die Doku nicht.
Die Umgebung `github-pages` erlaubt Veröffentlichungen nur vom Branch `develop`, nicht von einem Tag.
Der erste Versuch im Release-Lauf von `v0.1.0` scheiterte daran.

Online ist der Stand von `develop`.
Die Startseite nennt die Version, das Build-Datum und den Commit.
Nach einem Release steht dort zum Beispiel `0.1.0-3-g1a2b3c4`: das letzte Release `0.1.0`, drei Commits danach.

Voraussetzung: GitHub Pages ist im Repo aktiviert, mit der Quelle „GitHub
Actions“. Eine Site auf GitHub Pages ist öffentlich, auch wenn das Repo
privat ist.

## Abhängigkeiten aktualisieren (Renovate)

[Renovate](https://docs.renovatebot.com/) hält Go-Module, GitHub Actions und die Werkzeuge in `mise.toml` aktuell.
Er öffnet dafür Pull Requests gegen `develop`.
Er läuft selbst gehostet im Workflow `renovate.yml`.
Die Regeln stehen in `.github/renovate.json5`.

- **Takt:** alle 8 Stunden (cron `17 */8 * * *`) und von Hand über *Actions → Renovate → Run workflow*. Zwei Läufe überschneiden sich nie (`concurrency: renovate`). Es gibt keine Zeitfenster. Sicherheits-Updates (GitHub-Advisories und OSV) warten nie und zählen nicht gegen die Limits.
- **Limits:** höchstens 2 neue PRs pro Stunde und 5 offene PRs gleichzeitig.
- **Gruppen:** Die Charm-Module (`charm.land/*`, `github.com/charmbracelet/*`) kommen in einem gemeinsamen PR. Go in `go.mod` und in `mise.toml` kommt ebenfalls in einem PR (`tools:check` verlangt dieselbe Version).
- **Major-Updates:** Renovate öffnet sie erst, wenn sie im Dependency-Dashboard (einem Issue im Repository) angehakt sind. Das gilt auch für eine neue Go-Minor-Version.
- **Commits und Label:** Commits heißen `chore(deps): …`, die PRs tragen das Label `dependencies`.
- **Was Renovate liest:** `go.mod`, die GitHub Actions in `.github/workflows/` (Renovate hält den Commit-Hash und den Versionskommentar `# vX.Y.Z` zusammen), `[tools]` in `mise.toml`, die Zeile `# renovate: …` über einem `tools = { … }` in den Tasks und die Pins `uvx --with playwright==…` und `pyte==…`. Von einer Tabelle `tools = { … }` liest er nur das erste Werkzeug. Das Swift-Paket unter `app/` hat keine Abhängigkeiten; Renovate hat dort nichts zu tun. Die Swift-Version steht in `mise.toml`.
- **Trockenlauf:** Beim manuellen Start `dry_run` setzen (bei Bedarf `log_level` auf `debug`). Renovate protokolliert dann, was er täte, und legt keine Branches oder PRs an. Der erste Lauf nach dem Merge ist ein solcher Trockenlauf.
- **Secret `RENOVATE_TOKEN`:** ein fein granulares Personal Access Token mit Lese- und Schreibrecht auf *Contents*, *Pull requests*, *Issues* und *Workflows*. Mit dem `GITHUB_TOKEN` ginge es nicht: PRs, die damit entstehen, lösen keine weiteren Workflows aus (die CI liefe auf den Update-PRs nie), und er darf keine Dateien unter `.github/workflows` ändern. Läuft das Token ab, schlägt der Workflow fehl. Dann ein neues Token anlegen und das Secret ersetzen.
- **Lokal prüfen:** `mise x npm:renovate@44.239.1 -- renovate-config-validator .github/renovate.json5` prüft die Konfiguration.

## Installation

Die App installierst du aus dem ZIP nach `/Applications`.
`a38` legst du als symbolischen Link aus dem Bundle in den `PATH`.
Der Daemon findet die App dann im eigenen Bundle ({{ ref("D50") }}).
Die Schritte, die Prüfung, das Token und das Update stehen in [a38 aus dem Release installieren](../how-to/15-a38-aus-dem-release-installieren.md).

Für Linux und für `a38` allein gibt es das tar-Archiv.
`mise` mit dem Backend `github:seletz/passierschein` wählt auf macOS das tar-Archiv, nicht das ZIP der App (getestet am 2026-11-12).
Homebrew folgt, wenn das Repo öffentlich ist.
