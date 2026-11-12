---
title: 1Password-Backend einrichten
description: "Einen Service Account mit Lesezugriff auf genau einen Vault anlegen, das Token mit a38 token set im Schlüsselbund ablegen, a38 serve mit dem 1Password-Backend starten, mit a38 doctor prüfen, das Token rotieren und den Integrationstest laufen lassen."
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

Das Token liegt im Schlüsselbund von macOS ({{ ref("D51") }}).
Du fügst es im Terminal in `a38 token set` ein, und `a38` legt es im Schlüsselbund ab.
Die Token-Datei bleibt als Ausweichweg für Entwicklung und CI.

!!! warning "Grenzen des Schlüsselbunds ohne Developer ID"
    - Der Schlüsselbund bindet den Eintrag an genau den Build von `a38`, der ihn angelegt hat. Ohne Signatur mit der Developer ID braucht deshalb jeder neue Build von `a38` eine Bestätigung in einem Dialog von macOS ({{ ref("G5") }}).
    - Ein anderes Programm kann denselben Dialog auslösen, auch ein Agent. Der Dialog sieht dann gleich aus.
    - Bestätige nie einen Dialog für den Eintrag `a38.onepassword`, den du nicht selbst ausgelöst hast. Wähle im Zweifel „Verweigern“.
    - Der Schlüsselbund schützt nicht gegen das Auslesen des Speichers eines laufenden Daemons ohne Signatur ({{ ref("RI5") }}). Diesen Schutz bringt erst die Signatur mit Developer ID und Hardened Runtime.
    - Der Schutz gegen einen Prozess desselben Benutzers beginnt erst mit der Signatur mit Developer ID ({{ ref("G3") }}, {{ ref("G5") }}).

    Die Messungen dazu stehen im [Spike zum Schlüsselbund](../explanation/spikes/50-token-im-schluesselbund.md).

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

!!! danger "Das Token gehört nur in a38 token set"
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
Leite es direkt in `a38 token set`.
Das Token erscheint so nie im Terminal und liegt nie in einer Datei:

```sh
op service-account create a38-passierschein-dev \
  --vault passierschein.dev:read_items \
  --raw | a38 token set
```

