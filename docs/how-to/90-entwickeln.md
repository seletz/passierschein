---
title: Entwicklungsumgebung einrichten
description: "Werkzeuge mit mise installieren, Git-Hook einrichten, Checks ausführen, mit Worktrees arbeiten."
---
# Entwicklungsumgebung einrichten

Diese Anleitung richtet einen Klon des Repos für die Entwicklung ein.

## Voraussetzungen

- [mise](https://mise.jdx.dev/) ist installiert.
- `git` ist installiert.
- Für Worktrees: [Worktrunk](https://worktrunk.dev/) (`wt`) ist installiert.

## Werkzeuge installieren

1. Wechsle in das Repo.
2. Installiere die Werkzeuge:

    ```sh
    mise install
    ```

`mise` installiert die Versionen aus `mise.toml`. Die Datei legt jede
Werkzeugversion fest.

## Git-Hook einrichten

Der Hook `pre-commit` prüft die vorgemerkten Änderungen mit gitleaks auf
Secrets.

```sh
mise run hooks:install
```

Der Befehl gilt für den Klon und für alle seine Worktrees. Führe den Befehl
einmal pro Klon aus.

## Checks ausführen

```sh
mise run check
```

Der Task führt dieselben Checks aus wie die CI. Die Liste der Tasks steht in
der [Referenz der Tasks](../reference/40-tasks.md).

## Mit Worktrees arbeiten

Niemand arbeitet direkt auf dem Branch `develop`. Jede Änderung entsteht in
einem eigenen Worktree und kommt über einen Pull Request nach `develop`.

1. Lege ein Issue für die Änderung an.
2. Lege den Worktree an:

    ```sh
    wt switch --create feat/12-kurzer-name
    ```

3. Ändere, prüfe und committe im Worktree.
4. Öffne einen Pull Request nach `develop`.

Der Branchname beginnt mit der Art der Änderung und der Nummer des Issues.
Die Konventionen für Commits stehen in den
[Konventionen](../reference/60-konventionen.md).
