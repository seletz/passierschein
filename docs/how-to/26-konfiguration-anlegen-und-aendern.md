---
title: Konfiguration anlegen und ändern
description: "Die Konfigurationsdatei von a38 mit a38 config init anlegen, Vaults und Projekte mit a38 config vault und a38 config project setzen, prüfen und den Daemon neu starten."
render_macros: true
tags:
  - Thema/Betrieb
---
# Konfiguration anlegen und ändern

Diese Anleitung legt die Konfigurationsdatei von `a38` an und ändert sie mit Befehlen ({{ ref("F15") }}, {{ ref("UC11") }}).
Du musst die Datei nicht von Hand schreiben.
Die Befehle behalten deine Kommentare, die Reihenfolge und die Leerzeilen.
Die Entscheidung dazu steht in {{ ref("D55") }}.
Format und Ort der Datei legt {{ ref("D49") }} fest.

## Schritt 1: Prüfen, welche Datei gilt

```sh
a38 config path
```

Die Ausgabe nennt den Pfad, den Grund und ob die Datei existiert.
Die Reihenfolge ist: Schalter `--config`, Variable `A38_CONFIG`, `$XDG_CONFIG_HOME/a38/config.yaml`, `~/.config/a38/config.yaml`.

## Schritt 2: Datei anlegen

```sh
a38 config init
```

Der Befehl legt die Datei mit den Standardwerten und kurzen Kommentaren an.
Das Verzeichnis hat den Modus 0700, die Datei den Modus 0600.
Die Datei nennt die Quelle des Tokens ausdrücklich: `token_source: keychain`.
Der Befehl überschreibt nie eine Datei.
Existiert sie, endet er mit 1 und nennt den Pfad.
`a38 config init --force` verschiebt die alte Datei nach `config.yaml.bak-<Zeitstempel>` und schreibt eine neue.

## Schritt 3: Vaults eintragen

```sh
a38 config vault add datenbank-dev --environment dev
a38 config vault list
```

Die Umgebung ist `dev`, `test` oder `prod`.
Der Name muss genau der Name des Vaults im Backend sein ({{ ref("D12") }}).

## Schritt 4: Projekt eintragen

```sh
a38 config project add beispielprojekt --path ~/develop/beispielprojekt --path ~/develop/beispielprojekt__worktrees --vault datenbank-dev
a38 config project list
```

- Ein Pfad ist absolut oder beginnt mit `~/`. Ein relativer Pfad ist ein Fehler.
- Jeder Vault steht schon in der Liste.
- Ein Pfad, der nicht existiert, ist erlaubt. Der Befehl nennt ihn als Hinweis.

Ein Vault, den noch ein Projekt nutzt, lässt sich nicht entfernen. Die Fehlermeldung nennt die Projekte.

## Schritt 5: Token ablegen

```sh
a38 config onepassword-token set
```

Siehe [1Password-Backend einrichten](30-1password-backend-einrichten.md).

## Schritt 6: Prüfen

```sh
a38 config validate
a38 config show
a38 doctor
```

- `validate` lädt die Datei mit den Regeln des Daemons. Exit-Code 0: kein Befund. Exit-Code 1: Befunde mit dem nächsten Schritt.
- `show` zeigt jeden Wert mit seiner Herkunft (`file` oder `default`), die aufgelösten Pfade, die Projekte und das Ergebnis der Prüfung.
- `doctor` prüft zusätzlich den Daemon, die Sockets und das Token.

## Schritt 7: Daemon neu starten

Der Daemon liest die Datei nur beim Start ({{ ref("D49") }}).
Jeder Befehl, der die Datei ändert, weist darauf hin.
Beende `a38 serve` und starte es neu. Läuft der Daemon als Dienst, startet `a38 service restart` ihn neu ([Daemon als Dienst einrichten](28-daemon-als-dienst-einrichten.md)).

## Wenn etwas schiefgeht

| Meldung | Ursache und Schritt |
|---|---|
| „the file … exists“ | `init` überschreibt nie. Nutze `--force` oder die Befehle `vault` und `project`. |
| „the path is a symbolic link“ | a38 schreibt nicht durch einen Link. Nutze `--config` mit dem Ziel. |
| „the lock file … exists“ | Eine andere Änderung läuft. Läuft keine, lösche die Sperrdatei `config.yaml.lock`. |
| „the file changed while a38 edited it“ | Ein anderes Programm schrieb gleichzeitig. Es wurde nichts geschrieben; wiederhole den Befehl. |
| „cannot create a temporary file“ | Das Verzeichnis ist nicht beschreibbar. Prüfe die Rechte. |
| „the file is not valid“ | a38 ändert keine fehlerhafte Datei. Behebe sie mit `a38 config validate`. |