`read_items` gibt nur das Recht zu lesen.
Die [CLI-Referenz](https://www.1password.dev/cli/reference/management-commands/service-account) nennt die weiteren Schalter, zum Beispiel `--expires-in`.
Speichere das Token zusätzlich in deinem eigenen Vault, falls du es noch einmal brauchst.

Geprüft am 2026-11-12: Ein Token aus `op service-account create … --raw`, damals in die Token-Datei geschrieben, funktioniert mit dem SDK v0.4.1.
Der Weg über `a38 token set` ist noch nicht mit einem echten Token geprüft.

## Token im Schlüsselbund ablegen

1. Kopiere das Token aus deinem Item in 1Password in die Zwischenablage.
2. Starte den Befehl in deinem Terminal:

    ```sh
    a38 token set
    ```

3. Füge das Token beim Prompt ein und drücke die Eingabetaste. Das Terminal zeigt das Token nicht an.
4. Leere die Zwischenablage. Jeder Prozess deines Benutzers kann sie lesen, solange das Token dort steht.

Die Ausgabe nennt den Eintrag, die Länge und den Fingerabdruck:

```text
Stored the token in the keychain item a38.onepassword/service-account: length 852 bytes, fingerprint 1a2b3c4d.
```

Der Fingerabdruck sind die ersten 8 Hex-Zeichen von SHA-256 des Tokens.
Er verrät das Token nicht.
Mit ihm erkennst du später, welches Token im Schlüsselbund liegt.

Gibt es den Eintrag schon, ändert `a38 token set` nichts und endet mit dem Exit-Code 1.
Ersetze ihn mit `a38 token set --replace`.

Prüfe den Eintrag:

```sh
a38 token status
```

Der Befehl zeigt den Eintrag ohne Wert, die vertrauten Programme und die Signatur von `a38`.
Hat genau dieser Build von `a38` den Eintrag angelegt, zeigt er auch Länge und Fingerabdruck.
Sonst liest er den Wert nicht, denn das Lesen würde einen Dialog von macOS zeigen.

`a38` legt den Eintrag selbst an, nicht über `/usr/bin/security`.
Über `security` angelegt, könnte jeder Prozess deines Benutzers das Token still mit `security find-generic-password -w` lesen.

## Konfiguration

Die Konfiguration nennt den Vault mit seiner Umgebung:

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
    token_source: keychain
```

`backend.onepassword.token_source` ist optional; der Standard ist `keychain`.
Die [Referenz](../reference/80-befehle-fuer-den-menschen.md#konfiguration-des-1password-backends) nennt alle Schlüssel.

## Daemon starten

Das echte Backend startet nur mit der Freigabe-App ({{ ref("D40") }}):

```sh
a38 serve --backend 1password --ui app
```

Der Daemon liest das Token einmal beim Start aus dem Schlüsselbund.
Danach hält er es nur im Arbeitsspeicher.
Eine Änderung im Schlüsselbund wirkt erst nach einem Neustart.

Fehlt der Eintrag, startet der Daemon nicht.
Die Meldung nennt den Befehl `a38 token set`.

Er verbindet sich erst beim ersten Aufruf mit 1Password.
Das Log meldet kurz nach dem Start den Zustand:

- `the 1password backend works`: Das Token gilt, und der Service Account sieht die Vaults.
- `the 1password backend does not work yet` mit `code=backend_locked`: 1Password lehnt das Token ab.
- `the 1password backend does not work yet` mit `code=backend_unavailable`: 1Password antwortet nicht, zum Beispiel ohne Netz.

Bei einem ungültigen Token oder ohne Netz läuft der Daemon weiter.
Jede Anfrage endet dann mit `backend_locked` oder `backend_unavailable`.
Der Daemon versucht es frühestens nach 30 Sekunden erneut.

### Dialog nach einem neuen Build

Hat ein anderer Build von `a38` den Eintrag angelegt, zeigt macOS beim Start des Daemons einen Dialog.
Der Dialog fragt, ob `a38` den Eintrag `a38.onepassword` lesen darf.
Der Daemon wartet höchstens zwei Minuten auf die Antwort; danach bricht der Start ab.

- Hast du gerade selbst `a38` neu gebaut oder aktualisiert und den Daemon gestartet, ist der Dialog erwartet.
- Statt den Dialog zu bestätigen, kannst du das Token für den neuen Build neu ablegen: `a38 token set --replace`. Ob macOS beim Ersetzen selbst nachfragt, klärt der Praxistest.
- Kommt der Dialog ohne einen Grund, den du kennst, wähle „Verweigern“. Prüfe dann mit `a38 doctor` und `a38 token status`.

`a38 doctor` und `a38 token status` sagen vorher, ob der laufende Build den Eintrag ohne Dialog liest.

## Mit a38 doctor prüfen

```sh
a38 doctor
```

- Die Zeile `1password token` prüft die Quelle des Tokens. Sie liest das Token nicht.
- Mit der Quelle `keychain` nennt sie den Eintrag, sein Datum und die vertrauten Programme.
- Eine Zeile `NOTE` nennt die Signatur von `a38`. Ohne Developer ID warnt sie vor dem Dialog nach jedem Build. Das ist kein Befund.
- Ein vertrautes Programm außer `a38`, zum Beispiel `/usr/bin/security`, ist ein Befund.
- Eine übrige Token-Datei ist ein Befund, wenn die Quelle der Schlüsselbund ist.
- Die Zeile `backend` zeigt den Zustand aus dem Daemon: `ok`, `locked` oder `unreachable`.

## Von der Token-Datei in den Schlüsselbund umziehen

Bis zu dieser Version lag das Token in der Token-Datei `~/.config/a38/onepassword-token`.

1. Lege das Token mit `a38 token set` im Schlüsselbund ab. Füge es aus 1Password ein, wie oben beschrieben.
2. Vergleiche die Fingerabdrücke:

    ```sh
    a38 token status
    ```

    Die Zeilen `Token` (Schlüsselbund) und `Token file` (Datei) zeigen denselben Fingerabdruck.

3. Starte den Daemon neu und prüfe mit `a38 doctor`.
4. Lösche die Token-Datei:

    ```sh
    rm ~/.config/a38/onepassword-token
    ```

5. Entferne `token_file` und `token_source: file` aus der Konfiguration, falls dort eingetragen.

`a38 token set < ~/.config/a38/onepassword-token` geht auch, denn der Befehl liest stdin.
Die Anleitung empfiehlt das Einfügen aus 1Password: So prüfst du das Token in 1Password, und die Datei hat danach keinen Zweck mehr.

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
    a38 token set --replace
    ```

5. Starte den Daemon neu. Er liest das Token nur beim Start. Der Neustart beendet alle Grants ({{ ref("D33") }}).
6. Prüfe mit `a38 token status` den neuen Fingerabdruck und mit `a38 doctor` den Zustand des Backends.

Ist das Token entwendet, widerrufe es sofort in 1Password.com.
Der Widerruf wirkt sofort.

## Token löschen

```sh
a38 token delete
```

Der Befehl löscht den Eintrag im Schlüsselbund.
Ein laufender Daemon behält das Token im Arbeitsspeicher, bis er endet.
Sein nächster Start mit dem 1Password-Backend scheitert, bis du `a38 token set` ausführst.

## Token-Datei als Ausweichweg für die Entwicklung

Für die Entwicklung ist die Token-Datei bequemer: Jeder Neubau von `a38` würde sonst einen Dialog auslösen.
Ihre Schwäche: Jeder Prozess deines Benutzers kann die Datei lesen, auch ein Agent auf dem Mac ({{ ref("RI5") }}, {{ ref("G3") }}).
Die Datei kann in eine Datensicherung gelangen.
Nutze sie nur mit einem Test-Vault.

Wähle die Quelle in der Konfiguration:

```yaml
backend:
  onepassword:
    token_source: file
    token_file: ~/.config/a38/onepassword-token
```

`token_file` ist optional.
Der Standardpfad ist `~/.config/a38/onepassword-token`, mit gesetztem `XDG_CONFIG_HOME` `$XDG_CONFIG_HOME/a38/onepassword-token`.

Lege die Datei so an, dass das Token nicht in der Historie der Shell landet:

```sh
mkdir -p ~/.config/a38
umask 077
pbpaste > ~/.config/a38/onepassword-token
chmod 600 ~/.config/a38/onepassword-token
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
- Die Tests von `a38` berühren den Eintrag `a38.onepassword` nie. Die Tests des Pakets `internal/keychain` legen nur Einträge `a38-test-…` mit Dummy-Werten an und löschen sie wieder.
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
