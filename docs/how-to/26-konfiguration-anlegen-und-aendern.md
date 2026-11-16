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

## Einem Projekt einen Pfad hinzufügen

Ein Projekt hat oft mehr als ein Verzeichnis.
Ein Beispiel sind die Worktrees, die neben dem Repository liegen.
Eine Sitzung gehört zu einem Projekt, wenn ihr Arbeitsverzeichnis ein Pfad des Projekts ist oder darunter liegt.
Ein Worktree neben dem Repository liegt nicht unter dem Repository.
Darum braucht das Projekt das Verzeichnis der Worktrees als zweiten Pfad.

```sh
a38 config project add-path beispielprojekt --path ~/develop/beispielprojekt__worktrees
a38 config project list
```

- Der Befehl hängt den Pfad an die Liste des Projekts an. Er ändert nichts anderes in der Datei.
- Du kannst `--path` mehrfach angeben.
- Ein Pfad ist absolut oder beginnt mit `~/`. Ein relativer Pfad ist ein Fehler.
- Ein Pfad, den das Projekt schon hat, ist ein Fehler. Ein Pfad eines anderen Projekts auch.
- Ein Pfad, der nicht existiert, ist erlaubt. Der Befehl nennt ihn als Hinweis.
- Das Projekt muss in der Liste stehen. Sonst lege es mit `a38 config project add` an.

Ein Worktree-Werkzeug wie `wt` legt die Worktrees in das Verzeichnis `<Repository>__worktrees`.
Trage genau dieses Verzeichnis ein.
Der Daemon liest die Datei nur beim Start. Starte ihn danach neu (Schritt 7).

## Umgebungen und exec-Weg einstellen

Die Datei nennt je Umgebung den Weg der Auslieferung, die Laufzeit eines Grants und die Bindung ({{ ref("D66") }}).
`a38 config init` schreibt die Standardwerte in die Abschnitte `environments` und `exec`.
Ändere die Werte mit einem Editor. Die Befehle `vault` und `project` ändern diese Abschnitte nicht.

Beispiel: prod einschalten und die Laufzeit von dev kürzen.

```yaml
environments:
  dev:
    grant_lifetime: 1h
  prod:
    delivery: [exec]
```

- Die Umgebung `prod` ist im Standard aus (`delivery: []`). Mit `[exec]` schaltest du sie ein. Jedes Kommando braucht dann eine eigene Freigabe.
- `mcp` gilt nur für dev. In test und prod geht ein Wert nie in eine MCP-Antwort.
- In prod sind die Laufzeit `0s` und die Bindung `command` fest.
- Die Laufzeit `0s` in dev oder test heißt: Jedes Kommando braucht eine eigene Freigabe, wie in prod ({{ ref("D76") }}). `request_credentials` antwortet dann in dev mit `exec_required`.
- In test ist die Bindung `none` verboten.
- Die Referenz nennt alle Schlüssel und Grenzen: [Umgebungen und exec-Weg](../reference/80-befehle-fuer-den-menschen.md#umgebungen-und-exec-weg).

Steht noch `grant.lifetime` in deiner Datei, nennt `a38 config validate` den Schlüssel als veraltet.
Benenne ihn um in `environments.dev.grant_lifetime`.
Beide Schlüssel zusammen sind ein Fehler.

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

- `validate` lädt die Datei mit den Regeln des Daemons. Exit-Code 0: kein Befund; ein veralteter Schlüssel erscheint als Hinweis `NOTE`. Exit-Code 1: Befunde mit dem nächsten Schritt.
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
| „paths.ui_app names a file that does not exist“ | Der Pfad der Freigabe-App ist falsch. Korrigiere `paths.ui_app` in der Datei oder entferne den Schlüssel. |
| „environments.…“ oder „exec.…“ mit „must“ | Ein Wert verstößt gegen eine feste Regel von {{ ref("D66") }}. Die Meldung nennt den Schlüssel; setze einen erlaubten Wert. |
| „the file is not valid“ | a38 ändert keine fehlerhafte Datei. Behebe sie mit `a38 config validate`. |
