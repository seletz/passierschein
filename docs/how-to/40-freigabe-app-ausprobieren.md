---
title: Freigabe-App lokal ausprobieren
description: "Die Freigabe-App Passierschein bauen, ein Beispiel zeigen, den Ablauf mit dem Daemon prüfen und den Daemon von Hand mit der Debug-App starten, ohne Touch ID."
render_macros: true
tags:
  - Thema/Freigabe
---
# Freigabe-App lokal ausprobieren

Diese Anleitung zeigt, wie du die Freigabe-App „Passierschein“ ohne Touch ID ausprobierst.
Sie gilt für macOS.
Die Referenz der App steht unter [Freigabe-App](../reference/85-freigabe-app.md).

## Voraussetzungen

- macOS 15 oder neuer mit Xcode ({{ ref("F11") }}).
- `mise install` ist gelaufen.
- Du sitzt am Bildschirm. Die Fenster erscheinen dort.

## Bauen

```sh
mise run app:build
```

Der Task baut Debug und Release und legt `app/.build/bundle/Passierschein.app` an.
Der erste Lauf lädt die Swift-Toolchain (etwa 1,6 GB).

## Ein Beispiel zeigen

```sh
mise run app:run:sample -- bundle
```

Der Task zeigt `app/Samples/bundle.json` im Debug-Build mit `--skip-auth` und `--auto-close 30`.
Touch ID erscheint nie.
Ohne Namen zeigt er `first-request`.
Ein Schalter hinter `--` geht an die App, zum Beispiel `--demo-select all`.

## Den Ablauf mit dem Daemon prüfen

```sh
mise run app:e2e
```

Der Task baut den Debug-Build und startet `a38 serve --ui app` über echte Prozesse.
Er prüft drei Fälle: gefälschte Authentifizierung, Zeit abgelaufen und eine Meldung.
Zwei Fenster erscheinen für je etwa zwei Sekunden auf deinem Bildschirm.
Touch ID erscheint nie.
Die Fälle stehen in der [Referenz der Freigabe-App](../reference/85-freigabe-app.md#ablauf-uber-echte-prozesse).

## Den Daemon von Hand mit der Debug-App starten

Der Daemon startet die App ohne Argumente.
Ein kleines Skript ruft die Debug-App deshalb mit Testschaltern auf.

1. Lege Konfiguration und Fake-Backend an, wie im [Tutorial](../tutorials/10-erste-schritte.md) (Schritt 2 und 3).
2. Schreibe das Skript. Ersetze `<klon>` durch den Pfad deines Klons:

    ```sh
    cat > /tmp/a38-tutorial/passierschein-debug <<'EOF'
    #!/bin/sh
    exec "<klon>/app/.build/out/Products/Debug/Passierschein" --skip-auth --auto-close 60
    EOF
    chmod 700 /tmp/a38-tutorial/passierschein-debug
    ```

    Das Skript besteht die Prüfung der Datei: eine reguläre Datei, die dir gehört und die nur du schreibst.
    `swift build -c debug --show-bin-path` in `app/` zeigt den Pfad des Debug-Builds, falls er bei dir abweicht.

3. Starte den Daemon mit dem Skript:

    ```sh
    a38 serve --config config.yaml --backend fake --fake-file fake.yaml \
      --ui app --ui-app /tmp/a38-tutorial/passierschein-debug
    ```

4. Fordere Zugangsdaten an, wie im Tutorial (Schritt 6). Das Popup erscheint.

`--skip-auth` ersetzt Touch ID durch eine gefälschte Authentifizierung.
Der Daemon nimmt eine solche Freigabe nie an ({{ ref("D7") }}).
Eine Bestätigung im Popup endet deshalb mit `ui_failed`, und der Agent erhält keinen Wert.
Du kannst im Popup ablehnen („Deny“, Esc) oder die Zeit ablaufen lassen.
Beides liefert dem Agent `denied` oder `timeout`.

!!! warning "Eine echte Freigabe braucht das echte Touch ID"
    Eine Freigabe, die einen Wert liefert, braucht den echten Dialog von Touch ID.
    Ohne `--skip-auth` und im Release-Build erscheint dieser Dialog bei der Bestätigung.
    Diesen Fall prüft Stefan am Bildschirm ({{ ref("T14") }}, Teil 2; {{ ref("G4") }}).
    Starte die App für Versuche nie ohne `--skip-auth`.
