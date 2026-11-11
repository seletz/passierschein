---
title: Tasks
description: "Alle mise-Tasks des Repos und die festgelegten Werkzeugversionen."
tags:
  - Thema/Betrieb
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
| `tools:check` | Prüft die Werkzeugversionen und die Form der Workflows. |
| `check` | Führt alle Checks der CI aus. |
| `hooks:install` | Richtet den Git-Hook `pre-commit` ein. |

## Pull Requests

| Task | Zweck |
|---|---|
| `pr:screenshot` | Rendert die Ausgabe eines Befehls als PNG. |
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
| `docs:build` | Baut die Doku streng nach `site/`. |

## Release

| Task | Zweck |
|---|---|
| `release` | Erhöht die Version, legt den Tag an und schiebt ihn zu GitHub. |
| `release:snapshot` | Baut ein Release nach `dist/`, ohne etwas zu veröffentlichen. |
| `release:sbom:check` | Prüft die SBOMs und die Prüfsummen in `dist/`. |
| `release:publish` | Nur CI: baut und veröffentlicht das Release des Tags. |
| `release:secrets:check` | Nur CI: prüft, ob die Secrets für die Signatur gesetzt sind. |
| `release:signatures:check` | Prüft die Signatur der darwin-Binaries in `dist/`. |

## Werkzeugversionen

`mise.toml` legt jede Werkzeugversion fest:

- Unter `[tools]` stehen die Werkzeuge für alle Tasks: Go, uv, goreleaser,
  golangci-lint, govulncheck und gitleaks.
- Ein Werkzeug für genau einen Task steht an diesem Task unter `tools`.
- Die Version von Zensical steht in `pyproject.toml` und `uv.lock`.

Der Task `tools:check` prüft diese Regeln:

- Die Go-Version in `mise.toml` ist gleich der Go-Version in `go.mod`.
- Jeder Schritt `run:` eines Workflows ruft genau einen mise-Task auf.
- Jede Action ist auf einen Commit-Hash festgelegt. Die Version steht als
  Kommentar dahinter.

Die Swift-Version kommt in `mise.toml`, sobald das Repo Swift-Code enthält.
