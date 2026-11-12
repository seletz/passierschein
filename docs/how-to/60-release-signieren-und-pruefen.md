---
title: Release signieren und prüfen
description: "Die fünf Secrets für Signatur und Notarisierung setzen, mit dem signierten Snapshot testen, die Signatur eines heruntergeladenen Release prüfen und die App installieren."
render_macros: true
tags:
  - Thema/Release
---
# Release signieren und prüfen

Diese Anleitung richtet die Signatur mit der Developer ID und die Notarisierung bei Apple ein.
Sie zeigt auch, wie du ein heruntergeladenes Release prüfst und installierst.
Den Ablauf des Release beschreibt die Referenz [Release](../reference/50-release.md).
Die Entscheidungen stehen in {{ ref("D18") }}, {{ ref("D46") }} und {{ ref("D54") }}.

## Secrets für Signatur und Notarisierung einrichten

Das Release braucht fünf Secrets im Repo `seletz/passierschein`.
Fehlt eines, scheitert der Release-Workflow und nennt das fehlende Secret.
Ein unsigniertes Release entsteht nie.

| Secret | Inhalt |
|---|---|
| `MACOS_SIGN_P12` | Zertifikat „Developer ID Application“ mit privatem Schlüssel als PKCS#12-Datei (`.p12`), Base64-kodiert |
| `MACOS_SIGN_PASSWORD` | Passwort der `.p12`-Datei |
| `MACOS_NOTARY_KEY` | Privater Schlüssel des App-Store-Connect-API-Keys (`.p8`), Base64-kodiert |
| `MACOS_NOTARY_KEY_ID` | Key-ID des API-Keys, zehn Zeichen |
| `MACOS_NOTARY_ISSUER_ID` | Issuer-ID des Teams, eine UUID |

Die Werte sind dieselben wie im Projekt total-recall.
GitHub kopiert Secrets nicht zwischen Repos.
Du setzt sie deshalb aus deinen Dateien neu.

1. Zertifikat setzen. Du brauchst die `.p12`-Datei mit Zertifikat und privatem Schlüssel aus der Schlüsselbundverwaltung:

    ```sh
    base64 -i developer-id-application.p12 | tr -d '\n' | gh secret set MACOS_SIGN_P12 --repo seletz/passierschein
    gh secret set MACOS_SIGN_PASSWORD --repo seletz/passierschein
    ```

    Der zweite Befehl fragt das Passwort ab.

2. API-Key setzen. Du brauchst die Datei `AuthKey_<KEYID>.p8` aus App Store Connect („Users and Access“, „Integrations“, „App Store Connect API“, Team-Key mit der Rolle „Developer“):

    ```sh
    base64 -i AuthKey_XXXXXXXXXX.p8 | tr -d '\n' | gh secret set MACOS_NOTARY_KEY --repo seletz/passierschein
    gh secret set MACOS_NOTARY_KEY_ID --repo seletz/passierschein
    gh secret set MACOS_NOTARY_ISSUER_ID --repo seletz/passierschein
    ```

    Die Befehle fragen Key-ID und Issuer-ID ab. Beide stehen in App Store Connect auf der Seite der API-Keys.

3. Namen prüfen:

    ```sh
    gh secret list --repo seletz/passierschein
    ```

    Die Liste zeigt die fünf Namen `MACOS_*`.

`gh secret set` liest den Wert von der Standardeingabe.
Der Wert landet so nicht in der Shell-History.
Lege die kodierten Dateien nie ins Repo.

## Mit dem signierten Snapshot testen

Der Workflow `signed-snapshot.yml` signiert und notarisiert wie ein Release.
Er veröffentlicht nichts.
Er legt keinen Tag und kein Release an.

1. Workflow starten, auf `develop` oder auf einem Branch:

    ```sh
    gh workflow run signed-snapshot.yml --repo seletz/passierschein --ref develop
    ```

2. Auf das Ende warten:

    ```sh
    gh run watch --repo seletz/passierschein "$(gh run list --repo seletz/passierschein --workflow signed-snapshot.yml --limit 1 --json databaseId --jq '.[0].databaseId')"
    ```

3. Das Ergebnis lesen. Der Schritt „Check Passierschein.app“ zeigt je Programm Bezeichner, Flags, Team-ID und die Kette der Zertifikate. Er endet mit `passes (developer-id)`.

Ein Lauf kostet etwa zehn bis zwanzig Minuten auf einem macOS-Runner.
Der größte Teil ist die Wartezeit auf den Notardienst.

Die App des Laufs liegt sieben Tage als Artefakt `signed-snapshot-app` am Lauf.
Zum Prüfen lädst du sie in ein leeres Verzeichnis:

