---
title: Tasks
description: "Alle mise-Tasks des Repos und die festgelegten Werkzeugversionen."
tags:
  - Thema/Betrieb
render_macros: true
---
# Tasks

Alle Aufgaben des Repos sind Tasks in `mise.toml`. Das Repo hat kein
Makefile. Die GitHub-Workflows rufen nur diese Tasks auf. Ein lokaler Lauf
prüft deshalb dasselbe wie die CI.

Aufruf: `mise run <task>`.

## Bauen und prüfen

| Task | Zweck |
|---|---|
| `build` | Baut das Binary `bin/a38`. |
| `fmt` | Prüft die Formatierung mit `gofmt`. |
| `vet` | Führt `go vet` aus. |
| `lint` | Führt golangci-lint aus. |
| `test` | Führt alle Tests mit dem Race Detector aus. |
| `vuln` | Sucht mit govulncheck nach bekannten Schwachstellen. |
| `secrets` | Durchsucht die Git-Historie mit gitleaks nach Secrets. |
| `secrets:staged` | Durchsucht die vorgemerkten Änderungen nach Secrets. |
| `deps:linux:check` | Prüft, dass der linux-Build kein Paket für 1Password enthält. Der Selbsttest zeigt mit dem darwin-Build, dass die Prüfung das Backend und das SDK findet. |
| `deps:extism:check` | Prüft, dass `third_party/extism-go-sdk` das Modul `extism/go-sdk` der verlangten Version mit der aufgezeichneten Prüfsumme ist, plus dem Patch für den Interpreter von wazero, und sonst nichts ({{ ref("D59") }}). |
| `tools:check` | Prüft die Werkzeugversionen und die Form der Workflows. |
| `check` | Führt alle Checks der CI aus. |
| `hooks:install` | Richtet den Git-Hook `pre-commit` ein. |

## Freigabe-App

Die Tasks laufen nur auf macOS mit Xcode.
Sie gehören nicht zu `check`, weil die CI auf Linux läuft.
Die Einzelheiten stehen in der [Referenz der Freigabe-App](85-freigabe-app.md).

| Task | Zweck |
|---|---|
| `app:build` | Baut die Freigabe-App „Passierschein“ als Debug und Release, legt das App-Bundle `app/.build/bundle/Passierschein.app` an und prüft, dass der Release-Build die Testschalter ablehnt. |
| `app:test` | Führt die Tests der Freigabe-App aus, ohne Fenster und ohne Touch ID. |
| `app:run:sample` | Zeigt ein Beispiel aus `app/Samples` im Debug-Build mit gefälschter Authentifizierung. |
| `app:e2e` | Baut den Debug-Build und startet `a38 serve --ui app` mit der App über echte Prozesse. Es öffnen sich zwei Fenster für je etwa zwei Sekunden. Touch ID erscheint nie. |

## 1Password

