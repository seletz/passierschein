---
title: Mehrere 1Password-Konten einrichten
description: "Zwei oder mehr 1Password-Konten mit je einem Service Account in a38 eintragen, jedem Vault sein Konto geben, die Tokens je Konto ablegen, mit a38 doctor prüfen und eine Konfiguration mit einem Token umstellen."
render_macros: true
tags:
  - Thema/Backend
  - Thema/Betrieb
---
# Mehrere 1Password-Konten einrichten

Diese Anleitung richtet `a38` für mehrere 1Password-Konten ein ({{ ref("F25") }}).
Ein Service Account gehört zu genau einem Konto.
Jedes Konto braucht deshalb einen eigenen Service Account und ein eigenes Token.
Die Beispiele nutzen die Konten `privat` und `firma`.

Die Regeln:

- Die Konfiguration nennt die Konten unter `backend.onepassword.accounts` ({{ ref("D71") }}).
- Jeder Vault nennt sein Konto. Es gibt kein Standardkonto ({{ ref("D70") }}).
- Jedes Konto hat einen eigenen Eintrag im Schlüsselbund ({{ ref("D72") }}).
- Ein Konto ohne Token sperrt nur seine Vaults ({{ ref("D73") }}).

Den Service Account eines Kontos legst du an, wie [1Password-Backend einrichten](30-1password-backend-einrichten.md#service-account-anlegen) es beschreibt.
Melde dich dafür in 1Password.com bei dem Konto an, zu dem der Service Account gehören soll.

## Konten eintragen

Trage jedes Konto ein:

```sh
a38 config account add privat
a38 config account add firma
```

Der Name ist frei gewählt.
Er besteht aus Kleinbuchstaben, Ziffern und Bindestrich, mit 1 bis 32 Zeichen.
Der Name `service-account` ist gesperrt.

Für ein Konto der Entwicklung kannst du die Token-Datei wählen:

```sh
a38 config account add firma --token-source file
```

Der Standardpfad der Datei ist `~/.config/a38/onepassword-token-firma`.
Einen anderen Pfad nennt `--token-file`.

## Vaults mit Konto eintragen

Jeder Vault nennt sein Konto:

```sh
a38 config vault add beispiel.app.k8s-dev --environment dev --account privat
a38 config vault add datenbank-dev --environment dev --account firma
```

Ohne `--account` endet der Befehl mit dem Exit-Code 2.
Ein Konto, das nicht in der Liste steht, ist ein Fehler.

Die Datei sieht danach so aus:

```yaml
vaults:
  - name: beispiel.app.k8s-dev
    environment: dev
    account: privat
  - name: datenbank-dev
    environment: dev
    account: firma
backend:
  onepassword:
    accounts:
      - name: privat
        token_source: keychain
      - name: firma
        token_source: keychain
```

Der Name eines Vaults ist über alle Konten eindeutig.
Haben zwei Konten einen Vault mit demselben Titel, benenne einen davon in 1Password um ({{ ref("G41") }}).
Die Referenz des Agents bleibt `op://<vault>/<item>/<feld>`; sie nennt kein Konto.

Prüfe die Datei:

```sh
a38 config validate
a38 config account list
```

## Tokens ablegen

Lege das Token jedes Kontos in deinem Terminal ab:

```sh
a38 config onepassword-token set --account privat
a38 config onepassword-token set --account firma
```

Der Befehl fragt nach dem Token des genannten Kontos und liest es ohne Echo.
Er liest nur vom Terminal, nie aus einer Pipe oder einer Datei.
Steht das Konto nicht in der Konfiguration, liest er nichts und endet mit dem Exit-Code 1.

Die Ausgabe nennt nur Konto und Eintrag:

```text
Stored the token of the account "privat" in the keychain item a38.onepassword/privat.
```

Prüfe alle Konten:

```sh
a38 config onepassword-token status
```

Der Befehl zeigt je Konto die Vaults, die Quelle, den Eintrag und die Token-Datei.
Er liest kein Token.

## Daemon starten und prüfen

Starte den Daemon neu.
Er liest jedes Token einmal beim Start.

- Fehlt das Token eines Kontos, startet der Daemon trotzdem. Jede Anfrage an einen Vault dieses Kontos endet mit `backend_locked`. Das Log nennt das Konto und den nächsten Schritt.
- Fehlen alle Tokens, startet der Daemon nicht. Die Meldung nennt jedes Konto.

Das Log meldet kurz nach dem Start je Konto `the 1password account works` mit `account=<name>`.

Prüfe mit `a38 doctor`.
Die Zeile `1password token` erscheint je Konto, mit seinen Vaults und seiner Quelle.

!!! note "Dialoge nach einem neuen Build"
    Ohne Signatur mit Developer ID braucht jeder neue Build von `a38` einen Dialog von macOS ({{ ref("G5") }}).
    Mit zwei Konten kommen zwei Dialoge, einer je Eintrag.
    Lege die Tokens stattdessen neu ab: `a38 config onepassword-token set --account <name> --replace`.

## Von einem Token auf Konten umstellen

Eine Konfiguration mit `token_source` oder `token_file` direkt unter `backend.onepassword` lädt nicht mehr ({{ ref("D75") }}).
Ein Vault ohne `account` lädt ebenfalls nicht.
Der Daemon startet dann nicht, und `a38 config validate` nennt den Befund:

```text
backend.onepassword.token_source: this key is from the configuration with one token and is no longer valid; each account names its own token source. Next step: move the key into an account under backend.onepassword.accounts ...
vault "beispiel.app.k8s-dev": the account is missing; each vault names its 1Password account (account: NAME), and NAME must be in backend.onepassword.accounts
```

So stellst du um:

1. Öffne die Konfiguration (`a38 config path` nennt die Datei).
2. Ersetze den Block unter `backend.onepassword` durch ein Konto. Aus

    ```yaml
    backend:
      onepassword:
        token_source: keychain
    ```

    wird

    ```yaml
    backend:
      onepassword:
        accounts:
          - name: privat
            token_source: keychain
    ```

    Stand dort `token_source: file` mit `token_file`, ziehe beide Schlüssel in das Konto um.
3. Ergänze an jedem Vault `account: privat`.
4. Prüfe mit `a38 config validate`.
5. Lege das Token neu ab: `a38 config onepassword-token set --account privat`. Der alte Eintrag wird nicht übernommen. Füge das Token aus deinem Item in 1Password ein.
6. Starte den Daemon neu und prüfe mit `a38 doctor`.
7. Lösche den alten Eintrag:

    ```sh
    a38 config onepassword-token delete --legacy
    ```

    `a38 doctor` meldet ihn bis dahin als Befund.
8. Liegt noch die alte Token-Datei `~/.config/a38/onepassword-token` herum, löscht du sie oder ziehst sie in die Token-Datei eines Kontos um. `a38 doctor` meldet sie als Befund.

## Konto entfernen

1. Entferne zuerst die Vaults des Kontos: `a38 config vault remove <vault>`. Ein Projekt, das einen Vault nutzt, verhindert das; entferne den Vault vorher aus dem Projekt.
2. Entferne das Konto: `a38 config account remove firma`.
3. Lösche den Eintrag im Schlüsselbund: `a38 config onepassword-token delete --account firma`.
4. Widerrufe den Service Account in 1Password.com, wenn du ihn nicht mehr brauchst.
5. Starte den Daemon neu.

## Grenzen

- Freigabeanfrage, Audit-Log und `a38 tui` nennen das Konto noch nicht ({{ ref("F26") }}).
- Der Status des Daemons nennt den Zustand nicht je Konto ({{ ref("G45") }}). Lehnt 1Password das Token eines Kontos ab, zeigt das nur das Log des Daemons.
