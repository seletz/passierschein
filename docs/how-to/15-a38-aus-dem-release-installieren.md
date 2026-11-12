---
title: a38 aus dem Release installieren
description: "Die App Passierschein mit a38 aus dem Release laden, die Prüfsumme und die Signatur prüfen, installieren, Token und Konfiguration einrichten, die Installation aktualisieren; Linux und mise."
render_macros: true
tags:
  - Thema/Release
  - Thema/Betrieb
---
# a38 aus dem Release installieren

Diese Anleitung installiert Passierschein aus einem Release auf GitHub.
Auf macOS lädst du die App `Passierschein.app`; `a38` liegt in ihrem Bundle.
Auf Linux gibt es nur den Client `a38 mcp` ({{ ref("D53") }}).
Wie ein Release entsteht und wie ein Maintainer es prüft, steht in [Release signieren und prüfen](60-release-signieren-und-pruefen.md).
Die Dateien eines Release listet die Referenz [Release](../reference/50-release.md#artefakte).

Voraussetzungen auf macOS: macOS 15 oder neuer, ein Mac mit Touch ID und Zugriff auf das Repo `seletz/passierschein`.
Das Repo ist privat; der Download braucht deshalb dein Konto bei GitHub.
In den Beispielen steht `v0.1.0`; setze den Tag des Release ein, das du installierst.

## Release laden

Mit `gh` (angemeldet mit `gh auth login`):

```sh
TAG=v0.1.0
mkdir -p /tmp/a38-installieren && cd /tmp/a38-installieren
gh release download "$TAG" --repo seletz/passierschein \
  --pattern "passierschein-app-$TAG-darwin-universal.zip" \
  --pattern "checksums-$TAG.txt"
```

Im Browser: Öffne <https://github.com/seletz/passierschein/releases>, wähle das Release und lade die zwei Dateien aus der Liste „Assets“ in einen leeren Ordner.

Der Download im Browser setzt auf dem ZIP ein Quarantäne-Attribut.
Beim ersten Öffnen fragt macOS deshalb einmal, ob du die App aus dem Internet öffnen willst.
`gh release download` setzt kein Quarantäne-Attribut; macOS fragt dann nicht.

## Prüfsumme prüfen

```sh
shasum -a 256 --ignore-missing -c "checksums-$TAG.txt"
```

Erwartet: `passierschein-app-v0.1.0-darwin-universal.zip: OK`.
Bei jeder anderen Ausgabe installierst du nichts.

## Auspacken und installieren

1. Packe das ZIP aus:

    ```sh
    ditto -x -k "passierschein-app-$TAG-darwin-universal.zip" .
    ```

    Du kannst auch im Finder doppelt auf das ZIP klicken.

2. Lege `Passierschein.app` nach `/Applications`:

    ```sh
    ditto Passierschein.app /Applications/Passierschein.app
    ```

    `ditto` behält das angeheftete Ticket der Notarisierung und die Signatur. Das Verschieben im Finder tut das auch.

3. Lege `a38` aus dem Bundle in den `PATH`:

    ```sh
    mkdir -p ~/.local/bin
    ln -s /Applications/Passierschein.app/Contents/MacOS/a38 ~/.local/bin/a38
    ```

    `~/.local/bin` muss im `PATH` stehen.
    Der Daemon löst den Link auf und findet die App im selben Bundle ({{ ref("D50") }}). Die Einstellung `paths.ui_app` ist nicht nötig.

## Signatur prüfen

```sh
codesign --verify --deep --strict --verbose=2 /Applications/Passierschein.app
spctl -a -vv -t exec /Applications/Passierschein.app
xcrun stapler validate /Applications/Passierschein.app
```

Erwartet, am Release `v0.1.0` geprüft:

- `codesign`: `valid on disk` und `satisfies its Designated Requirement`.
- `spctl`: `accepted`, `source=Notarized Developer ID` und die Team-ID `23BX9MWDBN` in der Zeile `origin`.
- `stapler`: `The validate action worked!`

Weicht eine Ausgabe ab, starte die App nicht.
Die Einzelheiten jeder Zeile stehen in [Release signieren und prüfen](60-release-signieren-und-pruefen.md#signatur-eines-release-prufen).

Prüfe danach `a38`:

```sh
a38 version
```

Erwartet: eine Zeile mit der Version des Release, dem Commit und der Bauzeit, zum Beispiel `a38 0.1.0 (commit 4f30ed8, built …)`.

## Konfiguration anlegen

`a38` liest die Konfiguration aus `~/.config/a38/config.yaml` ({{ ref("D49") }}).
Sie nennt die Vaults und die Projekte.
Ein Beispiel und den Aufbau zeigt [1Password-Backend einrichten](30-1password-backend-einrichten.md#konfiguration); den Aufbau erklärt die [Architektur](../explanation/architektur.md).
Die Datei legst du heute von Hand an.

## Token ablegen

Das Token des 1Password-Service-Accounts liegt im Schlüsselbund von macOS ({{ ref("D51") }}).
Den Service Account legst du an, wie in [1Password-Backend einrichten](30-1password-backend-einrichten.md#service-account-anlegen) beschrieben.
Dann legst du das Token ab:

```sh
a38 config onepassword-token set
```

Füge das Token beim Prompt ein.
Der Standard der Quelle ist `keychain`; die Konfiguration braucht dafür keinen Eintrag.

Hast du eine ältere Konfiguration, die nur `token_file` nennt, hast du zwei Wege:

- Lege das Token mit `a38 config onepassword-token set` im Schlüsselbund ab und entferne `token_file` aus der Konfiguration.
- Oder trage `token_source: file` unter `backend.onepassword` ein. Dann liest der Daemon weiter die Token-Datei.

Die Datei ist der Ausweichweg für Entwicklung und CI.
Den Umzug beschreibt [1Password-Backend einrichten](30-1password-backend-einrichten.md#von-der-token-datei-in-den-schlusselbund-umziehen).

## Prüfen und starten

```sh
a38 doctor
a38 serve --backend 1password --ui app
```

`a38 doctor` zeigt jeden Befund mit dem nächsten Schritt.
Der Daemon startet die App `Passierschein` selbst.
Danach meldest du `a38 mcp` im Agent-Werkzeug an ([a38 als MCP-Server anmelden](22-mcp-server-anmelden.md)) und installierst den Skill ([Skill für Agents installieren](20-skill-installieren.md)).

## Installation aktualisieren

1. Beende den Daemon (`Ctrl-C` im Terminal von `a38 serve`). Der Neustart beendet alle Grants ({{ ref("D33") }}).
2. Lade, prüfe und packe das neue Release aus, wie oben.
3. Ersetze die App:

    ```sh
    rm -r /Applications/Passierschein.app
    ditto Passierschein.app /Applications/Passierschein.app
    ```

    Der Link `~/.local/bin/a38` zeigt weiter auf das Bundle. Du legst ihn nicht neu an.
4. Prüfe mit `a38 version` und `a38 doctor`, und starte den Daemon neu.

Der Eintrag im Schlüsselbund bleibt gültig, weil jeder Build des Release mit derselben Developer ID signiert ist.
So steht es in der Dokumentation von macOS; ein Praxistest mit einem Update steht noch aus ({{ ref("G5") }}).
Erscheint beim Start des Daemons trotzdem ein Dialog, der fragt, ob `a38` den Eintrag `a38.onepassword` lesen darf:

- Hast du gerade selbst `a38` aktualisiert und den Daemon gestartet, ist der Dialog erwartet. Du kannst ihn bestätigen.
- Alternativ legst du das Token neu ab: `a38 config onepassword-token set --replace`.
- Kommt der Dialog ohne einen Grund, den du kennst, wähle „Verweigern“. Ein anderes Programm, auch ein Agent, kann denselben Dialog auslösen.

Melde einen Dialog nach einem Update als Beobachtung; er hilft, die Lücke {{ ref("G5") }} zu schließen.

## Nur den Client auf Linux installieren

Auf Linux läuft nur `a38 mcp` ({{ ref("D53") }}).
Der Daemon, die App und der Schlüsselbund gibt es dort nicht.
Der Client spricht mit einem Daemon auf einem Mac.

```sh
TAG=v0.1.0
ARCH=amd64   # oder arm64
mkdir -p /tmp/a38-installieren && cd /tmp/a38-installieren
gh release download "$TAG" --repo seletz/passierschein \
  --pattern "passierschein-$TAG-linux-$ARCH.tar.gz" --pattern "checksums-$TAG.txt"
sha256sum --ignore-missing -c "checksums-$TAG.txt"
tar -xzf "passierschein-$TAG-linux-$ARCH.tar.gz" a38
install -m 0755 a38 ~/.local/bin/a38
a38 version
```

## Mit mise installieren

`mise` installiert das einzelne `a38` aus dem tar-Archiv, ohne die App.
Das Release enthält für macOS zwei Dateien, die passen könnten: das tar-Archiv `passierschein-<tag>-darwin-<arch>.tar.gz` und das ZIP der App.
Getestet am 2026-11-12 mit mise 2026.11.6 und dem Release `v0.1.0`: `mise` wählt auf macOS (arm64) das tar-Archiv und legt `a38` mit `LICENSE` und `README.md` in das Installationsverzeichnis.
Das ZIP der App wählt `mise` nicht.
Der Test lief in einem leeren Verzeichnis mit eigenem `HOME` und eigenen Verzeichnissen von mise; die Installation des Anwenders blieb unberührt.

Die Zeile für `mise.toml`:

```toml
[tools]
"github:seletz/passierschein" = "latest"
```

Eine Option `bin` oder `asset_pattern` ist nicht nötig: `a38` liegt im Archiv ohne Unterverzeichnis, und die Autoerkennung wählt das Archiv.
Das Repo ist privat; `mise` braucht ein Token für GitHub in der Umgebung (`GITHUB_TOKEN` oder `MISE_GITHUB_TOKEN`).
Ein Token aus `gh auth token` genügt. Gib es nie in einer Datei des Repos an.

Beachte:

- Das `a38` aus dem Archiv ist mit der Developer ID signiert; ein Ticket ist nicht angeheftet, macOS holt es beim ersten Start online ([Release signieren und prüfen](60-release-signieren-und-pruefen.md#signatur-eines-release-prufen)).
- Für das Popup braucht der Daemon die App. Installiere sie wie oben nach `/Applications`; der Daemon findet sie dort ({{ ref("D50") }}). Sonst gib den Pfad mit `--ui-app` oder `paths.ui_app` an.
- Auf Linux wählt `mise` das tar-Archiv der Plattform.
