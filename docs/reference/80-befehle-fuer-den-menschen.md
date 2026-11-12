---
title: Befehle für den Menschen
description: "a38 grants, fault, config, doctor, audit und tui: Argumente, Ausgabe und Exit-Codes."
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
Die Befehle `grants`, `fault`, `doctor` und `config onepassword-token status` kennen `--json`.

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

| Schlüssel | Standard | Bedeutung |
|---|---|---|
| `backend.onepassword.token_source` | `keychain` | Quelle des Tokens des Service Accounts: `keychain` (Schlüsselbund von macOS) oder `file` (Token-Datei). Ein anderer Wert verhindert das Laden der Konfiguration. |
| `backend.onepassword.token_file` | `$XDG_CONFIG_HOME/a38/onepassword-token`, ohne die Variable `~/.config/a38/onepassword-token` | Token-Datei des Service Accounts. Nur `--backend 1password` mit `token_source: file` liest sie. |

Die Quelle `keychain` ist der Standard ({{ ref("D51") }}).
Der Daemon liest dann den Eintrag `a38.onepassword` (Konto `service-account`) im Anmelde-Schlüsselbund.
`a38 config onepassword-token set` legt ihn an (siehe [a38 config](#a38-config)).
Fehlt der Eintrag, startet der Daemon mit `--backend 1password` nicht.
Die Meldung nennt den Befund und den Befehl `a38 config onepassword-token set`.
Wartet der Schlüsselbund länger als zwei Minuten, zum Beispiel auf einen Dialog von macOS, bricht der Start ebenfalls ab.

Die Quelle `file` ist der Ausweichweg für Entwicklung und CI.
Die Token-Datei ist eine reguläre Datei, gehört dem Benutzer und hat genau den Modus `0600`.
Sonst startet der Daemon mit `--backend 1password` nicht, und die Meldung nennt den Befund.
Jeder Prozess desselben Benutzers kann die Datei lesen, auch ein Agent ({{ ref("RI5") }}).
`A38_CONFIG` verschiebt die Token-Datei nicht.

Der Daemon liest das Token einmal beim Start, aus genau einer Quelle.
Eine Änderung des Tokens wirkt erst nach einem Neustart des Daemons.
Ein ungültiges Token oder ein fehlendes Netz verhindern den Start nicht: Jede Anfrage endet dann mit `backend_locked` oder `backend_unavailable`.
Die Einrichtung beschreibt [1Password-Backend einrichten](../how-to/30-1password-backend-einrichten.md).

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
Heute gibt es einen Unterbefehl: `onepassword-token`.

### a38 config onepassword-token

Der Befehl verwaltet das Token des 1Password-Service-Accounts im Schlüsselbund von macOS ({{ ref("D51") }}).
Nur der macOS-Build kennt ihn.
Unter Linux endet jeder Unterbefehl mit „not available on this platform“ und dem Exit-Code 1.

| Aufruf | Wirkung |
|---|---|
| `a38 config onepassword-token set [--replace]` | Liest das Token vom Terminal und legt es im Eintrag `a38.onepassword` ab. Ein vorhandener Eintrag bleibt ohne `--replace` unverändert; der Befehl endet dann mit 1. |
| `a38 config onepassword-token status [--json]` | Zeigt die Quelle, die Metadaten des Eintrags, die Signatur von `a38` und eine übrige Token-Datei. Exit-Code 1, wenn die konfigurierte Quelle kein Token hat. |
| `a38 config onepassword-token delete` | Löscht den Eintrag. Die Ausgabe nennt, was der Daemon beim nächsten Start tut. |

Alle drei kennen `--config <datei>`.
Ohne Konfigurationsdatei gilt die Quelle `keychain`.
Kein Unterbefehl zeigt das Token oder einen Wert, der daraus abgeleitet ist (Länge, Prüfsumme).

`set`:

- Die Standardeingabe muss ein Terminal sein. Der Befehl fragt auf stderr nach dem Token und liest ohne Echo.
- Ist die Standardeingabe kein Terminal, zum Beispiel eine Pipe oder eine Datei, liest der Befehl nichts und endet mit dem Exit-Code 2.
- Das Token ist nie ein Argument und nie eine Umgebungsvariable. Ein Argument ist ein Aufruffehler (Exit-Code 2); der Befehl wiederholt es nicht.
- Die Prüfung der Form: nicht leer, mindestens 16 und höchstens 4096 Bytes, kein Leerzeichen, kein Steuerzeichen. Ein zu kurzes Token endet mit „too short“. Ein Token ohne den Anfang `ops_` ergibt nur eine Warnung, denn dieser Anfang ist eine Annahme.
- Die Ausgabe nennt nur den Eintrag.
- `a38` legt den Eintrag selbst an, nicht über `/usr/bin/security`. Der Schlüsselbund vertraut dann nur diesem Build von `a38`.
- `--replace` löscht den alten Eintrag und legt ihn neu an. Danach vertraut der Eintrag dem neuen Build.

`status`:

- Der Befehl liest das Token nie. Er zeigt deshalb nie einen Dialog von macOS.
- Die Zeile `Keychain` nennt den Eintrag, das Datum der Anlage und der letzten Änderung, die vertrauten Programme und die Partitionsliste.
- Nennt die Partitionsliste nicht den Hash dieses Builds (`cdhash:`), folgt ein Hinweis: Der nächste Start des Daemons zeigt einen Dialog.
- Die Zeile `Signature` nennt die Art der Signatur von `a38`: `unsigned`, `ad-hoc`, `developer-id` oder `other-certificate`. Ohne Developer ID folgt eine Warnung ({{ ref("G5") }}).
- Die Zeile `Token file` nennt die Token-Datei. Der Befehl prüft nur Besitzer und Rechte und liest den Inhalt nicht. Ist die Quelle der Schlüsselbund, ist eine übrige Datei ein Befund.

## a38 doctor

`a38 doctor` führt die Prüfungen aus.
Jede Prüfung steht als Zeile `OK`, `NOTE` oder `FINDING`.
Eine Zeile `NOTE` nennt eine bekannte Grenze und ist kein Befund.
Ein Befund nennt die Ursache und den nächsten Schritt.
Der Exit-Code ist 0 ohne Befund.

| Prüfung | Inhalt |
|---|---|
| `configuration` | Die Konfiguration lädt. Je ungültigem Projekt folgt ein Befund mit dem Namen des Projekts. |
| `socket modes` | Das Verzeichnis hat den Modus 0700, die Sockets 0600. |
| `daemon` | Der Daemon antwortet auf dem Admin-Socket. Die Zeile nennt Version, Sitzungen und Grants. |
| `delivery lock` | Zustand der Sperre mit Ursache und Beginn. Nächster Schritt: `a38 fault clear`. |
| `audit file` | Die Datei existiert und ist schreibbar, das Verzeichnis ebenfalls. |
| `audit space` | Im Verzeichnis sind mindestens 100 MiB frei. |
| `audit chain` | Die Hash-Kette stimmt. Die Prüfung liest die Datei nur. |
| `backend` | Zustand des Backends, soweit der Daemon ihn meldet: `ok`, `locked` oder `unreachable`. Für 1Password heißt `locked`: 1Password lehnt das Token ab. |
| `1password token` | Nur macOS. Läuft der Daemon mit dem Fake-Backend, entfällt die Prüfung. Sonst prüft sie die Quelle aus `token_source`, ohne das Token zu lesen. Quelle `keychain`: Der Eintrag existiert; die Zeile nennt Datum und vertraute Programme. Ein vertrautes Programm außer diesem `a38` ist ein Befund. Ein Eintrag eines anderen Builds ergibt eine Zeile `NOTE`. Die Signatur von `a38` ohne Developer ID ergibt eine Zeile `NOTE` ({{ ref("G5") }}). Eine übrige Token-Datei ist ein Befund. Quelle `file`: Die Datei existiert, ist eine reguläre Datei des Benutzers und hat den Modus `0600`. Ein fehlender Eintrag oder eine fehlende Datei ist nur ein Befund, wenn der Daemon mit `--backend 1password` läuft. |
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
Die Definitionen der Kennzahlen stehen in der Anleitung [Freigaben auswerten](../how-to/26-freigaben-auswerten.md#was-die-kennzahlen-bedeuten).

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
`NO_COLOR` schaltet die Farben ab; Fettdruck und Umkehrung bleiben.
Die Tasten stehen in der Anleitung [Freigaben auswerten](../how-to/26-freigaben-auswerten.md#tasten).
