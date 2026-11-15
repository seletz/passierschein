---
title: Daemon als Dienst einrichten
description: "a38 aus dem Release installieren, den Daemon mit a38 service install als LaunchAgent einrichten, mit a38 service status und a38 doctor prüfen, nach einem Update neu starten und den Dienst entfernen."
render_macros: true
tags:
  - Thema/Betrieb
---
# Daemon als Dienst einrichten

Diese Anleitung richtet den Daemon `a38 serve` als LaunchAgent ein ({{ ref("F15") }}, {{ ref("UC11") }}).
launchd startet den Daemon dann mit jeder Anmeldung am Mac.
Der Dienst läuft nur unter macOS ({{ ref("D53") }}).
Die Regeln des Dienstes stehen in {{ ref("D56") }}.

!!! note "Kein Neustart von selbst"
    launchd startet den Daemon bei der Anmeldung und danach nie wieder von selbst.
    Endet der Daemon, bleibt er aus, bis du ihn mit `a38 service restart` startest.
    Ein Neustart hebt die globale Sperre auf ({{ ref("D48") }}); das soll nur der Mensch tun.

## Voraussetzungen

- macOS 15 oder neuer.
- Du arbeitest in einem Terminal auf dem Bildschirm des Macs, nicht über SSH. launchd lädt den Dienst nur in die Sitzung am Bildschirm.
- Du hast das Release mit `Passierschein.app` heruntergeladen und geprüft ([Release signieren und prüfen](60-release-signieren-und-pruefen.md)).

## a38 aus dem Release installieren

1. Die App nach `/Applications` legen:

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

Der Dienst startet später nicht den Link, sondern die Datei im Bundle.
`a38 service install` löst den Link selbst auf.

## Konfiguration und Token einrichten

1. Lege die Konfiguration an und trage Vaults und Projekte ein ([Konfiguration anlegen und ändern](26-konfiguration-anlegen-und-aendern.md)):

    ```sh
    a38 config init
    a38 config validate
    ```

2. Lege das Token des Service Accounts jedes Kontos im Schlüsselbund ab:

    ```sh
    a38 config onepassword-token set --account privat
    ```

    Führe den Befehl mit dem `a38` aus dem Bundle aus, also über den Link aus dem letzten Abschnitt.
    Der Schlüsselbund vertraut dann genau diesem `a38`, und der Dienst liest das Token ohne Dialog ({{ ref("D72") }}).

3. Prüfe die Konfiguration:

    ```sh
    a38 doctor
    ```

    Ohne laufenden Daemon meldet `a38 doctor` den Befund `daemon`. Das ist hier noch richtig.

## Dienst einrichten

1. Vorschau ansehen. Der Befehl prüft alles und zeigt die plist, ändert aber nichts:

    ```sh
    a38 service install --dry-run
    ```

2. Dienst einrichten:

    ```sh
    a38 service install
    ```

Der Befehl prüft vorher, ob der Daemon starten kann:

- Die Konfiguration lädt.
- Die Freigabe-App „Passierschein“ besteht die Prüfung der Datei.
- Für jedes Konto mit Vault existiert der Eintrag im Schlüsselbund. Der Befehl liest kein Token. Mit `token_source: file` prüft er nur Besitzer und Modus der Token-Datei. Fehlt nur bei einem Teil der Konten das Token, ist das eine Warnung: Der Daemon startet und sperrt die Vaults dieser Konten ({{ ref("D73") }}). Fehlt es bei allen, ist es ein Befund.
- Die Pfade der Sockets sind höchstens 103 Byte lang.
- Kein anderer Daemon antwortet auf dem Admin-Socket. Läuft ein Daemon, den du von Hand gestartet hast, beende ihn zuerst mit Ctrl-C.
- `a38` gehört dir oder `root`, und weder Gruppe noch andere dürfen die Datei oder ihr Verzeichnis schreiben.

Jeder Befund steht als Zeile `FINDING` mit dem nächsten Schritt in der Ausgabe.
Der Befehl richtet dann nichts ein und endet mit dem Exit-Code 1.

Mit Befund nennt die Ausgabe zum Beispiel:

```text
FINDING  account "privat": the keychain has no token of the 1Password service account (token_source: keychain)
         Next step: run a38 config onepassword-token set --account privat in this terminal, then run a38 service install again
a38 service install: refused: the daemon would not start; fix the findings first
```

Ohne Befund schreibt der Befehl die plist nach `~/Library/LaunchAgents/io.github.seletz.passierschein.daemon.plist`.
Danach lädt er den Dienst mit `launchctl enable` und `launchctl bootstrap`.
launchd startet den Daemon sofort.

Ein zweiter Aufruf mit denselben Angaben ändert nichts.
Mit anderen Angaben, zum Beispiel einer anderen Konfigurationsdatei, ersetzt der Befehl die plist und startet den Daemon neu.

## Was macOS zeigt

