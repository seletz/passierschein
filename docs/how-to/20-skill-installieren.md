---
title: Skill für Agents installieren
description: "Den Skill von a38 aus dem Binary ausgeben und für Claude Code ablegen."
---
# Skill für Agents installieren

Der Skill erklärt einem Agent, wie er Zugangsdaten über a38 anfragt.
Er steckt im Binary. Er passt deshalb immer zur installierten Version.

## Voraussetzungen

- `a38` ist installiert.
- Der Agent ist Claude Code.

## Skill ablegen

1. Lege den Ordner für den Skill an:

    ```sh
    mkdir -p ~/.claude/skills/a38
    ```

2. Schreibe den Skill in die Datei `SKILL.md`:

    ```sh
    a38 llm --skill > ~/.claude/skills/a38/SKILL.md
    ```

3. Starte die Sitzung des Agents neu.

Für ein einzelnes Projekt legst du die Datei nach `.claude/skills/a38/SKILL.md`
im Projekt ab.

## Skill aktualisieren

Führe den Befehl aus Schritt 2 nach jedem Update von `a38` erneut aus.

## Prüfen

`a38 llm` ohne `--skill` meldet einen Fehler
und endet mit dem Code 2.
Die erste Zeile der Ausgabe ist `---`. Danach folgen `name: a38`
und eine `description`.
