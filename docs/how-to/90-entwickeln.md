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

## Freigabe-App bauen und testen

Die Freigabe-App in `app/` ist Swift-Code.
Sie baut nur auf macOS.

Voraussetzungen:

- macOS 15 oder neuer.
- Xcode ist installiert, und `xcode-select -p` zeigt auf Xcode. Die App braucht das macOS-SDK von Xcode.
- Die Swift-Version legt `mise.toml` an den Tasks `app:*` fest (Swift 6.4.2). `mise` installiert die Toolchain beim ersten Aufruf. Der Download ist etwa 1,6 GB groß.

1. Baue Debug und Release:

    ```sh
    mise run app:build
    ```

    Der Task legt `app/.build/bundle/a38-approve.app` an.
    Er prüft, dass der Release-Build jeden Testschalter ablehnt.

2. Führe die Tests aus:

    ```sh
    mise run app:test
    ```

    Kein Test öffnet ein Fenster.
    Kein Test ruft Touch ID auf.

3. Zeige ein Beispiel:

    ```sh
    mise run app:run:sample -- bundle
    ```

    Der Task startet den Debug-Build mit `--skip-auth` und `--auto-close 30`.
    Touch ID erscheint dabei nie.
    Ohne Namen zeigt der Task `first-request`. Ein unbekannter Name listet die Beispiele aus `app/Samples/`.

!!! warning "Touch ID nur mit Absicht"
    Der Debug-Build ohne `--skip-auth` und der Release-Build rufen bei der Bestätigung den echten Dialog von Touch ID auf.
    Starte die App für Versuche immer mit `mise run app:run:sample` oder mit `--skip-auth`.

`mise run check` enthält die Tasks der App nicht.
Die CI läuft auf Linux und baut die App nicht.
Vor einem Pull Request mit Änderungen in `app/` führe `mise run app:build` und `mise run app:test` lokal aus.
Die Referenz steht unter [Freigabe-App](../reference/85-freigabe-app.md).

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