- macOS zeigt die Mitteilung „Hintergrundobjekt hinzugefügt“.
- Unter **Systemeinstellungen > Allgemein > Anmeldeobjekte und Erweiterungen** steht der Eintrag im Bereich „Im Hintergrund erlauben“. Lass ihn eingeschaltet.
- Mit dem signierten Release steht dort „Passierschein“. Mit einem selbst gebauten `a38` steht dort wahrscheinlich `a38`.

Schaltest du den Eintrag aus, startet launchd den Daemon nicht mehr.
`a38 service status` und `a38 doctor` melden dann den Befund.

## Dienst prüfen

```sh
a38 service status
a38 doctor
```

`a38 service status` zeigt:

| Zeile | Inhalt |
|---|---|
| `Service` | Ort der plist, oder „not installed“ |
| `Program` | Programm und Argumente aus der plist; ob es dasselbe `a38` ist wie das aufgerufene |
| `launchd` | geladen, läuft mit Prozess-ID, oder läuft nicht mit dem letzten Exit-Code |
| `Log` | Pfad und Größe des Logs |
| `Daemon` | ob der Daemon auf dem Admin-Socket antwortet; Version, Sitzungen, Grants und die globale Sperre |

Der Exit-Code ist 0, wenn der Dienst läuft und das Programm passt, sonst 1.

## Erster Start nach der Anmeldung

- launchd startet den Daemon gleich nach der Anmeldung.
- macOS entsperrt den Anmelde-Schlüsselbund mit deinem Passwort. Der Daemon liest das Token dann ohne Dialog.
- Vertraut der Eintrag einem anderen Build von `a38`, zeigt macOS einen Dialog. Der Daemon wartet zwei Minuten darauf. Danach endet er mit dem Exit-Code 1.
- Bestätige keinen Dialog für `a38.onepassword`, den du nicht erwartest. Lege das Token stattdessen neu ab und starte neu:

    ```sh
    a38 config onepassword-token set --account privat --replace
    a38 service restart
    ```

- Erkennt der Daemon eine Störung, sperrt er die Auslieferung und läuft weiter ({{ ref("D48") }}). `a38 service status` zeigt dann `delivery LOCKED`. Das Vorgehen steht in [Störung finden und beheben](24-stoerung-finden-und-beheben.md).

## Log lesen

Der Daemon schreibt nach `~/Library/Logs/a38/serve.log`.

```sh
tail -n 50 ~/Library/Logs/a38/serve.log
```

Die Konsole von macOS zeigt die Datei unter „Log-Berichte“.
Das Log enthält keinen Secret-Wert.
Niemand rotiert die Datei.
Ist sie dir zu groß, leere sie:

```sh
: > ~/Library/Logs/a38/serve.log
```

## Nach einer Änderung neu starten

Der Daemon liest Konfiguration und Tokens nur beim Start ({{ ref("D49") }}, {{ ref("D73") }}).
Nach einer Änderung startest du ihn neu:

```sh
a38 service restart
```

Der Neustart beendet alle Grants ({{ ref("D33") }}).

## Nach einem Update der App

1. Die neue App nach `/Applications` legen, wie oben.
2. Prüfen, ob die plist noch passt:

    ```sh
    a38 service status
    ```

    Liegt `a38` am selben Ort, passt die plist.
    Hat sich der Ort geändert, zeigt die Zeile `Program` eine Warnung. Dann richtest du den Dienst neu ein:

    ```sh
    a38 service install
    ```

3. Den Daemon neu starten, damit er das neue Programm nutzt:

    ```sh
    a38 service restart
    ```

Mit der Developer ID liest das neue `a38` das Token ohne Dialog.
Ohne Developer ID braucht jeder neue Build einen Dialog ({{ ref("G5") }}); lege das Token dann mit `a38 config onepassword-token set --account <name> --replace` neu ab.

## Dienst entfernen

```sh
a38 service uninstall
```

Der Befehl beendet den Daemon mit SIGTERM.
Der Daemon beendet seine Grants und schreibt das Ende ins Audit-Log.
Danach entfernt der Befehl die plist.
Das Log und die Konfiguration bleiben.

Eine plist mit demselben Label, die `a38 service install` nicht geschrieben hat, ändert und entfernt der Befehl nicht.
Prüfe die Datei dann selbst. `--force` ersetzt oder entfernt sie.

## Probleme

| Befund | Ursache | Schritt |
|---|---|---|
| `launchctl bootstrap … failed` | Der Befehl lief über SSH, oder der Eintrag in den Anmeldeobjekten ist ausgeschaltet. | Im Terminal am Bildschirm ausführen; den Eintrag einschalten; `a38 service install` |
| `loaded, not running`, `exit code 1` | Fehler beim Start: Konfiguration, Token, Verzeichnis der Sockets | Log lesen, Ursache beheben, `a38 service restart` |
| `exit code 2` | Falsche Argumente in der plist oder Absturz | Log lesen, `a38 service install` |
| `the program does not exist` | Ein Update hat `a38` verschoben. | `a38 service install` mit dem neuen `a38` |
| `WARNING … lies in the source directory` | `a38` stammt aus `mise run build`. | Für den Dienst das `a38` aus dem Release nutzen |