| Task | Zweck |
|---|---|
| `test:sdk-core` | Nur macOS, nicht Teil von `check`. Baut `a38`, signiert es ad hoc mit Hardened Runtime und ohne Entitlements und startet `a38 selftest sdk-core` ({{ ref("T30") }}). Braucht kein Token und kein Netz. Läuft im Workflow `macos.yml`. |
| `test:onepassword:ci` | Nur in der CI (Workflow `macos.yml`). Schreibt das Token aus `OP_SERVICE_ACCOUNT_TOKEN` in eine temporäre Datei mit `0600` und führt denselben Test aus. Scheitert, wenn das Secret fehlt ([CI](55-ci.md)). |
| `test:onepassword` | Nur macOS, nur von Hand, nicht Teil von `check`. Führt den Integrationstest des 1Password-Backends gegen den Vault `passierschein.dev` aus. Der Test liest nur. Er braucht `A38_OP_TOKEN_FILE` ([1Password-Backend einrichten](../how-to/30-1password-backend-einrichten.md#integrationstest)). |

## Demo

Die Tasks laufen nur auf macOS und nur von Hand.
Sie gehören nicht zu `check` und laufen nie in der CI.
Den Ablauf beschreibt [Demo vorführen](../how-to/50-demo-vorfuehren.md).
Ohne Variable gilt die echte Demo: Der Aufbau installiert `~/.local/bin/a38`, `~/Applications/Passierschein.app`, `~/.config/a38/config.yaml` und `~/develop/demo-projekt`.
Mit `DEMO_BACKEND=fake` bei jedem Task gilt die Probe in `/tmp/a38-probe`; sie berührt nichts anderes.

| Task | Zweck |
|---|---|
| `demo:setup` | Baut `a38` und die Freigabe-App und installiert die Demo. Überschreibt nie, was er nicht selbst installiert hat. Eine andere Konfiguration verschiebt er nur mit `DEMO_FORCE_CONFIG=1`. |
| `demo:preflight` | Prüft die Demo mit Zeilen `PASS` und `FAIL`. Startet nichts und liest kein Token. |
| `demo:start` | Führt die Prüfung aus und startet den Daemon der Demo im Vordergrund. In der Probe wählt `DEMO_UI` die Freigabe: `auto-approve`, `auto-deny` oder `debug-app`. |
| `demo:status` | Zeigt `a38 doctor`, die Grants, `a38 fault status`, die letzten Einträge des Audit-Logs und `a38 audit verify`. |
| `demo:request` | Schickt einen Akt aus `examples/demo/requests` an `a38 mcp`, ohne Agent. Die Ausgabe maskiert jeden Wert. Aufruf: `mise run demo:request -- akt2`. |
| `demo:fault` | Entzieht dem Verzeichnis des Audit-Logs das Schreibrecht. Die nächste Anfrage löst die globale Sperre aus. |
| `demo:fault:fix` | Stellt den alten Modus des Verzeichnisses wieder her und nennt den nächsten Schritt `a38 fault clear`. |
| `demo:teardown` | Entfernt, was `demo:setup` installiert hat. Behält Konfiguration und Audit-Log; `DEMO_PURGE=1` verschiebt die Konfiguration nach `.bak` und löscht `~/.local/state/a38`. |

## Pull Requests

| Task | Zweck |
|---|---|
| `pr:screenshot` | Rendert die Ausgabe eines Befehls als PNG. |
| `pr:screenshot:tui` | Startet ein Programm mit Oberfläche im Terminal (zum Beispiel `a38 tui`) in einem Pseudo-Terminal fester Größe, sendet Tasten und rendert den Bildschirm mit Farben als PNG (Skript `scripts/tui_screenshot.py`, Emulator `pyte`). `--theme light` nennt dem Programm einen hellen Hintergrund, `--truecolor` erlaubt 24-Bit-Farben. |
| `pr:screenshots:upload` | Legt Bilder auf dem Branch `review-assets` ab. |

## Design

| Task | Zweck |
|---|---|
| `design:check` | Prüft das Design-Set mit `dg-register check`. |

`design:check` läuft nur lokal und braucht das global installierte
`dg-register` (globale mise-Konfiguration). `dg-register` liegt in einem
privaten Repo einer anderen Organisation. Die CI dieses Repos hat darauf
keinen Zugriff.

## Doku

| Task | Zweck |
|---|---|
| `uv:sync` | Installiert die Werkzeuge der Doku nach `.venv`. |
| `docs:serve` | Zeigt die Doku lokal und lädt Änderungen neu. |
| `docs:build` | Baut die Doku streng nach `site/`, schreibt `llms.txt` und `llms-full.txt` und prüft beide ([CI](55-ci.md#ausgabe-fur-agenten-llmstxt)). |

## Release

Die Tasks mit „nur CI“ laufen nur in den Workflows `release.yml` und `signed-snapshot.yml`.
Den Ablauf beschreibt [Release](50-release.md).

| Task | Zweck |
|---|---|
| `release` | Legt interaktiv den nächsten Tag an und schiebt ihn zu GitHub. |
| `release:snapshot` | Baut Archive, SBOMs und Prüfsummen nach `dist/`, ohne Veröffentlichung. |
| `release:sbom:check` | Prüft je Archiv (tar.gz und App-ZIP) die SBOM und die Prüfsummendatei. |
| `release:publish` | Nur CI. goreleaser legt den Entwurf des Release an. |
| `release:secrets:check` | Nur CI. Scheitert und nennt jedes fehlende der fünf Secrets. |
| `release:signatures:check` | Prüft die Signatur der darwin-Binaries in `dist/` mit quill. |
| `release:app` | Nur macOS. Legt `build/release-app/Passierschein.app` aus dem universellen Release-Build der App und dem darwin-`a38` aus `dist/` an. Unsigniert. |
| `release:app:sign` | Nur macOS. Signiert `a38` im Bundle, dann das Bundle, mit der Developer ID aus einem temporären Schlüsselbund. `APP_SIGN_ADHOC=1` signiert ad-hoc, nie für einen Tag. |
| `release:app:notarize` | Nur CI auf macOS. Reicht das Bundle beim Notardienst ein, wartet, heftet das Ticket an und prüft es. |
| `release:app:check` | Nur macOS. Prüft Signatur, Bezeichner, Hardened Runtime, Developer-ID-Kette, Team-ID, Gatekeeper und Ticket. `APP_CHECK=adhoc` prüft ein ad-hoc signiertes Bundle. |
| `release:app:package` | Nur macOS. Packt das ZIP der App, prüft die ausgepackte Kopie, schreibt die SBOM und ergänzt die Prüfsummendatei. |
| `release:app:upload` | Nur CI. Lädt ZIP, SBOM und Prüfsummendatei in den Entwurf. |
| `release:assets:check` | Nur CI. Prüft, dass der Entwurf jede Datei hat und die Prüfsummendatei jede nennt. |
| `release:finalize` | Nur CI. Veröffentlicht den Entwurf. |

## Werkzeugversionen

`mise.toml` legt jede Werkzeugversion fest:

- Unter `[tools]` stehen die Werkzeuge für alle Tasks: Go, uv, goreleaser,
  golangci-lint, govulncheck und gitleaks.
- Ein Werkzeug für genau einen Task steht an diesem Task unter `tools`.
- Swift steht an jedem Task der Freigabe-App und an `release:app` unter `tools`, nicht unter
  `[tools]`. So installiert die CI auf Linux kein Swift.
- Die Version von Zensical steht in `pyproject.toml` und `uv.lock`.

Der Task `tools:check` prüft diese Regeln:

- Die Go-Version in `mise.toml` ist gleich der Go-Version in `go.mod`.
- Jeder Schritt `run:` eines Workflows ruft genau einen mise-Task auf.
- Jede Action ist auf einen Commit-Hash festgelegt. Die Version steht als
  Kommentar dahinter.
- Jeder Task `app:*` und der Task `release:app` legen dieselbe Swift-Version fest.
