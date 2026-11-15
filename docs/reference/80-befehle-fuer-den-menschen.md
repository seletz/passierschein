---
title: Befehle für den Menschen
description: "a38 grants, fault, config, service, doctor, audit und tui: Argumente, Ausgabe und Exit-Codes."
render_macros: true
tags:
  - Thema/Betrieb
---
# Befehle für den Menschen

Diese Befehle bedient der Mensch.
Ein Agent hat keinen Zugang zu ihnen: Die Befehle `grants`, `fault` und `doctor` sprechen mit dem Daemon über den Admin-Socket.
Der Agent-Socket lehnt die Nachrichten des Admin-Sockets ab.
Ein Befehl zeigt nie einen Secret-Wert.

Alle Befehle lesen die Konfigurationsdatei mit `--config <datei>`.
Ohne das Argument gilt `A38_CONFIG`, danach der Standardort.
Die Befehle `grants`, `fault`, `doctor`, `config path`, `config show`, `config validate`, `config account list`, `config vault list`, `config project list` und `config onepassword-token status` kennen `--json`.

Die Schalter dürfen vor und nach den Argumenten stehen.
`a38 grants revoke <grant-id> --config <datei>` und `a38 grants revoke --config <datei> <grant-id>` sind gleich.
Ein `--` beendet die Schalter.

## Exit-Codes

| Code | Bedeutung |
|---|---|
| 0 | Der Befehl ist gelungen. |
| 1 | Der Befehl ist gescheitert, oder `doctor` hat einen Befund. |
| 2 | Der Aufruf ist falsch. |

Läuft der Daemon nicht, nennen `grants` und `fault` den Grund und enden mit 1.

## a38 serve

`a38 serve` startet den Daemon.
Der Mensch startet ihn, nicht der Agent.

| Schalter | Bedeutung |
|---|---|
| `--config <datei>` | Konfigurationsdatei, siehe oben. |
| `--backend fake\|1password` | Das Backend. Ein echtes Backend verlangt `--ui app` ({{ ref("D40") }}). `1password` gibt es nur im macOS-Build; unter Linux bricht der Start mit „not available on this platform“ ab ({{ ref("D42") }}). |
| `--fake-file <datei>` | YAML-Datei des Fake-Backends. Pflicht mit `--backend fake`. |
| `--ui app\|auto-approve\|auto-deny\|script` | Die Freigabe. `app` startet die Freigabe-App. Die anderen Werte sind nur mit dem Fake-Backend erlaubt. `app` mit dem Fake-Backend ist erlaubt, um die Oberfläche zu testen. |
| `--ui-script <befehl>` | Programm der Freigabe mit `--ui script`. |
| `--ui-app <pfad>` | Pfad der Freigabe-App mit `--ui app`. |

### Konfiguration des 1Password-Backends

Die Konfiguration nennt die 1Password-Konten ({{ ref("D71") }}).
Jedes Konto hat einen eigenen Service Account und ein eigenes Token.
Jeder Vault nennt sein Konto ({{ ref("D70") }}); es gibt kein Standardkonto.

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
      - name: firma
        token_source: file
        token_file: ~/.config/a38/onepassword-token-firma
