---
title: 1Password-Backend einrichten
description: "Einen Service Account mit Lesezugriff auf genau einen Vault anlegen, die Token-Datei ablegen, a38 serve mit dem 1Password-Backend starten, mit a38 doctor prüfen und den Integrationstest laufen lassen."
render_macros: true
tags:
  - Thema/Backend
  - Thema/Betrieb
---
# 1Password-Backend einrichten

Diese Anleitung richtet das 1Password-Backend von `a38 serve` ein ({{ ref("F12") }}).
Das Backend liest über das Go-SDK von 1Password mit dem Token eines Service Accounts ({{ ref("D1") }}, {{ ref("D39") }}).
Es gilt nur unter macOS ({{ ref("D42") }}).
Den Aufbau erklärt die [Architektur](../explanation/architektur.md#1password-backend).

!!! warning "Die Token-Datei ist ein Übergang"
    Das Ziel für das Token ist der Schlüsselbund von macOS, gebunden an das signierte Programm `a38` ({{ ref("D51") }}).
    Du fügst das Token dann im Terminal in einen Befehl von `a38` ein, und `a38` legt es im Schlüsselbund ab.
    Den Weg klärt der [Spike in Issue 53](https://github.com/seletz/passierschein/issues/53).
    Bis dahin ist die Token-Datei die einzige Quelle, für die Entwicklung und für Tests.
    Ihre Schwäche: Jeder Prozess deines Benutzers kann die Datei lesen, auch ein Agent auf dem Mac ({{ ref("RI5") }}, {{ ref("G3") }}).

## Was der Service Account kann und was nicht

- Er liest die Items der Vaults, die du beim Anlegen wählst. Diese Anleitung wählt genau einen Vault.
- Er liest nie die Vaults „Personal“, „Private“, „Employee“ und den Standard-Vault „Shared“.
- Seine Rechte und seine Vaults sind nach dem Anlegen fest. Ein weiterer Vault braucht einen neuen Service Account.
- Mit dem Recht „Read Items“ ändert, legt und löscht er nichts.
- 1Password begrenzt die Zahl der Anfragen je Stunde ([Rate Limits](https://www.1password.dev/service-accounts/rate-limits)).
- Wer das Token hat, liest alle Items der gewählten Vaults, bis du das Token widerrufst. Jeder Prozess deines Benutzers kann die Token-Datei lesen, auch ein Agent. Gib deshalb nur einen Test-Vault oder dev-Vaults frei.

`a38` prüft zusätzlich selbst: Das Backend fragt nur nach den Vaults der Konfiguration ({{ ref("D12") }}).

## Service Account anlegen

Es gibt zwei Wege: die Weboberfläche oder die 1Password-CLI in deinem eigenen Terminal.
Die offizielle Anleitung steht unter [Get started with 1Password Service Accounts](https://www.1password.dev/service-accounts/get-started).

!!! danger "Das Token gehört nur in die Token-Datei"
    Füge das Token nirgends sonst ein: nicht in einen Chat, nicht in einen Agent, nicht in ein Issue, nicht in eine Shell-Variable.
    Ein Agent führt diese Schritte nie aus; du führst sie selbst in deinem Terminal aus.

### Weg 1: Weboberfläche

Die Schritte in der Weboberfläche:

1. Melde dich bei 1Password.com an.
2. Öffne **Developer** > **Directory**. Wähle unter „Infrastructure Secrets Management“ den Eintrag **Other** und dann **Create a Service Account**.
3. Gib einen Namen ein, zum Beispiel `a38-passierschein-dev`.
4. Erlaube nicht, dass der Service Account Vaults anlegt.
5. Wähle genau einen Vault, zum Beispiel `passierschein.dev`.
6. Öffne beim Vault die Einstellungen (Zahnrad) und lass nur **Read Items** an. Schalte **Write Items** und **Share Items** aus.
7. Gib keinen Zugriff auf Environments.
8. Wähle **Create Account**.
9. 1Password zeigt das Token genau einmal. Speichere es mit **Save in 1Password** in deinem eigenen Vault.

Lege danach die Token-Datei an, wie im nächsten Abschnitt beschrieben.

### Weg 2: 1Password-CLI

Die CLI schreibt das Token mit `--raw` direkt in die Token-Datei.
Das Token erscheint so nie im Terminal:

```sh
mkdir -p ~/.config/a38
umask 077
op service-account create a38-passierschein-dev \
  --vault passierschein.dev:read_items \
  --raw > ~/.config/a38/onepassword-token
chmod 600 ~/.config/a38/onepassword-token
```

`read_items` gibt nur das Recht zu lesen.
Die [CLI-Referenz](https://www.1password.dev/cli/reference/management-commands/service-account) nennt die weiteren Schalter, zum Beispiel `--expires-in`.

Geprüft am 2026-11-12: Ein Token aus `op service-account create … --raw`, in die Token-Datei geschrieben, funktioniert mit dem SDK v0.4.1.
Der Integrationstest ist damit bestanden.

## Token-Datei ablegen

Der Daemon liest das Token heute aus einer Datei.
Mit Weg 2 ist sie schon angelegt; prüfe dann nur die Rechte mit `ls -l`.
Der Standardpfad ist `~/.config/a38/onepassword-token`.
Mit gesetztem `XDG_CONFIG_HOME` ist er `$XDG_CONFIG_HOME/a38/onepassword-token`.

Lege die Datei so an, dass das Token nicht in der Historie der Shell landet:

```sh
mkdir -p ~/.config/a38
umask 077
pbpaste > ~/.config/a38/onepassword-token
chmod 600 ~/.config/a38/onepassword-token
```

Kopiere vorher das Token aus deinem Item in 1Password in die Zwischenablage.
Leere danach die Zwischenablage.

Die Datei muss diese Bedingungen erfüllen. Sonst startet der Daemon nicht:

- Sie ist eine reguläre Datei. Ein symbolischer Link oder ein Verzeichnis gilt nicht.
- Sie gehört deinem Benutzer.
- Ihre Rechte sind genau `0600`.
- Sie enthält das Token. Leerzeichen und Zeilenende am Anfang und am Ende zählen nicht.

Es gibt keine Umgebungsvariable für das Token.
Jeder Kindprozess erbt die Umgebung, auch ein Agent.
Nimm die Datei aus einer Datensicherung heraus, wenn das geht.

## Konfiguration

Die Konfiguration nennt den Vault mit seiner Umgebung und, falls nötig, den Pfad der Token-Datei:

```yaml
version: 1
vaults:
  - name: passierschein.dev
    environment: dev
projects:
  - name: beispielprojekt
    paths:
      - ~/develop/beispielprojekt
    vaults:
      - passierschein.dev
backend:
  onepassword:
    token_file: ~/.config/a38/onepassword-token
```

`backend.onepassword.token_file` ist optional.
Die [Referenz](../reference/80-befehle-fuer-den-menschen.md#konfiguration-des-1password-backends) nennt alle Schlüssel.

## Daemon starten

Das echte Backend startet nur mit der Freigabe-App ({{ ref("D40") }}):

```sh
a38 serve --backend 1password --ui app
```

Der Daemon liest die Token-Datei einmal beim Start.
Er verbindet sich erst beim ersten Aufruf mit 1Password.
Das Log meldet kurz nach dem Start den Zustand:

- `the 1password backend works`: Das Token gilt, und der Service Account sieht die Vaults.
- `the 1password backend does not work yet` mit `code=backend_locked`: 1Password lehnt das Token ab.
- `the 1password backend does not work yet` mit `code=backend_unavailable`: 1Password antwortet nicht, zum Beispiel ohne Netz.

Bei einem ungültigen Token oder ohne Netz läuft der Daemon weiter.
Jede Anfrage endet dann mit `backend_locked` oder `backend_unavailable`.
Der Daemon versucht es frühestens nach 30 Sekunden erneut.

## Mit a38 doctor prüfen

```sh
a38 doctor
```

- Die Zeile `1password token` prüft die Token-Datei: vorhanden, eigener Benutzer, Rechte `0600`. Sie liest den Inhalt nicht.
- Die Zeile `backend` zeigt den Zustand aus dem Daemon: `ok`, `locked` oder `unreachable`.

## Namen in einer Referenz

- Der Vault muss genau so heißen wie in der Konfiguration.
- Item und Feld müssen genau so heißen wie in 1Password. Groß- und Kleinschreibung zählen.
- Das Item nennt die Referenz mit seinem Titel, nicht mit seiner Kennung.
- Ein Feld darf in einem Abschnitt liegen. Die Referenz nennt den Abschnitt nicht.
- Referenzen mit Attribut wie `?attribute=otp` gehen nicht ({{ ref("G25") }}).
- Zwei Items mit demselben Titel im Vault enden mit `backend_unavailable`. Das Log des Daemons nennt Vault und Titel. Benenne eines der Items um ({{ ref("D52") }}).

## Integrationstest

Der Test prüft das Backend gegen den Vault `passierschein.dev` ({{ ref("T15") }}).
Er liest nur.
Er berührt keinen anderen Vault.
Er gibt kein Token und keinen Wert aus.

Lege diese Items im Vault `passierschein.dev` von Hand in der 1Password-App an.
Nimm offensichtlich falsche Werte.

| Item | Kategorie | Feld | Art des Felds | Referenz |
|---|---|---|---|---|
| `datenbank-dev` | Login | `username` | Benutzername | `op://passierschein.dev/datenbank-dev/username` |
| `datenbank-dev` | Login | `password` | Passwort | `op://passierschein.dev/datenbank-dev/password` |
| `objektspeicher-dev` | Login | `access-key` | eigenes Feld, verdeckt | `op://passierschein.dev/objektspeicher-dev/access-key` |

Starte den Test:

```sh
A38_OP_TOKEN_FILE=~/.config/a38/onepassword-token mise run test:onepassword
```

Der Test prüft:

- Der Service Account sieht den Vault.
- Die drei Referenzen lösen sich auf.
- Jeder Wert ist nicht leer. Der Test gibt den Wert nicht aus.
- Ein fehlendes Item endet mit `not_found`.
- Ein Vault außerhalb der Konfiguration erreicht 1Password nicht.
- Ein kaputtes Token endet mit `backend_locked` oder `backend_unavailable`.

Optional vergleicht der Test die SHA-256-Prüfsumme des Passworts von `datenbank-dev`.
Setze dafür `A38_OP_EXPECT_SHA256` auf die Prüfsumme in Hex.

## Token rotieren

Die offizielle Anleitung steht unter [Manage service accounts](https://www.1password.dev/service-accounts/manage-service-accounts).

1. Öffne in 1Password.com **Developer** und den Service Account.
2. Wähle die Rotation des Tokens. Wähle, wann das alte Token abläuft, zum Beispiel nach einer Stunde.
3. Speichere das neue Token in deinem Item in 1Password.
4. Schreibe das neue Token in die Token-Datei, wie oben beschrieben.
5. Starte den Daemon neu. Der Neustart beendet alle Grants ({{ ref("D33") }}).
6. Prüfe mit `a38 doctor`.

Ist das Token entwendet, widerrufe es sofort in 1Password.com.
Der Widerruf wirkt sofort.

## Weitere Quellen

- [Sicherheit von Service Accounts](https://www.1password.dev/service-accounts/security)
- [Go-SDK von 1Password](https://github.com/1Password/onepassword-sdk-go)
- [Spike: 1Password-Go-SDK](../explanation/spikes/10-1password-sdk.md)
