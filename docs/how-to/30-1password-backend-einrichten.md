---
title: 1Password-Backend einrichten
description: "Einen Service Account mit Lesezugriff auf genau einen Vault anlegen, das Token mit a38 config onepassword-token set --account im Schlüsselbund ablegen, a38 serve mit dem 1Password-Backend starten, mit a38 doctor prüfen, das Token rotieren und den Integrationstest laufen lassen."
render_macros: true
tags:
  - Thema/Backend
  - Thema/Betrieb
---
# 1Password-Backend einrichten

Diese Anleitung richtet das 1Password-Backend von `a38 serve` ein ({{ ref("F12") }}).
Das Backend liest über das Go-SDK von 1Password mit dem Token eines Service Accounts ({{ ref("D1") }}, {{ ref("D39") }}).
Es gilt nur unter macOS ({{ ref("D42") }}, {{ ref("D53") }}).
Den Aufbau erklärt die [Architektur](../explanation/architektur.md#1password-backend).

Die Konfiguration nennt jedes 1Password-Konto mit einem Namen ({{ ref("D71") }}).
Jeder Vault nennt sein Konto ({{ ref("D70") }}).
Diese Anleitung richtet ein Konto mit dem Namen `privat` ein.
Mehrere Konten beschreibt [Mehrere 1Password-Konten einrichten](31-mehrere-1password-konten.md).

Das Token liegt im Schlüsselbund von macOS, in einem Eintrag je Konto ({{ ref("D72") }}).
Du fügst es im Terminal in `a38 config onepassword-token set --account privat` ein, und `a38` legt es im Schlüsselbund ab.
Die Token-Datei bleibt als Ausweichweg für Entwicklung und CI.

!!! warning "Grenzen des Schlüsselbunds ohne Developer ID"
    - Der Schlüsselbund bindet den Eintrag an genau den Build von `a38`, der ihn angelegt hat. Ohne Signatur mit der Developer ID braucht deshalb jeder neue Build von `a38` eine Bestätigung in einem Dialog von macOS ({{ ref("G5") }}).
    - Ein anderes Programm kann denselben Dialog auslösen, auch ein Agent. Der Dialog sieht dann gleich aus.
    - Bestätige nie einen Dialog für den Eintrag `a38.onepassword`, den du nicht selbst ausgelöst hast. Wähle im Zweifel „Verweigern“.
    - Der Schlüsselbund schützt nicht gegen das Auslesen des Speichers eines laufenden Daemons ohne Signatur ({{ ref("RI5") }}). Diesen Schutz bringt erst die Signatur mit Developer ID und Hardened Runtime.
    - Der Schutz gegen einen Prozess desselben Benutzers beginnt erst mit der Signatur mit Developer ID ({{ ref("G3") }}, {{ ref("G5") }}).

    Die Messungen dazu stehen im [Spike zum Schlüsselbund](../explanation/spikes/50-token-im-schluesselbund.md).

Die Konfigurationsdatei mit den Vaults und Projekten legst du vorher an: [Konfiguration anlegen und ändern](26-konfiguration-anlegen-und-aendern.md).
`a38 config init` schreibt eine leere Liste `accounts` in die Datei.
Trage das Konto ein:

```sh
a38 config account add privat
```

## Was der Service Account kann und was nicht

- Er liest die Items der Vaults, die du beim Anlegen wählst. Diese Anleitung wählt genau einen Vault.
- Er liest nie die Vaults „Personal“, „Private“, „Employee“ und den Standard-Vault „Shared“.
- Seine Rechte und seine Vaults sind nach dem Anlegen fest. Ein weiterer Vault braucht einen neuen Service Account.
- Mit dem Recht „Read Items“ ändert, legt und löscht er nichts.
- 1Password begrenzt die Zahl der Anfragen je Stunde ([Rate Limits](https://www.1password.dev/service-accounts/rate-limits)).
- Wer das Token hat, liest alle Items der gewählten Vaults, bis du das Token widerrufst. Gib deshalb nur einen Test-Vault oder dev-Vaults frei.

`a38` prüft zusätzlich selbst: Das Backend fragt nur nach den Vaults der Konfiguration ({{ ref("D12") }}).

## Service Account anlegen

Es gibt zwei Wege: die Weboberfläche oder die 1Password-CLI in deinem eigenen Terminal.
Die offizielle Anleitung steht unter [Get started with 1Password Service Accounts](https://www.1password.dev/service-accounts/get-started).

!!! danger "Das Token gehört nur in a38 config onepassword-token set --account"
    Füge das Token nirgends sonst ein: nicht in einen Chat, nicht in einen Agent, nicht in ein Issue, nicht in eine Shell-Variable, nicht als Argument eines Befehls.
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

Lege das Token danach im Schlüsselbund ab, wie im nächsten Abschnitt beschrieben.

### Weg 2: 1Password-CLI

Die CLI gibt das Token mit `--raw` auf stdout aus.
Leite es in die Zwischenablage, damit es nicht im Terminal erscheint:

```sh
op service-account create a38-passierschein-dev \
  --vault passierschein.dev:read_items \
  --raw | pbcopy
```

`read_items` gibt nur das Recht zu lesen.
Die [CLI-Referenz](https://www.1password.dev/cli/reference/management-commands/service-account) nennt die weiteren Schalter, zum Beispiel `--expires-in`.
Füge das Token zuerst in ein Item deines eigenen Vaults ein, dann in `a38 config onepassword-token set --account privat` (nächster Abschnitt).
Der Befehl liest das Token nur vom Terminal, nie aus einer Pipe.

Geprüft am 2026-11-12: Ein Token aus `op service-account create … --raw`, damals in die Token-Datei geschrieben, funktioniert mit dem SDK v0.4.1.
Der Weg über `a38 config onepassword-token set` ist noch nicht mit einem echten Token geprüft.
Der Weg über `--account` mit zwei Konten ist ebenfalls noch nicht mit echten Tokens geprüft.

## Token im Schlüsselbund ablegen

1. Kopiere das Token aus deinem Item in 1Password in die Zwischenablage.
2. Starte den Befehl in deinem Terminal:

    ```sh
    a38 config onepassword-token set --account privat
    ```

3. Füge das Token beim Prompt ein und drücke die Eingabetaste. Das Terminal zeigt das Token nicht an.
4. Leere die Zwischenablage. Jeder Prozess deines Benutzers kann sie lesen, solange das Token dort steht.

Die Ausgabe nennt nur den Eintrag:

```text
Stored the token of the account "privat" in the keychain item a38.onepassword/privat.
```

Das Konto muss in der Konfiguration stehen.
Sonst liest der Befehl nichts und endet mit dem Exit-Code 1.

Kein Befehl zeigt das Token oder einen Wert, der daraus abgeleitet ist, auch nicht Länge oder Prüfsumme.
Der Befehl liest das Token nur vom Terminal.
Ist die Standardeingabe kein Terminal, zum Beispiel eine Pipe oder eine Datei, liest er nichts und endet mit dem Exit-Code 2.
Ein zu kurzes Token, etwa nach einem abgeschnittenen Einfügen, lehnt er mit „too short“ ab.

Gibt es den Eintrag schon, ändert `a38 config onepassword-token set` nichts und endet mit dem Exit-Code 1.
Ersetze ihn mit `a38 config onepassword-token set --account privat --replace`.

Prüfe den Eintrag:

```sh
a38 config onepassword-token status --account privat
```

Der Befehl zeigt nur Metadaten: ob der Eintrag existiert, Datum der Anlage und der letzten Änderung, die vertrauten Programme, die Partitionsliste und die Signatur von `a38`.
Er liest das Token nie und zeigt deshalb nie einen Dialog von macOS.
Ob das Token gilt, zeigt erst der Start des Daemons.

`a38` legt den Eintrag selbst an, nicht über `/usr/bin/security`.
Über `security` angelegt, könnte jeder Prozess deines Benutzers das Token still mit `security find-generic-password -w` lesen.

## Konfiguration

Die Konfiguration nennt den Vault mit seiner Umgebung und seinem Konto:

```yaml
version: 1
vaults:
  - name: passierschein.dev
    environment: dev
    account: privat
projects:
  - name: beispielprojekt
    paths:
      - ~/develop/beispielprojekt
    vaults:
      - passierschein.dev
backend:
  onepassword:
    accounts:
      - name: privat
        token_source: keychain
```

`token_source` ist optional; der Standard ist `keychain`.
`a38 config vault add passierschein.dev --environment dev --account privat` trägt den Vault ein.
Die [Referenz](../reference/80-befehle-fuer-den-menschen.md#konfiguration-des-1password-backends) nennt alle Schlüssel.

## Daemon starten

Das echte Backend startet nur mit der Freigabe-App ({{ ref("D40") }}):

```sh
a38 serve --backend 1password --ui app
```

Der Daemon liest das Token einmal beim Start aus dem Schlüsselbund.
Danach hält er es nur im Arbeitsspeicher.
Eine Änderung im Schlüsselbund wirkt erst nach einem Neustart.

Fehlt der Eintrag des einzigen Kontos, startet der Daemon nicht.
Die Meldung nennt den Befehl `a38 config onepassword-token set --account privat`.
Mit mehreren Konten sperrt ein fehlendes Token nur die Vaults seines Kontos ({{ ref("D73") }}).

Er verbindet sich erst beim ersten Aufruf mit 1Password.
Das Log meldet kurz nach dem Start den Zustand:

- `the 1password account works` mit `account=privat`: Das Token gilt, und der Service Account sieht die Vaults.
- `the 1password account does not work yet` mit `code=backend_locked`: 1Password lehnt das Token ab.
- `the 1password account does not work yet` mit `code=backend_unavailable`: 1Password antwortet nicht, zum Beispiel ohne Netz.

Die Zeile davor (`the 1password backend failed`) nennt das Konto (`account=`) und die Klasse des Fehlers (`class=`):

| Klasse | Bedeutung |
|---|---|
| `authentication` | 1Password lehnt das Token ab. |
| `network`, `timeout` | 1Password antwortet nicht. |
| `rate_limit` | 1Password begrenzt die Zahl der Anfragen. |
| `integration_info` | Das SDK lehnt Namen oder Version ab, die `a38` sendet. Das ist ein Fehler von `a38` ([Issue 89](https://github.com/seletz/passierschein/issues/89)). |
| `input` | Das SDK lehnt eine andere Eingabe von `a38` ab. |
| `unknown` | Eine Meldung, die `a38` nicht kennt. |

Bei `integration_info`, `input` und `unknown` nennt das Feld `hint` die Wörter der Meldung des SDK, die auf einer festen Liste stehen.
Es nennt nie das Token, einen Wert oder ein anderes Wort der Meldung.
Mit diesen Wörtern meldest du den Fehler.

Bei einem ungültigen Token oder ohne Netz läuft der Daemon weiter.
Jede Anfrage endet dann mit `backend_locked` oder `backend_unavailable`.
Der Daemon versucht es frühestens nach 30 Sekunden erneut.

### Dialog nach einem neuen Build

Hat ein anderer Build von `a38` den Eintrag angelegt, zeigt macOS beim Start des Daemons einen Dialog.
Der Dialog fragt, ob `a38` den Eintrag `a38.onepassword` lesen darf.
Der Daemon wartet höchstens zwei Minuten auf die Antwort; danach gilt das Token des Kontos als fehlend.

- Hast du gerade selbst `a38` neu gebaut oder aktualisiert und den Daemon gestartet, ist der Dialog erwartet.
- Statt den Dialog zu bestätigen, kannst du das Token für den neuen Build neu ablegen: `a38 config onepassword-token set --account privat --replace`. Ob macOS beim Ersetzen selbst nachfragt, klärt der Praxistest.
- Kommt der Dialog ohne einen Grund, den du kennst, wähle „Verweigern“. Prüfe dann mit `a38 doctor` und `a38 config onepassword-token status`.

`a38 doctor` und `a38 config onepassword-token status` sagen vorher, ob der laufende Build den Eintrag ohne Dialog liest.

## Mit a38 doctor prüfen

```sh
a38 doctor
```

- Die Zeile `1password token` prüft je Konto die Quelle des Tokens. Sie liest kein Token.
- Jede Zeile nennt das Konto und seine Vaults.
- Mit der Quelle `keychain` nennt sie den Eintrag, sein Datum und die vertrauten Programme.
- Der alte Eintrag `a38.onepassword/service-account` ist ein Befund ([Umstellung](31-mehrere-1password-konten.md#von-einem-token-auf-konten-umstellen)).
- Eine Zeile `NOTE` nennt die Signatur von `a38`. Ohne Developer ID warnt sie vor dem Dialog nach jedem Build. Das ist kein Befund.
- Ein vertrautes Programm außer `a38`, zum Beispiel `/usr/bin/security`, ist ein Befund.
- Eine übrige Token-Datei ist ein Befund, wenn die Quelle der Schlüsselbund ist.
- Die Zeile `backend` zeigt den Zustand aus dem Daemon: `ok`, `locked` oder `unreachable`.

## Von der Token-Datei in den Schlüsselbund umziehen

Bis zu dieser Version lag das Token in der Token-Datei `~/.config/a38/onepassword-token`.

1. Lege das Token mit `a38 config onepassword-token set --account privat` im Schlüsselbund ab. Füge es aus 1Password ein, wie oben beschrieben.
2. Prüfe mit `a38 config onepassword-token status --account privat`, dass der Eintrag existiert.
3. Starte den Daemon neu. Das Log meldet `the 1password account works`. Prüfe mit `a38 doctor`.
4. Lösche die Token-Datei erst danach:

    ```sh
    rm ~/.config/a38/onepassword-token
    ```

5. Entferne `token_file` und `token_source: file` aus dem Konto in der Konfiguration, falls dort eingetragen.

Der Befehl liest die Datei nicht; füge das Token aus 1Password ein.

## Service Account der CI

Die CI führt denselben Integrationstest auf einem macOS-Runner aus ([CI](../reference/55-ci.md)).
Sie nutzt einen zweiten Service Account, nur für die CI, mit Lesezugriff auf `passierschein.dev`.
So kannst du ihn widerrufen, ohne den Service Account auf deinem Mac zu berühren.

Lege ihn in deinem eigenen Terminal an und setze ihn direkt als Secret des Repositorys:

```sh
op service-account create passierschein-ci --vault passierschein.dev:read_items --raw \
  | gh secret set OP_SERVICE_ACCOUNT_TOKEN --repo seletz/passierschein
```

Das Token erscheint dabei weder im Terminal noch auf der Platte.
`gh secret set` liest den Wert ohne `--body` von der Standardeingabe.
Füge das Token nie von Hand ein.

Prüfe danach den Workflow „macOS“: Starte ihn in GitHub unter **Actions** von Hand und sieh dir den Schritt „1Password integration test“ an.
Fehlt das Secret, scheitert der Schritt mit der Meldung `the repository secret OP_SERVICE_ACCOUNT_TOKEN is missing`.

So rotierst du das Token der CI:

1. Lege einen neuen Service Account an und setze das Secret mit dem Befehl oben, zum Beispiel mit dem Namen `passierschein-ci-2`.
2. Starte den Workflow „macOS“ von Hand und prüfe den Schritt „1Password integration test“.
3. Widerrufe den alten Service Account in 1Password.com unter **Developer**.

## Token rotieren

Die offizielle Anleitung steht unter [Manage service accounts](https://www.1password.dev/service-accounts/manage-service-accounts).

1. Öffne in 1Password.com **Developer** und den Service Account.
2. Wähle die Rotation des Tokens. Wähle, wann das alte Token abläuft, zum Beispiel nach einer Stunde.
3. Speichere das neue Token in deinem Item in 1Password.
4. Ersetze das Token im Schlüsselbund:

    ```sh
    a38 config onepassword-token set --account privat --replace
    ```

5. Starte den Daemon neu. Er liest das Token nur beim Start. Der Neustart beendet alle Grants ({{ ref("D33") }}).
6. Prüfe mit `a38 config onepassword-token status --account privat` das Datum der Änderung und mit `a38 doctor` den Zustand des Backends. Das Log des Daemons meldet `the 1password account works`.

Ist das Token entwendet, widerrufe es sofort in 1Password.com.
Der Widerruf wirkt sofort.

## Token löschen

```sh
a38 config onepassword-token delete --account privat
```

Der Befehl löscht den Eintrag des Kontos im Schlüsselbund.
Ein laufender Daemon behält das Token im Arbeitsspeicher, bis er endet.
Nach seinem nächsten Start sind die Vaults des Kontos gesperrt, bis du `a38 config onepassword-token set --account privat` ausführst.

## Token-Datei als Ausweichweg für die Entwicklung

Die Token-Datei ist nicht der normale Weg.
Sie bleibt für die Entwicklung und die CI.
Für die Entwicklung ist die Token-Datei bequemer: Jeder Neubau von `a38` würde sonst einen Dialog auslösen.
Ihre Schwäche: Jeder Prozess deines Benutzers kann die Datei lesen, auch ein Agent auf dem Mac ({{ ref("RI5") }}, {{ ref("G3") }}).
Die Datei kann in eine Datensicherung gelangen.
Nutze sie nur mit einem Test-Vault.

Wähle die Quelle am Konto in der Konfiguration:

```yaml
backend:
  onepassword:
    accounts:
      - name: privat
        token_source: file
        token_file: ~/.config/a38/onepassword-token-privat
```

`token_file` ist optional.
Der Standardpfad ist `~/.config/a38/onepassword-token-<name>`, mit gesetztem `XDG_CONFIG_HOME` `$XDG_CONFIG_HOME/a38/onepassword-token-<name>`.

Lege die Datei so an, dass das Token nicht in der Historie der Shell landet:

```sh
mkdir -p ~/.config/a38
umask 077
pbpaste > ~/.config/a38/onepassword-token-privat
chmod 600 ~/.config/a38/onepassword-token-privat
```

Die Datei muss diese Bedingungen erfüllen. Sonst startet der Daemon nicht:

- Sie ist eine reguläre Datei. Ein symbolischer Link oder ein Verzeichnis gilt nicht.
- Sie gehört deinem Benutzer.
- Ihre Rechte sind genau `0600`.
- Sie enthält das Token. Leerzeichen und Zeilenende am Anfang und am Ende zählen nicht.

Es gibt keine Umgebungsvariable für das Token.
Jeder Kindprozess erbt die Umgebung, auch ein Agent.

## CI und Tests

Die CI braucht keinen Schlüsselbund.

- Die CI läuft unter Linux mit dem Fake-Backend. Der Linux-Build enthält weder das 1Password-Backend noch Code für den Schlüsselbund ({{ ref("D42") }}).
- Die Tests von `a38` berühren nie einen Eintrag `a38.onepassword`, auch nicht den Eintrag eines Kontos. Die Tests des Pakets `internal/keychain` legen nur Einträge `a38-test-…` mit Dummy-Werten an und löschen sie wieder.
- Der Integrationstest liest das Token aus einer Datei, die `A38_OP_TOKEN_FILE` nennt (siehe unten).

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
Er liest das Token aus einer Token-Datei, nicht aus dem Schlüsselbund.

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

## Weitere Quellen

- [Sicherheit von Service Accounts](https://www.1password.dev/service-accounts/security)
- [Go-SDK von 1Password](https://github.com/1Password/onepassword-sdk-go)
- [Spike: 1Password-Go-SDK](../explanation/spikes/10-1password-sdk.md)
- [Spike: Token des Service Accounts im Schlüsselbund](../explanation/spikes/50-token-im-schluesselbund.md)