```

| Schlüssel | Standard | Bedeutung |
|---|---|---|
| `backend.onepassword.accounts[].name` | – (Pflicht) | Name des Kontos in `a38`. Kleinbuchstaben, Ziffern und Bindestrich, 1 bis 32 Zeichen, am Anfang ein Buchstabe oder eine Ziffer. Jeder Name nur einmal. `service-account` ist gesperrt. |
| `backend.onepassword.accounts[].token_source` | `keychain` | Quelle des Tokens: `keychain` (Schlüsselbund von macOS) oder `file` (Token-Datei). Ein anderer Wert verhindert das Laden der Konfiguration. |
| `backend.onepassword.accounts[].token_file` | `$XDG_CONFIG_HOME/a38/onepassword-token-<name>`, ohne die Variable `~/.config/a38/onepassword-token-<name>` | Token-Datei des Kontos. Nur `--backend 1password` mit `token_source: file` liest sie. Zwei Konten mit derselben Datei sind ein Fehler. |
| `vaults[].account` | – (Pflicht) | Name des Kontos des Vaults. Das Konto muss in `accounts` stehen. |

Ein Konto ohne Vault ist erlaubt; der Daemon liest sein Token nicht, und `a38 doctor` nennt es als Hinweis.
Der Name eines Vaults ist über alle Konten eindeutig ({{ ref("G41") }}).
Die Referenz des Agents nennt kein Konto; der Daemon findet das Konto über den Vault.

Die Schlüssel `backend.onepassword.token_source` und `backend.onepassword.token_file` direkt unter `onepassword` stammen aus der Konfiguration mit einem Token.
Sie verhindern das Laden ({{ ref("D75") }}).
Die Meldung nennt den Umbau; die Schritte stehen in [Mehrere 1Password-Konten einrichten](../how-to/31-mehrere-1password-konten.md#von-einem-token-auf-konten-umstellen).

Die Quelle `keychain` ist der Standard ({{ ref("D72") }}).
Der Daemon liest dann den Eintrag mit dem Dienst `a38.onepassword` und dem Konto `<name>` im Anmelde-Schlüsselbund.
`a38 config onepassword-token set --account <name>` legt ihn an (siehe [a38 config](#a38-config)).
Wartet der Schlüsselbund länger als zwei Minuten, zum Beispiel auf einen Dialog von macOS, gilt das Token des Kontos als fehlend.

Die Quelle `file` ist der Ausweichweg für Entwicklung und CI.
Die Token-Datei ist eine reguläre Datei, gehört dem Benutzer und hat genau den Modus `0600`.
Jeder Prozess desselben Benutzers kann die Datei lesen, auch ein Agent ({{ ref("RI5") }}).
`A38_CONFIG` verschiebt die Token-Datei nicht.

Beim Start ({{ ref("D73") }}):

- Der Daemon liest das Token jedes Kontos einmal, in der Reihenfolge der Konfiguration. Ein Konto ohne Vault liest er nicht.
- Fehlt das Token eines Kontos, startet der Daemon trotzdem. Jede Anfrage an einen Vault dieses Kontos endet mit `backend_locked`. Das Log nennt das Konto und den nächsten Schritt.
- Hat kein Konto ein Token, startet der Daemon nicht. Die Meldung nennt jedes Konto mit Befund und nächstem Schritt.
- Das Backend hält einen Client des SDK je Konto. Lehnt 1Password das Token eines Kontos ab, sperrt das nur die Vaults dieses Kontos.
- Eine Änderung eines Tokens wirkt erst nach einem Neustart des Daemons.
- Ein ungültiges Token oder ein fehlendes Netz verhindern den Start nicht: Jede Anfrage an das Konto endet dann mit `backend_locked` oder `backend_unavailable`.

Die Einrichtung beschreiben [1Password-Backend einrichten](../how-to/30-1password-backend-einrichten.md) und [Mehrere 1Password-Konten einrichten](../how-to/31-mehrere-1password-konten.md).

### Umgebungen und exec-Weg

Die Abschnitte `environments` und `exec` setzen Weg, Laufzeit und Bindung je Umgebung und die Grenzen des exec-Wegs ({{ ref("D66") }}, {{ ref("F22") }}).
Die Versionsnummer bleibt `1`. Jeder Schlüssel hat einen Standard.

```yaml
environments:
  dev:
    delivery: [mcp, exec]
    grant_lifetime: 4h
    binding: none
  test:
    delivery: [exec]
    grant_lifetime: 30m
    binding: program
  prod:
    delivery: []
    grant_lifetime: 0s
    binding: command
exec:
  default_timeout: 2m
  max_timeout: 10m
  output_limit: 64KiB
