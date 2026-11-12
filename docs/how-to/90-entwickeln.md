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

    Der Task legt `app/.build/bundle/Passierschein.app` an.
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
Vor einem Pull Request mit Änderungen in `app/` führe `mise run app:build`, `mise run app:test` und `mise run app:e2e` lokal aus.
Die Anleitung [Freigabe-App lokal ausprobieren](40-freigabe-app-ausprobieren.md) zeigt den Daemon mit der Debug-App.
Die Referenz steht unter [Freigabe-App](../reference/85-freigabe-app.md).

## Tests mit dem Schlüsselbund

Die Tests des Pakets `internal/keychain` legen echte Einträge im Anmelde-Schlüsselbund an.
Jeder Eintrag heißt `a38-test-…` und hat einen Dummy-Wert.
Dasselbe Test-Binary legt ihn an, liest ihn und löscht ihn.
Ein anderes Programm liest ihn nie, denn das würde einen Dialog von macOS zeigen.
Den Eintrag `a38.onepassword` verweigert das Paket in jedem Test-Binary.

Setze `A38_KEYCHAIN_TEST_LOG` auf eine Datei, um die Namen der Einträge mitzuschreiben.
Prüfe danach, dass keiner übrig ist:

```sh
A38_KEYCHAIN_TEST_LOG=/tmp/a38-keychain-tests.log go test ./internal/keychain/
for s in $(cat /tmp/a38-keychain-tests.log); do
  security find-generic-password -s "$s" >/dev/null 2>&1 && echo "left: $s"
done
```

Ohne `security … -w` liest der Befehl nur die Attribute und zeigt keinen Dialog.

Für einen Versuch mit dem gebauten `bin/a38` nennt `A38_TEST_KEYCHAIN_SERVICE` einen Testeintrag statt `a38.onepassword`.
Der Name muss mit `a38-test-` beginnen.
Nutze nur einen Dummy-Wert:

```sh
export A38_TEST_KEYCHAIN_SERVICE=a38-test-versuch
printf 'test-dummy-wert-1-versuch\n' | bin/a38 token set
bin/a38 token status
bin/a38 token delete
```

Ein neuer Build von `bin/a38` liest den Eintrag eines alten Builds nur nach einem Dialog.
Lösche den Testeintrag deshalb mit demselben Build, der ihn angelegt hat.

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