```sh
mkdir -p /tmp/a38-pruefen && cd /tmp/a38-pruefen
gh run download --repo seletz/passierschein --name signed-snapshot-app
ditto -x -k passierschein-app-*-darwin-universal.zip .
```

Danach prüfst du wie im nächsten Abschnitt.

## Signatur eines Release prüfen

Lade das ZIP der App und die Prüfsummendatei des Release in ein Verzeichnis.

1. Prüfsumme prüfen:

    ```sh
    shasum -a 256 --ignore-missing -c checksums-v1.2.3.txt
    ```

2. Auspacken:

    ```sh
    ditto -x -k passierschein-app-v1.2.3-darwin-universal.zip .
    ```

3. Signatur der App lesen:

    ```sh
    codesign -dv --verbose=4 Passierschein.app
    ```

    Erwartet:

    - `Identifier=io.github.seletz.passierschein`
    - `Authority=Developer ID Application: …`, darunter `Authority=Developer ID Certification Authority` und `Authority=Apple Root CA`
    - `flags=0x10000(runtime)`: die Hardened Runtime
    - eine Zeile `Timestamp=…`
    - `TeamIdentifier=` mit der Team-ID

4. Signatur von `a38` im Bundle lesen:

    ```sh
    codesign -dv --verbose=4 Passierschein.app/Contents/MacOS/a38
    ```

    Erwartet: `Identifier=io.github.seletz.passierschein.a38`, dieselben `Authority`-Zeilen, dieselbe Team-ID, `flags=0x10000(runtime)` und ein `Timestamp`.

5. Bundle vollständig prüfen:

    ```sh
    codesign --verify --deep --strict --verbose=2 Passierschein.app
    ```

    Erwartet: `valid on disk` und `satisfies its Designated Requirement`.

6. Urteil von Gatekeeper für das Bundle:

    ```sh
    spctl -a -vv -t exec Passierschein.app
    ```

    Erwartet: `accepted` und `source=Notarized Developer ID`.

7. Angeheftetes Ticket prüfen:

    ```sh
    xcrun stapler validate Passierschein.app
    ```

    Erwartet: `The validate action worked!`

Für das einzelne `a38` aus dem tar-Archiv gilt `-t install` statt `-t exec`:

```sh
tar -xzf passierschein-v1.2.3-darwin-arm64.tar.gz a38
codesign -dv --verbose=4 ./a38
spctl -a -vv -t install ./a38
```

`-t exec` meldet für ein einzelnes Programm `rejected`, auch wenn es notarisiert ist.
An ein einzelnes Programm heftet Apple kein Ticket.
macOS holt das Ticket beim ersten Start bei Apple.

### Was Gatekeeper zeigt

- Notarisiert und signiert: Beim ersten Öffnen nach einem Download im Browser fragt macOS einmal, ob du die aus dem Internet geladene App öffnen willst. Eine Warnung vor Schadsoftware erscheint nicht.
- Nur signiert, nicht notarisiert: macOS blockiert die App und meldet, dass Apple sie nicht auf Schadsoftware prüfen konnte.
- Ad-hoc signiert oder unsigniert: macOS blockiert die App ebenfalls.
- Ohne Quarantäne-Attribut, also nach `curl` oder `gh release download`, prüft Gatekeeper beim Start nicht.

## App installieren

1. ZIP auspacken und das Bundle nach `/Applications` legen:

    ```sh
    ditto -x -k passierschein-app-v1.2.3-darwin-universal.zip /Applications
    ```

2. `a38` aus dem Bundle in den `PATH` legen:

    ```sh
    mkdir -p ~/.local/bin
    ln -s /Applications/Passierschein.app/Contents/MacOS/a38 ~/.local/bin/a38
    ```

3. Prüfen:

    ```sh
    a38 --version
    ```

Der Daemon löst den symbolischen Link auf.
Er findet die App dann als Geschwister im selben Bundle ({{ ref("D50") }}).
Eine Konfiguration `paths.ui_app` ist nicht nötig.

Mit dem signierten `a38` prüft der Daemon vor jedem Start der App deren Signatur.
Die App muss die Developer ID desselben Teams tragen.
Eine Debug-App oder eine Ad-hoc-App startet er dann nicht.
Für die Entwicklung nutzt du deshalb ein selbst gebautes `a38` (`mise run build`).
Ein selbst gebautes `a38` ist ad-hoc signiert, und der Daemon prüft die Signatur der App nicht.
Einzelheiten stehen in der [Referenz der Freigabe-App](../reference/85-freigabe-app.md#signatur-der-app).
