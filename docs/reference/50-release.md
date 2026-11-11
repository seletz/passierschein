---
title: Release
description: "Ablauf eines Release, Artefakte, SBOM, Prüfsummen, Signatur, Provenienz und Veröffentlichung der Doku."
tags:
  - Thema/Release
---
# Release

## Ablauf

1. Ein Mensch führt `mise run release` auf dem Branch `develop` aus.
2. Der Task fragt nach der Art der Erhöhung: Patch, Minor oder Major.
3. Der Task legt den Tag `vX.Y.Z` an und schiebt ihn zu GitHub.
4. Der Workflow `release.yml` startet mit dem Tag.
5. Der Workflow führt alle Checks aus.
6. goreleaser baut die Artefakte und legt das GitHub-Release an.
7. Der Workflow baut die Doku des Tags und veröffentlicht sie auf GitHub
   Pages.

Die Versionen folgen [SemVer](https://semver.org/lang/de/). Der Text des
Release entsteht aus den Commits seit dem letzten Tag. Die Commits folgen
[Conventional Commits](https://www.conventionalcommits.org/de/).

## Artefakte

Ein Release enthält für jede Plattform diese Dateien:

| Datei | Inhalt |
|---|---|
| `passierschein-<tag>-<os>-<arch>.tar.gz` | Binary `a38`, `LICENSE`, `README.md` |
| `passierschein-<tag>-<os>-<arch>.cdx.json` | SBOM des Binarys |
| `checksums-<tag>.txt` | SHA-256-Prüfsummen aller Archive und SBOMs |

Plattformen: `darwin` und `linux`, je `amd64` und `arm64`.

## SBOM

Eine SBOM (Software Bill of Materials) ist die Stückliste einer Software.
Jedes Release hat je Plattform eine SBOM im Format CycloneDX.
`cyclonedx-gomod` erzeugt die SBOM aus dem ausgelieferten Binary. Der Task
`release:sbom:check` bricht ab, wenn eine SBOM fehlt.

## Signatur und Notarisierung

goreleaser signiert die darwin-Binaries mit einer Developer ID und lässt sie
von Apple notarisieren. goreleaser nutzt dafür quill. Der Schritt läuft auf
dem Linux-Runner.

Der Schritt braucht diese Secrets im Repo:

| Secret | Inhalt |
|---|---|
| `MACOS_SIGN_P12` | Zertifikat „Developer ID Application“ als P12, Base64-kodiert |
| `MACOS_SIGN_PASSWORD` | Passwort der P12-Datei |
| `MACOS_NOTARY_ISSUER_ID` | Issuer-ID des App-Store-Connect-Schlüssels |
| `MACOS_NOTARY_KEY_ID` | Key-ID des App-Store-Connect-Schlüssels |
| `MACOS_NOTARY_KEY` | Privater Schlüssel (`.p8`), Base64-kodiert |

Ohne `MACOS_SIGN_P12` liefert das Release unsignierte darwin-Binaries.

Der Workflow `signed-snapshot.yml` testet die Signatur ohne Release. Ein
Mensch startet den Workflow von Hand. Der Workflow veröffentlicht nichts.

Die Signatur der Swift-App ist offen. Ein Spike klärt sie.

## Build-Provenienz

Der Workflow erzeugt eine Build-Provenienz für jede Datei aus der
Prüfsummendatei. Die Provenienz ist ein GitHub-Attest nach SLSA (Supply-chain
Levels for Software Artifacts).

GitHub bietet Atteste für private Repos nur in GitHub Enterprise Cloud an.
Der Schritt läuft deshalb nur, wenn das Repo öffentlich ist.

## Doku auf GitHub Pages

Der Workflow veröffentlicht die Doku bei jedem Release. Die Startseite zeigt
die Version, das Build-Datum und den Commit-Hash.

Voraussetzung: GitHub Pages ist im Repo aktiviert, mit der Quelle „GitHub
Actions“. Eine Site auf GitHub Pages ist öffentlich, auch wenn das Repo
privat ist.

## Installation

Die Verteilung läuft zunächst über `mise` mit dem Backend `github:`. Das
Backend lädt die Artefakte des GitHub-Release. Homebrew folgt, wenn das Repo
öffentlich ist.
