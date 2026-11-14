---
title: Doku schreiben und bauen
description: "Eine Seite anlegen, Tags und Makros nutzen, die Doku lokal ansehen und streng bauen."
---
# Doku schreiben und bauen

Die Doku ist eine [Zensical](https://zensical.org/)-Site. Die Quellen liegen
im Ordner `docs/`.

## Seite anlegen

1. Wähle den Abschnitt nach der Art der Seite:

    | Art | Ordner |
    |---|---|
    | Tutorial | `docs/tutorials/` |
    | Anleitung | `docs/how-to/` |
    | Referenz | `docs/reference/` |
    | Erklärung | `docs/explanation/` |
    | Spike | `docs/explanation/spikes/` |

2. Lege eine Markdown-Datei an. Ein Zahlenpräfix im Dateinamen legt die
   Reihenfolge fest, zum Beispiel `40-tasks.md`.
3. Setze im Front Matter `title` und `description`.
4. Verlinke die Seite auf der `index.md` des Ordners.

Die Seitenleiste entsteht aus den Ordnern. Die Datei `zensical.toml` enthält
keine Navigation.

## Tags

Jeder Ordner gibt seinen Seiten Tags über die Datei `.meta.yml`. Eine Seite
braucht deshalb keine eigenen Tags für ihre Art. Themen-Tags setzt die Seite
selbst im Front Matter unter `tags`. Die Liste der Tags steht im
[Tag-Index](../tags/index.md).

## Makros

Eine Seite mit `render_macros: true` im Front Matter darf Makros nutzen. Die
Makros stehen in `docs_macros.py`.

| Makro | Ergebnis |
|---|---|
| `version_banner()` | Version, Build-Datum und Commit-Hash der Doku |
| `ref("D1")` | Link auf ein Design-Artefakt |
| `design_kopf(page)` | Kopfzeile eines Design-Artefakts |
| `design_index(page)` | Tabellen einer Familie des Design-Sets |

## Doku ansehen und bauen

```sh
mise run docs:serve
```

Der Task startet einen lokalen Server und lädt Änderungen neu.

```sh
mise run docs:build
```

Der Task baut die Site streng nach `site/`. Ein toter Link bricht den Build
ab. Die CI führt denselben Task aus.
Der Task schreibt auch `llms.txt` und `llms-full.txt` für Agenten und prüft sie.
Eine neue Seite erscheint dort automatisch, wenn sie in einem der Ordner liegt, die `zensical.toml` unter `[project.plugins.llmstxt.sections]` nennt.

## Sprache

Die Doku ist deutsch und folgt DIN 8579. Die Regeln stehen in den
[Konventionen](../reference/60-konventionen.md).