```

| Schlüssel | Standard | Erlaubte Werte |
|---|---|---|
| `environments.<env>.delivery` | dev `[mcp, exec]`, test `[exec]`, prod `[]` | Liste aus `mcp` und `exec`, ohne Wiederholung. `mcp` nur bei dev ({{ ref("D62") }}). Die leere Liste schaltet die Umgebung aus. |
| `environments.<env>.grant_lifetime` | dev `4h`, test `30m`, prod `0s` | Dauer von `0s` bis `24h`. Bei prod nur `0s`. `0s` heißt: Ein Grant gilt für genau eine Auslieferung ({{ ref("D63") }}). |
| `environments.<env>.binding` | dev `none`, test `program`, prod `command` | `none`, `program` oder `command`. Bei test nicht `none`. Bei prod nur `command`. |
| `exec.default_timeout` | `2m` | Dauer größer als 0, höchstens `exec.max_timeout`. |
| `exec.max_timeout` | `10m` | Dauer größer als 0, höchstens `1h`. |
| `exec.output_limit` | `64KiB` | Größe von `1KiB` bis `1MiB` mit der Einheit `KiB` oder `MiB`; gilt je Strom ({{ ref("D64") }}). |

`<env>` ist `dev`, `test` oder `prod`. Eine andere Umgebung unter `environments` ist ein unbekannter Schlüssel.
Die festen Regeln stehen im Code. Die Konfiguration kann sie nicht lockern.
Der Mensch kann prod einschalten (`delivery: [exec]`) und dev und test enger fassen.
Ein Verstoß verhindert das Laden; die Meldung nennt den Schlüssel.

`grant.lifetime` ist veraltet ({{ ref("D32") }}).
Der Schlüssel setzt weiter die Laufzeit von dev.
Zusammen mit `environments.dev.grant_lifetime` ist er ein Fehler.
`a38 config validate` und `a38 doctor` nennen ihn als veraltet (Zeile `NOTE`, Exit-Code bleibt 0) und nennen den neuen Schlüssel.
`a38 config show` führt `grant.lifetime` nur auf, wenn die Datei ihn setzt.

Die Anzeige je Umgebung in `a38 doctor` und `a38 tui` folgt in einem eigenen Schritt ({{ ref("F22") }}).
Der Broker wertet die Schlüssel mit den Methoden `Policy`, `DeliveryAllowed`, `GrantLifetime` und `Binding` der Konfiguration und mit dem Feld `Exec` aus.

### Obergrenze eines Bündels

Der Schlüssel `bundle.max_entries` der Konfiguration legt fest, wie viele Einträge ein Bündel höchstens hat.
Der Standard ist 10.
Der größte erlaubte Wert ist 50.
Die Freigabe-App lehnt eine Anfrage mit mehr als 50 Einträgen ab.
`a38 serve` und jeder andere Befehl lehnen die Konfiguration mit einem größeren Wert beim Laden ab.
In Go steht die Grenze in `config.MaxBundleEntries`, in Swift in `app/Sources/A38ApprovalCore/Decoding.swift`.

### Ort der Freigabe-App

Der Daemon sucht den Pfad in dieser Reihenfolge:

1. Schalter `--ui-app`. Ein relativer Pfad gilt ab dem Arbeitsverzeichnis von `a38 serve`.
2. Schlüssel `paths.ui_app` der Konfigurationsdatei.
3. Der Daemon löst die symbolischen Links seines eigenen Binarys auf. Danach sucht er der Reihe nach:
    1. Läuft `a38` in einem Bundle (`<X>.app/Contents/MacOS/a38`), das Geschwister `<X>.app/Contents/MacOS/Passierschein`.
    2. Neben `a38`: `Passierschein.app/Contents/MacOS/Passierschein`, danach eine einfache ausführbare Datei `Passierschein`.
    3. `/Applications/Passierschein.app/Contents/MacOS/Passierschein`, danach `~/Applications/Passierschein.app/Contents/MacOS/Passierschein`.

Der erste vorhandene Treffer gilt.
Die Prüfung der Datei unten gilt für jeden Treffer.
Ein Bundle in `/Applications` gehört meist dem Benutzer oder `root`; beides ist erlaubt.
Der Daemon schreibt beim Start den gewählten Pfad ins Log.
Die App heißt „Passierschein“, das Werkzeug `a38` ({{ ref("D50") }}).

### Auslieferung

Das Release kann `a38` im Bundle `Passierschein.app` ausliefern (`Contents/MacOS/a38`) und einen symbolischen Link in den `PATH` legen.
Der Linux-Build bleibt das einzelne Binary `a38` ohne App.
Die Paketierung gehört zur Arbeit am Release ({{ ref("G5") }}, {{ ref("D46") }}) und nicht zu diesem Stand.

### Prüfung vor jedem Start

Der Daemon prüft die Datei vor jedem Start der App ({{ ref("D6") }}, {{ ref("D40") }}):

- Die Datei existiert und ist eine reguläre Datei. Ein symbolischer Link gilt als sein Ziel.
- Die Datei gehört dem Benutzer des Daemons oder `root`.
- Weder die Gruppe noch andere dürfen schreiben.
- Der Besitzer darf die Datei ausführen.
- Die Datei hat kein Setuid- und kein Setgid-Bit.

Besteht die Datei eine Prüfung nicht, startet der Daemon die App nicht.
Die Anfrage endet mit `ui_failed`, und der Daemon schreibt eine Zeile ins Log.
Der Daemon selbst startet trotzdem.
`a38 doctor` meldet den Befund in der Prüfung `approval UI`.
Die Prüfung der Code-Signatur fehlt noch ({{ ref("G5") }}).

## a38 service

Der Befehl richtet den Daemon als LaunchAgent ein ({{ ref("F15") }}, {{ ref("D56") }}).
Nur der macOS-Build kennt ihn.
Unter Linux endet jeder Unterbefehl mit „not available on this platform“ und dem Exit-Code 1 ({{ ref("D53") }}).
Die Schritte stehen in [Daemon als Dienst einrichten](../how-to/28-daemon-als-dienst-einrichten.md).

| Aufruf | Wirkung |
|---|---|
| `a38 service install` | Prüft, ob der Daemon starten kann, schreibt die plist und lädt den Dienst (`launchctl enable`, `launchctl bootstrap gui/<uid>`). Ein zweiter Aufruf mit denselben Angaben ändert nichts. |
| `a38 service uninstall [--force]` | Beendet den Daemon (`launchctl bootout`) und entfernt die plist. Das Log bleibt. |
| `a38 service restart` | Startet den Daemon neu (`launchctl kickstart -k`). Ist der Dienst nicht geladen, lädt der Befehl ihn. |
| `a38 service status [--config <datei>] [--json]` | Zeigt plist, Programm, Zustand bei launchd, letzten Exit-Code, letzten Grund des Abbruchs, Log und die Antwort des Daemons. Exit-Code 0, wenn der Dienst läuft und das Programm passt, sonst 1. |

Schalter von `install`:

| Schalter | Bedeutung |
|---|---|
| `--backend 1password\|fake` | Backend des Daemons. Standard: `1password`. |
| `--ui app\|auto-approve\|auto-deny` | Freigabe. Standard: `app`. Die anderen Werte nur mit `--backend fake` ({{ ref("D40") }}). |
| `--fake-file <datei>` | YAML-Datei des Fake-Backends. Pflicht mit `--backend fake`. |
| `--ui-app <pfad>` | Pfad der Freigabe-App. Ohne den Schalter gilt die Suche von `a38 serve`. |
| `--config <datei>` | Konfigurationsdatei. Nur mit diesem Schalter oder mit `A38_CONFIG` steht `--config` in der plist. |
| `--force` | Ersetzt eine plist, die `a38` nicht geschrieben hat. |
| `--dry-run` | Prüft und zeigt die plist. Ändert nichts. |

Die plist:

| Schlüssel | Wert |
|---|---|
| `Label` | `io.github.seletz.passierschein.daemon` |
| `ProgramArguments` | echter Pfad des aufrufenden `a38` (Links aufgelöst), dann `serve --backend … --ui …` |
| `RunAtLoad` | `true` |
| `KeepAlive` | `false`: launchd startet den Daemon nie von selbst neu. |
| `LimitLoadToSessionType` | `Aqua`: nur in der Sitzung am Bildschirm |
| `ProcessType` | `Interactive` |
| `AssociatedBundleIdentifiers` | `io.github.seletz.passierschein` |
| `ExitTimeOut` | 20 Sekunden zwischen SIGTERM und SIGKILL |
| `WorkingDirectory` | `/` |
| `Umask` | `077` |
| `StandardOutPath`, `StandardErrorPath` | `~/Library/Logs/a38/serve.log` |
| `EnvironmentVariables` | nur `XDG_CONFIG_HOME` und `XDG_STATE_HOME`, wenn die Shell sie setzt; nie ein Secret |

Ein Kommentar in der plist kennzeichnet sie als Datei von `a38`.
Fehlt er, ändern `install` und `uninstall` die Datei nur mit `--force`.
Alle Aufrufe von launchctl laufen über `/bin/launchctl`, ohne Shell und mit einer Zeitgrenze von 15 Sekunden.

Exit-Codes von `a38 serve` unter launchd:

| Code | Bedeutung | Neustart durch launchd |
|---|---|---|
| 0 | normales Ende nach SIGTERM (Abmeldung, `uninstall`, `restart`) | nein |
| 1 | Fehler beim Start oder im Betrieb: Konfiguration, Quelle des Tokens, Verzeichnis der Sockets, Audit-Log; das Log nennt die Ursache | nein |
| 2 | falscher Aufruf in der plist oder Absturz der Go-Laufzeit | nein |
| Signal | Absturz oder Ende von außen | nein |

Nennt launchd einen Grund des Abbruchs (`last exit reason` in `launchctl print`), zeigt `a38 service status` ihn mit an.
Mit `--json` steht er im Feld `last_exit.Reason`.
Ein Abbruch durch den Kernel hat oft nur den Grund, keinen Exit-Code und kein Signal.
Der Grund `OS_REASON_CODESIGNING` heißt: macOS hat den Daemon wegen seiner Signatur beendet.
Die Hardened Runtime der Signatur verbietet Code, den `a38` ausführt ({{ ref("D59") }}).
Der nächste Schritt ist `a38 selftest sdk-core` mit dem `a38` des Dienstes ([Störung finden und beheben](../how-to/24-stoerung-finden-und-beheben.md#daemon-wegen-der-signatur-beendet)).

## a38 grants

| Aufruf | Wirkung |
|---|---|
| `a38 grants list` | Zeigt die Grants aller Sitzungen. |
| `a38 grants revoke <grant-id>` | Beendet einen Grant. Die nächste Anfrage braucht eine neue Freigabe. |

`list` zeigt eine Tabelle mit diesen Spalten:

| Spalte | Inhalt |
|---|---|
| `GRANT` | Kennung des Grants |
| `SESSION` | die ersten acht Zeichen der Kennung der Sitzung |
| `NAME` | Name der Sitzung |
| `PROJECT` | Projekt der Sitzung |
| `CREDENTIALS` | Referenzen, nie ein Wert |
| `REMAINING` | Restlaufzeit |

Ohne Grants steht `No active grants.` in der Ausgabe.
Mit `--json` gibt `list` ein JSON-Array mit den vollen Angaben aus, auch mit der ganzen Kennung der Sitzung.
Ein unbekannter Grant endet mit einer Meldung und dem Exit-Code 1.
Der Widerruf steht als Ereignis `revocation` im Audit-Log.

## a38 fault

Der Befehl bedient die globale Sperre der Auslieferung (siehe [Störung finden und beheben](../how-to/24-stoerung-finden-und-beheben.md)).

| Aufruf | Wirkung |
|---|---|
| `a38 fault status` | Zeigt, ob die Auslieferung gesperrt ist, mit Ursache und Beginn, soweit der Daemon sie meldet. Exit-Code 1, solange die Sperre besteht. |
| `a38 fault clear` | Bittet den Daemon, die Sperre aufzuheben. Der Daemon hebt sie nur auf, wenn die Ursache behoben ist. Sonst steht der Befund in der Ausgabe, und der Exit-Code ist 1. |

Die Namen sind eine Wahl von Issue 25 und warten auf die Bestätigung durch Stefan.

## a38 config

`a38 config` fasst die Befehle für die Konfiguration zusammen.
`a38 config --help` listet die Unterbefehle.
Jeder Unterbefehl kennt `--help` und `--config <datei>`.
Eine Anleitung steht in [Konfiguration anlegen und ändern](../how-to/26-konfiguration-anlegen-und-aendern.md).
Die Entscheidung dazu ist {{ ref("D55") }}.

| Unterbefehl | Wirkung |
|---|---|
| `path` | Zeigt die Datei, die gilt, den Grund (Schalter, `A38_CONFIG`, `XDG_CONFIG_HOME`, Standard) und ob sie existiert. Exit-Code 0. |
| `show` | Zeigt jeden Wert mit der Herkunft (`file`, `default`), auch die Schlüssel von `environments` und `exec`, die aufgelösten Pfade (Sockets, Audit-Datei), die Konten mit ihrer Quelle des Tokens, die vorhandenen Vaults mit Umgebung und Konto und die Projekte mit Pfaden (geschrieben, aufgelöst, fehlend) und die Prüfung. Eine Datei, die nicht lädt, gibt den Fehler mit Zeile und Exit-Code 1, nie eine Teilausgabe. Der Befehl liest das Token nie. |
| `init [--force]` | Legt die Datei mit den Standardwerten (auch `environments` und `exec`) und Kommentaren an (Verzeichnis 0700, Datei 0600). Eine vorhandene Datei bleibt; der Befehl endet mit 1. `--force` verschiebt sie nach `config.yaml.bak-<Zeitstempel>`. |
| `validate` | Lädt und prüft die Datei wie die Prüfung `configuration` von `a38 doctor`. Exit-Code 0 ohne Befund, sonst 1 mit dem nächsten Schritt. |
| `account add <name> [--token-source keychain\|file] [--token-file <pfad>]` | Trägt ein 1Password-Konto ein ({{ ref("D71") }}). `--token-file` gilt nur mit der Quelle `file`. Ein doppelter oder ungültiger Name ist ein Fehler. Die Ausgabe nennt den nächsten Schritt `a38 config onepassword-token set --account <name>`. |
| `account remove <name>` | Entfernt ein Konto. Nennt ein Vault das Konto, endet der Befehl mit 1 und nennt die Vaults. Der Eintrag im Schlüsselbund bleibt. |
| `account list` | Listet die Konten mit Quelle und Vaults. |
| `vault add <name> --environment dev\|test\|prod --account <konto>` | Trägt einen Vault ein. Ein doppelter Name, eine andere Umgebung oder ein Konto, das nicht in der Liste steht, ist ein Fehler. Ohne `--account` endet der Befehl mit 2. |
| `vault remove <name>` | Entfernt einen Vault. Nutzt ihn ein Projekt, endet der Befehl mit 1 und nennt die Projekte. |
| `vault list` | Listet die Vaults mit Umgebung und Konto. |
| `project add <name> --path <dir> [--path …] --vault <vault> [--vault …]` | Trägt ein Projekt ein. Jeder Pfad ist absolut oder beginnt mit `~/`. Jeder Vault steht in der Liste. Ein fehlender Pfad ist ein Hinweis. |
| `project remove <name>` | Entfernt ein Projekt. |
| `project list` | Listet die Projekte. |
| `onepassword-token` | Siehe unten. |

Die Befehle `account add|remove`, `vault add|remove` und `project add|remove` ändern nur den Block der Liste.
Kommentare, Reihenfolge und Leerzeilen bleiben erhalten; ein Kommentar über einem entfernten Eintrag bleibt stehen.
Die Änderung durchläuft vor dem Schreiben dieselben Regeln wie das Laden.
Eine Datei, die nicht lädt, ändern sie nicht.
Sie schreiben eine temporäre Datei im selben Verzeichnis und benennen sie um; der Modus der Datei bleibt.
Sie schreiben nicht durch einen symbolischen Link.
Eine Sperrdatei `<datei>.lock` und ein Vergleich vor dem Umbenennen verhindern gleichzeitige Änderungen.
Nach der Änderung weisen sie darauf hin, dass der Daemon die Datei nur beim Start liest und neu starten muss.
Ein Schalter darf vor und nach den Argumenten stehen.

### a38 config onepassword-token

Der Befehl verwaltet das Token des Service Accounts jedes 1Password-Kontos im Schlüsselbund von macOS ({{ ref("D72") }}).
Jedes Konto hat einen eigenen Eintrag: Dienst `a38.onepassword`, Konto `<name>`.
Nur der macOS-Build kennt ihn.
Unter Linux endet jeder Unterbefehl mit „not available on this platform“ und dem Exit-Code 1.

| Aufruf | Wirkung |
|---|---|
| `a38 config onepassword-token set --account <name> [--replace]` | Liest das Token vom Terminal und legt es im Eintrag des Kontos ab. Das Konto muss in der Konfiguration stehen; sonst liest der Befehl nichts und endet mit 1. Ohne `--account` endet er mit 2. Ein vorhandener Eintrag bleibt ohne `--replace` unverändert; der Befehl endet dann mit 1. |
| `a38 config onepassword-token status [--account <name>] [--json]` | Zeigt je Konto Vaults, Quelle, die Metadaten des Eintrags und die Token-Datei; dazu den alten Eintrag und die alte Token-Datei der Konfiguration mit einem Token und die Signatur von `a38`. Mit `--account` nur dieses Konto. Exit-Code 1, wenn ein Konto mit Vault kein Token hat oder die Konfiguration nicht lädt. |
| `a38 config onepassword-token delete --account <name>` | Löscht den Eintrag des Kontos. Das Konto muss nicht in der Konfiguration stehen. Die Ausgabe nennt, was der Daemon beim nächsten Start tut. |
| `a38 config onepassword-token delete --legacy` | Löscht den alten Eintrag `a38.onepassword/service-account` der Konfiguration mit einem Token ({{ ref("D75") }}). Der Daemon liest ihn nicht mehr. |

Alle Aufrufe kennen `--config <datei>`.
Kein Unterbefehl zeigt das Token oder einen Wert, der daraus abgeleitet ist (Länge, Prüfsumme).

`set`:

- Die Standardeingabe muss ein Terminal sein. Der Befehl fragt auf stderr nach dem Token des genannten Kontos und liest ohne Echo.
- Ist die Standardeingabe kein Terminal, zum Beispiel eine Pipe oder eine Datei, liest der Befehl nichts und endet mit dem Exit-Code 2.
- Das Token ist nie ein Argument und nie eine Umgebungsvariable. Ein Argument ist ein Aufruffehler (Exit-Code 2); der Befehl wiederholt es nicht.
- Die Prüfung der Form: nicht leer, mindestens 16 und höchstens 4096 Bytes, kein Leerzeichen, kein Steuerzeichen. Ein zu kurzes Token endet mit „too short“. Ein Token ohne den Anfang `ops_` ergibt nur eine Warnung, denn dieser Anfang ist eine Annahme.
- Die Ausgabe nennt nur Konto und Eintrag.
- `a38` legt den Eintrag selbst an, nicht über `/usr/bin/security`. Der Schlüsselbund vertraut dann nur diesem Build von `a38`.
- `--replace` löscht den alten Eintrag und legt ihn neu an. Danach vertraut der Eintrag dem neuen Build.
- Nutzt das Konto die Quelle `file`, legt der Befehl den Eintrag trotzdem an und weist darauf hin, dass der Daemon ihn nicht liest.

`status`:

- Der Befehl liest kein Token. Er zeigt deshalb nie einen Dialog von macOS.
- Je Konto: die Zeile `Account` mit den Vaults, die Zeile `Source`, die Zeile `Keychain` mit Eintrag, Datum der Anlage und der letzten Änderung, vertrauten Programmen und Partitionsliste, und die Zeile `Token file`.
- Nennt die Partitionsliste nicht den Hash dieses Builds (`cdhash:`), folgt ein Hinweis: Der nächste Start des Daemons zeigt einen Dialog.
- Die Zeile `Token file` nennt die Token-Datei des Kontos. Der Befehl prüft nur Besitzer und Rechte und liest den Inhalt nicht. Ist die Quelle der Schlüsselbund, ist eine übrige Datei ein Befund.
- Die Zeilen `Old item` und `Old file` nennen den Eintrag und die Token-Datei der Konfiguration mit einem Token, wenn es sie noch gibt, mit dem nächsten Schritt.
- Die Zeile `Signature` nennt die Art der Signatur von `a38`: `unsigned`, `ad-hoc`, `developer-id` oder `other-certificate`. Ohne Developer ID folgt eine Warnung ({{ ref("G5") }}).

## a38 doctor

`a38 doctor` führt die Prüfungen aus.
Jede Prüfung steht als Zeile `OK`, `NOTE` oder `FINDING`.
Eine Zeile `NOTE` nennt eine bekannte Grenze und ist kein Befund.
Ein Befund nennt die Ursache und den nächsten Schritt.
Der Exit-Code ist 0 ohne Befund.

| Prüfung | Inhalt |
|---|---|
| `configuration` | Die Konfiguration lädt. Je ungültigem Projekt folgt ein Befund mit dem Namen des Projekts. Ein veralteter Schlüssel (`grant.lifetime`) steht als Zeile `NOTE` mit dem neuen Schlüssel. |
| `socket modes` | Das Verzeichnis hat den Modus 0700, die Sockets 0600. |
| `service` | Nur macOS. Der LaunchAgent: nicht eingerichtet (Zeile `OK`, kein Befund), eingerichtet und läuft, eingerichtet und läuft nicht (Befund mit letztem Exit-Code, Grund des Abbruchs und Log), von macOS wegen der Signatur beendet (`OS_REASON_CODESIGNING`, eigener Befund mit `a38 selftest sdk-core` als nächstem Schritt), plist nennt ein fehlendes oder ein anderes `a38` (Befund), plist nicht von `a38` (Befund), abgeschaltet (Befund). |
| `daemon` | Der Daemon antwortet auf dem Admin-Socket. Die Zeile nennt Version, Sitzungen und Grants. |
| `delivery lock` | Zustand der Sperre mit Ursache und Beginn. Nächster Schritt: `a38 fault clear`. |
| `audit file` | Die Datei existiert und ist schreibbar, das Verzeichnis ebenfalls. |
| `audit space` | Im Verzeichnis sind mindestens 100 MiB frei. |
| `audit chain` | Die Hash-Kette stimmt. Die Prüfung liest die Datei nur. |
| `backend` | Zustand des Backends, soweit der Daemon ihn meldet: `ok`, `locked` oder `unreachable`. Für 1Password heißt `locked`: 1Password lehnt das Token ab. |
| `1password token` | Nur macOS. Läuft der Daemon mit dem Fake-Backend, entfällt die Prüfung. Sonst prüft sie je Konto die Quelle aus `token_source`, ohne ein Token zu lesen ({{ ref("D74") }}). Jede Zeile nennt das Konto und seine Vaults. Ein Konto ohne Vault ergibt eine Zeile `NOTE`. Der alte Eintrag `a38.onepassword/service-account` und die alte Token-Datei `onepassword-token` sind Befunde mit dem nächsten Schritt ({{ ref("D75") }}). Die Signatur zeigt die Prüfung einmal. Quelle `keychain`: Der Eintrag existiert; die Zeile nennt Datum und vertraute Programme. Ein vertrautes Programm außer diesem `a38` ist ein Befund. Ein Eintrag eines anderen Builds ergibt eine Zeile `NOTE`. Die Signatur von `a38` ohne Developer ID ergibt eine Zeile `NOTE` ({{ ref("G5") }}). Eine übrige Token-Datei ist ein Befund. Quelle `file`: Die Datei existiert, ist eine reguläre Datei des Benutzers und hat den Modus `0600`. Ein fehlender Eintrag oder eine fehlende Datei ist nur ein Befund, wenn der Daemon mit `--backend 1password` läuft. |
| `approval UI` | Zustand der Freigabe-App, soweit der Daemon ihn meldet. Im Modus `app` prüft die Prüfung die Datei der App wie der Daemon vor dem Start. |

Mit `--json` gibt der Befehl ein Objekt mit `ok` und der Liste `results` aus.
Jedes Ergebnis hat `check`, `ok`, `detail` oder `finding`, `cause` und `next_step`.
Eine Zeile `NOTE` hat zusätzlich `note: true`.

## a38 audit

| Aufruf | Wirkung |
|---|---|
| `a38 audit list` | Listet das Audit-Log mit Filtern (`--project`, `--session-name`, `--ref`, `--type`, `--code`, `--since`, `--until`, `--limit`). |
| `a38 audit verify` | Prüft die Hash-Kette. Bei einem Bruch nennt der Befehl den ersten fehlerhaften Eintrag und endet mit 1. |
| `a38 audit stats` | Gibt die Kennzahlen der Übersicht von `a38 tui` aus: Popups, Freigaben, Ablehnungen, Abweichungen, Auslieferungen, Größe der Bündel, Rangfolgen. Schalter: `--range today\|7d\|30d\|all` (Standard `7d`), `--project`, `--session-name`, `--top N` (Standard 5, 0 für alle), `--json`. |

Die Befehle öffnen die Audit-Datei nur lesend.
Der Daemon muss nicht laufen.
`--file <audit-datei>` ersetzt den Pfad aus der Konfiguration.
`list` kennt `--json` und gibt je Eintrag eine Zeile aus.
`stats --json` gibt ein Objekt aus; die Felder heißen wie im Paket `internal/auditstats` (`popups`, `denied`, `denial_share`, `deviations`, `deliveries_under_grant`, `days`, `top_sessions` …).
Die Definitionen der Kennzahlen stehen in der Anleitung [Freigaben auswerten](../how-to/29-freigaben-auswerten.md#was-die-kennzahlen-bedeuten).

## a38 tui

`a38 tui` zeigt das Audit-Log im Terminal ({{ ref("F13") }}, {{ ref("UC6") }}).
Den Befehl gibt es nur unter macOS ({{ ref("D57") }}).
Unter Linux endet er mit 1 und nennt `a38 audit list` und `a38 audit stats`.

| Schalter | Wirkung |
|---|---|
| `--config <datei>` | Konfigurationsdatei; die Audit-Datei steht in `paths.audit_file`. |
| `--file <audit-datei>` | ersetzt den Pfad aus der Konfiguration. |
| `--range today\|7d\|30d\|all` | Zeitraum beim Start, Standard `7d`. Die Taste `t` wechselt ihn. |
| `--refresh <dauer>` | liest das Audit-Log nach der Dauer erneut, zum Beispiel `30s`. Standard `0`: nur mit der Taste `r`. Der kleinste Wert ist `1s`. |

Die Ansicht öffnet die Audit-Datei nur lesend und schließt sie nach jedem Lesen.
Sie schreibt nie und spricht mit keinem Socket des Daemons.
Der Daemon muss nicht laufen.
Ohne Terminal endet der Befehl mit 1.
Eine fehlende oder unlesbare Datei endet vor dem Start der Ansicht mit 1.
Ein falscher Schalter endet mit 2.
`NO_COLOR` schaltet die Farben ab; Fettdruck, Klammern und Marken bleiben.
Die Tasten stehen in der Anleitung [Freigaben auswerten](../how-to/29-freigaben-auswerten.md#tasten).

## a38 selftest

`a38 selftest sdk-core` prüft das laufende Binary ({{ ref("D59") }}, {{ ref("T30") }}).
Der Befehl lädt den WebAssembly-Kern des 1Password-SDK und gibt ihm ein ungültiges Token und die Version von `a38`, wie der Daemon.
Das SDK prüft dabei auch die Version (höchstens 20 Zeichen; Buchstaben, Ziffern, Leerzeichen, `-`, `.`, `_`).
`a38` kürzt und ersetzt die Version vorher; aus `SNAPSHOT` wird `snap`.
Er liest kein Token, keine Datei und keinen Eintrag im Schlüsselbund.
Er ruft kein Netz: Der Kern lehnt das Token vorher ab.

| Ergebnis | Bedeutung |
|---|---|
| Exit-Code 0 | Der Kern läuft in diesem `a38`. Die Zeile nennt die Dauer. |
| Exit-Status 137 in der Shell (`Killed: 9`) | macOS hat `a38` wegen der Signatur beendet. Die Hardened Runtime verbietet den Code des Kerns. Dieses `a38` kann das 1Password-Backend nicht nutzen. |
| Exit-Code 1 | Der Kern hat nicht wie erwartet geantwortet, oder die Plattform hat kein SDK (Linux). |
| Exit-Code 2 | Falscher Aufruf. |

## a38 hook

`a38 hook op-guard` ist ein Hook für das Agent-Werkzeug ({{ ref("F16") }}, {{ ref("D58") }}).
Das Agent-Werkzeug startet ihn vor jedem Befehl des Werkzeugs `Bash`.
Der Mensch richtet ihn ein ([Hook gegen den direkten Aufruf von op einrichten](../how-to/23-hook-gegen-op-einrichten.md)).

| Aufruf | Wirkung |
|---|---|
| `a38 hook op-guard` | Liest die Eingabe des Hooks (JSON) von stdin und entscheidet. Führt den Befehl der Eingabe nie aus. |
| `a38 hook op-guard --print-config` | Gibt den Block `hooks` für die `settings.json` von Claude Code aus, mit dem Namen `a38` (`--scope project`, Standard). |
| `a38 hook op-guard --print-config --scope user` | Derselbe Block mit dem absoluten Pfad des laufenden `a38`. |

Der Befehl liest weder die Konfigurationsdatei noch den Daemon.
Er ändert keine Datei.

Die Exit-Codes folgen dem Vertrag des Agent-Werkzeugs, nicht der Tabelle oben:

| Code | Bedeutung |
|---|---|
| 0 | Durchlassen. Keine Ausgabe. Auch bei einer Eingabe, die kein JSON ist, größer als 1 MiB ist oder deren Befehl kein Parser versteht; dann steht nur der Grund auf stderr, nie ein Teil der Eingabe. |
| 2 | Blockieren. Die Meldung für den Agent steht auf stderr, stdout bleibt leer. |
| 1 | Falscher Aufruf, zum Beispiel ein unbekannter Schalter. Das Agent-Werkzeug lässt den Befehl dann durch. |

Der Hook prüft nur das Werkzeug `Bash`. Er blockiert drei Fälle:

| Regel | Fall |
|---|---|
| `op-cli` | Ein Befehl startet `op`, auch über einen Pfad, hinter `sudo`, `env`, `command`, `exec`, `nohup`, `time`, `nice`, `timeout`, `xargs`, `find -exec`, in Pipeline, Liste, Subshell, Befehlsersetzung oder in der Zeichenkette von `bash -c`, `sh -c`, `zsh -c`, `eval` und `env -S` (bis zur Tiefe 3). |
| `a38-token-file` | Ein Befehl nennt einen Pfad auf `/onepassword-token` oder `/onepassword-token-<name>` als Argument oder als Eingabe (`<`). `ls`, `stat`, `test`, `[`, `echo`, `printf` und `a38` lesen die Datei nicht und passieren. |
| `a38-keychain-token` | `security find-generic-password` mit `-w` oder `-g` für einen Eintrag `a38.onepassword` oder sein Label, auch das Label eines Kontos. |
