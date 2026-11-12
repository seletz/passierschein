---
title: Freigabe-App
description: "Die Swift-App Passierschein: Fälle des Popups, Regeln gegen eine versehentliche Freigabe, Meldung, Protokoll, Exit-Codes, Testschalter und Bau."
render_macros: true
tags:
  - Thema/Freigabe
---
# Freigabe-App

Die Freigabe-App zeigt dem Menschen eine Freigabeanfrage oder eine Meldung ({{ ref("F11") }}).
Der Daemon `a38 serve` startet die App je Nachricht ({{ ref("D6") }}).
Die App heißt „Passierschein“.
Das Kommandozeilenwerkzeug heißt weiter `a38`.
Die App liegt im Ordner `app/` des Repos.
Sie ist ein Swift-Paket mit dem Namen `A38Approval`.
Das Paket und die Module (`A38ApprovalCore`, `A38ApprovalUI`) behalten ihren Namen.
Das ausführbare Programm heißt `Passierschein`.
Das App-Bundle heißt `Passierschein.app`.

Die App kennt keinen Secret-Wert.
Die Nachricht enthält je Zugangsdatum nur Kennung, Vault, Titel und Feld.

!!! warning "Nicht geprüft"
    - Den echten Dialog von Touch ID hat noch niemand mit der App ausgelöst ({{ ref("G4") }}).
    - Die Sicht unter OmniWM, über Vollbild-Apps und auf allen Arbeitsflächen ist nicht geprüft ({{ ref("G4") }}).
    - Esc, Eingabetaste und Leertaste sind nur im Code geregelt. Ein Mensch hat sie am Bildschirm nicht geprüft ({{ ref("T14") }}, Teil 2).
    - Das App-Bundle trägt nur eine Ad-hoc-Signatur. Developer ID und Notarisierung fehlen ({{ ref("G5") }}).

## Aufbau des Codes

| Teil | Inhalt |
|---|---|
| `A38ApprovalCore` | Nachrichten, strenge Prüfung, Entscheidung, Exit-Codes, Zustand von Popup und Meldung, Texte, Schalter. Kein AppKit, kein SwiftUI, keine LocalAuthentication. |
| `A38ApprovalUI` | Fenster (`NSPanel`), Ansichten in SwiftUI, Touch ID mit LocalAuthentication |
| `Passierschein` | Einstieg des Programms |
| `Tests/A38ApprovalCoreTests` | Tests mit Swift Testing, ohne Fenster und ohne Touch ID |
| `Samples/` | Beispielnachrichten mit neutralen Daten |

Alle Texte der Oberfläche stehen in `app/Sources/A38ApprovalCore/Strings.swift`.
Die Texte sind englisch wie im abgenommenen Prototyp.
Das Banner trägt rechts den Namen „Passierschein“.
Texte mit Befehlen für den Menschen nennen weiter `a38`, zum Beispiel `a38 doctor`.
Eine Übersetzung ändert nur diese Datei.

## Fälle des Popups

Die App leitet den Fall aus den Daten ab.
`existing_grant` kennzeichnet eine Abweichung.
Ein eigenes Feld für den Fall gibt es nicht.

| Fall | Erkennung | Darstellung |
|---|---|---|
| Erste Anfrage | kein `existing_grant`, 1 bis 6 Einträge | Blaues Banner „Credential request“. Die Liste rollt nicht. |
| Bündel | kein `existing_grant`, mehr als 6 Einträge | Blaues Banner. Nur die Liste rollt. Unter der Liste steht die Zahl der Einträge und der Vaults. |
| Abweichung | `existing_grant` gesetzt | Orangefarbenes Banner „Deviation from active grant“. Die geprüften Fakten nennen den bestehenden Grant mit Kennung, Anzahl, Vaults und Ablaufzeit. Jeder Eintrag trägt die Marke „NEW ITEM“ oder „NEW VAULT“. |

Eine Anfrage außerhalb von dev erreicht die App im MVP nie ({{ ref("D24") }}, {{ ref("G26") }}).
Die App lehnt einen Eintrag mit `environment` `test` oder `prod` deshalb als ungültig ab.
Der Fall „außerhalb von dev“ des Prototyps fehlt in der App.

## Aufbau des Popups

Die Bereiche stehen in dieser Reihenfolge:

1. Banner mit Fall, Symbol und Farbe.
2. Geprüfte Fakten: Projekt, Kennung der Sitzung, Herkunft, Prozess mit PID und UID, Arbeitsverzeichnis, bei einer Abweichung der bestehende Grant.
3. Angaben des Agents mit der Überschrift „NOT verified“, gestrichelt umrandet: Name der Sitzung, Agent, Aufgabe, Begründung ({{ ref("D4") }}, {{ ref("D25") }}). Die Überschrift und der Rahmen sind orange. Die Werte haben die volle Textfarbe und sind nicht kursiv. Grün kennzeichnet die geprüften Fakten, Orange die ungeprüften Angaben. Der Spike-Bericht zeigt die Angaben noch grau und kursiv; Stefan hat das am 2026-11-12 wegen des schlechten Kontrasts geändert.
4. Zugangsdaten: ein Kontrollkästchen je Eintrag mit Titel und Feld, gruppiert nach Vault, mit Zähler „n of m selected“.
5. Laufzeit und Umfang des Grants.
6. Fußzeile: „Deny (Esc)“ links, Restzeit in der Mitte, Bestätigung rechts.

Der Name der Sitzung steht nur bei den Angaben des Agents.
Die App kürzt ihn nie.
Jede andere Angabe des Agents kürzt die App auf drei Zeilen.
Fehlt eine Angabe, steht „(not given)“ da.
Ein Steuerzeichen oder ein unsichtbares Formatzeichen einer Angabe erscheint als „�“.

## Regeln gegen eine versehentliche Freigabe

Die Regeln setzen {{ ref("D31") }} und {{ ref("D7") }} um.
Der Zustand des Popups (`ApprovalViewModel`) enthält sie.
Die Tests prüfen sie ohne Fenster.

- Kein Kontrollkästchen ist vorbelegt.
- Die Bestätigung ist 1,5 Sekunden lang gesperrt. Sie zeigt in dieser Zeit „Read the request…“.
- Die Bestätigung bleibt gesperrt, solange kein Eintrag ausgewählt ist. Sie zeigt dann „Select credentials“.
- Die Bestätigung nennt die Anzahl: „Approve 2 with Touch ID“.
- Kein Bedienelement nimmt den Tastaturfokus an. Kein Knopf ist der Standardknopf.
- Das Fenster schluckt Eingabetaste, Enter und Leertaste.
- Esc lehnt ab, wenn das Fenster nach einem Klick den Fokus hat.
- „Deny“ ist sofort aktiv.
- Nur ein bestandenes Touch ID gibt frei. Ein Klick allein gibt nichts frei.
- Während Touch ID läuft, ist die Auswahl gesperrt. Die Freigabe nennt die Auswahl zum Zeitpunkt des Klicks.
- „Select all n“ gibt es nur je Vault, nur für einen dev-Vault und nur bei mehr als drei Einträgen im Vault. Einen Knopf für alle Vaults gibt es nicht.
- Nach Ablauf der Zeit endet die App mit `timeout`. Ein offener Dialog von Touch ID endet dabei (`LAContext.invalidate()`).

## Laufzeit

Das Popup zeigt die vorgeschlagene Laufzeit, zum Beispiel „Valid for: 4 h (proposed)“.
„Shorter…“ öffnet die Auswahl.
Die Auswahl ist eine feste Menge ({{ ref("D32") }}):

- die vorgeschlagene Laufzeit,
- die Hälfte davon, abgerundet auf ganze Minuten,
- 15 Minuten.

Jeder Wert ist mindestens eine Minute und nie länger als der Vorschlag.
Doppelte Werte entfallen.
Beispiele: 4 h ergibt 4 h, 2 h, 15 min. 20 min ergibt 20 min, 15 min, 10 min. 10 min ergibt 10 min, 5 min.

Der Text zum Umfang lautet: „The grant covers only the selected credentials, for this session and project …“.
Ein Grant umfasst genau die angehakten Einträge ({{ ref("D32") }}, {{ ref("G26") }}).

## Meldung

Eine Nachricht vom Typ `notice` zeigt eine Meldung ({{ ref("D36") }}, {{ ref("D48") }}, {{ ref("D28") }}, {{ ref("G20") }}).
Die Meldung informiert nur.
Ihre einzige Aktion ist „Close“.

| Art | Banner | Inhalt |
|---|---|---|
| `daemon_fault` | rot, „a38 is locked: daemon fault“ | Folge für alle Sitzungen, Text des Daemons, Code, Zeit, Kennung, nächste Schritte: `a38 doctor`, Ursache beheben, `a38 fault clear` oder Neustart von `a38 serve` |
| `project_config_invalid` | orange, „a38: invalid project configuration“ | betroffenes Projekt, Text des Daemons, Code, Zeit, Kennung, nächste Schritte: `a38 doctor`, Projekt in der Konfiguration korrigieren, `a38 serve` neu starten |

Die Meldung bleibt offen, bis der Mensch sie schließt.

## Protokoll

Der Daemon startet `Passierschein.app/Contents/MacOS/Passierschein` oder ein einzelnes Programm `Passierschein` ohne Argumente.
Das Arbeitsverzeichnis ist `/`.
Die Umgebung enthält nur `HOME`, `USER`, `LOGNAME`, `TMPDIR`, `LANG`, `LC_ALL`, `PATH` und `__CF_USER_TEXT_ENCODING`.
Die App braucht keine weitere Variable.

1. Der Daemon schreibt genau ein JSON-Objekt auf die Standardeingabe und schließt sie.
2. Die App prüft die Nachricht vollständig, bevor sie ein Fenster öffnet.
3. Bei einer Freigabeanfrage schreibt die App genau eine Zeile mit der Entscheidung auf die Standardausgabe und endet.
4. Bei einer Meldung schreibt die App nichts auf die Standardausgabe. Sie endet mit 0, wenn der Mensch die Meldung schließt.
5. Die Standardfehlerausgabe enthält nur Diagnosetext mit Kennungen und Codes, nie einen Text der Nachricht.

Die Felder folgen den Go-Typen des Pakets `internal/approval` (siehe [Architektur](../explanation/architektur.md#freigabe-schnittstelle)).

### Prüfung der Nachricht

Die App lehnt jede dieser Nachrichten ab, ohne ein Fenster zu öffnen:

| Befund | Fehlercode |
|---|---|
| mehr als 256 KiB | `invalid_request` |
| kein einzelnes JSON-Objekt | `invalid_request` |
| `version` ist nicht 1 | `unsupported_version` |
| `type` ist nicht `approval_request` oder `notice` | `invalid_request` |
| ein unbekanntes Feld auf irgendeiner Ebene | `invalid_request` |
| ein Pflichtfeld fehlt, ist `null` oder hat den falschen Typ | `invalid_request` |
| eine Kennung ist leer, länger als 128 Zeichen oder enthält andere Zeichen als Buchstaben, Ziffern, `.`, `_`, `:`, `-` | `invalid_request` |
| `timeout_seconds` liegt nicht zwischen 1 und 3600 | `invalid_request` |
| keine Einträge, mehr als 50 Einträge oder doppelte Kennungen | `invalid_request` |
| `environment` ist nicht `dev` | `invalid_request` |
| `deviation` ist nicht `new_item` oder `new_vault` | `invalid_request` |
| `deviation` ohne `existing_grant` oder `existing_grant` mit einem Eintrag ohne `deviation` | `invalid_request` |
| `grant.ttl_seconds` ist nicht größer als 0 | `invalid_request` |
| eine Zeit ist nicht RFC 3339 | `invalid_request` |
| eine Meldung über ein Projekt ohne `project` oder eine Störung des Daemons mit `project` | `invalid_request` |

Pflichtfelder sind die Felder, die Go immer schreibt.
`grant.scope.vaults` darf `null` sein, weil Go eine leere Liste so schreibt.
Der Fehlertext nennt den Pfad des Feldes, nie einen Wert.
Ist `request_id` lesbar, wiederholt die Entscheidung sie.

### Entscheidung

| Ergebnis | Inhalt |
|---|---|
| `approved` | ausgewählte Kennungen in der Reihenfolge der Anfrage, gewählte Laufzeit `ttl_seconds`, `auth` mit `ok: true` |
| `denied` | leere Liste, kein `auth` |
| `timeout` | leere Liste, kein `auth` |
| `error` | leere Liste, `error` mit Code und Text, bei Touch ID auch `auth` mit `ok: false` |

Vor der Ausgabe prüft die App eine Freigabe ein letztes Mal.
Eine Freigabe ohne bestandene Authentifizierung, ohne Eintrag, mit fremder Kennung oder mit zu langer Laufzeit wird zu `error` mit dem Code `internal`.
Im Release-Build muss die Richtlinie `biometrics` sein.

### Exit-Codes

| Exit-Code | Ergebnis | Bedeutung |
|---|---|---|
| 0 | `approved` | Freigabe |
| 0 | keine Ausgabe | Meldung geschlossen, `--help`, `--version` |
| 10 | `denied` | Der Mensch lehnt ab. |
| 11 | `timeout` | Die Zeit ist abgelaufen. |
| 12 | `error` | Touch ID ist nicht verfügbar (`auth_unavailable`) oder schlägt fehl (`auth_failed`). |
| 64 | `error` | Aufruf falsch, zum Beispiel ein Testschalter im Release-Build |
| 65 | `error` | Nachricht ungültig (`invalid_request`, `unsupported_version`) |
| 70 | `error` | interner Fehler (`internal`) |

Nur Exit-Code 0 zusammen mit `approved` ist eine Freigabe.

## Touch ID

Die App nutzt nur die Richtlinie `.deviceOwnerAuthenticationWithBiometrics` ({{ ref("D31") }}).
Einen Rückfall auf das Passwort gibt es nicht.
Die Einstellungen am `LAContext`:

- `localizedFallbackTitle = ""` blendet „Passwort verwenden“ aus.
- `touchIDAuthenticationAllowableReuseDuration = 0` lässt keinen früheren Fingerabdruck gelten.
- `localizedCancelTitle` ist „Back“.
- `canEvaluatePolicy` läuft direkt vor `evaluatePolicy`.

Der Text für den Dialog nennt nur geprüfte Fakten: „approve 2 credentials for project beispielprojekt“.
„Back“ lässt das Popup offen und zeigt einen Hinweis.
Ist Touch ID nicht verfügbar, endet die App mit `auth_unavailable` und Exit-Code 12.

## Testschalter

Die Testschalter gibt es nur im Debug-Build ({{ ref("T14") }}).
Die Bedingung `A38_TEST_SWITCHES` gilt nur für die Debug-Konfiguration (`Package.swift`).
Der Release-Build enthält den Code der Schalter nicht.
Er lehnt jeden Schalter mit Exit-Code 64 ab.

| Schalter | Wirkung |
|---|---|
| `--request <datei>` | liest die Nachricht aus einer Datei |
| `--skip-auth` | gefälschte Authentifizierung mit Erfolg |
| `--fake-auth <ergebnis>` | gefälschte Authentifizierung: `success`, `cancel`, `unavailable`, `fail` |
| `--auto-close <s>` | Freigabeanfrage: `timeout` nach dieser Zeit, höchstens `timeout_seconds`; Meldung: schließt nach dieser Zeit |
| `--no-window` | läuft ohne Fenster (Tests) |
| `--demo-select <ids>` | wählt Einträge beim Start aus, `all` wählt alle (nur für Screenshots) |
| `--demo-scroll-end` | rollt eine lange Liste ans Ende |
| `--demo-lifetime-open` | öffnet die Auswahl der Laufzeit |
| `--demo-arm-delay <s>` | ändert die Sperre der Bestätigung |
| `--demo-confirm` | klickt die Bestätigung, sobald sie aktiv ist (mit `--no-window`) |
| `--debug-window` | schreibt die Eigenschaften des Fensters auf die Standardfehlerausgabe |

Die gefälschte Authentifizierung meldet die Richtlinie `skipped`.
Der Daemon nimmt `skipped` nie als Freigabe an.
Eine Freigabe aus einem Debug-Build mit Testschalter erreicht deshalb nie einen Agent.

## Bau

Die App baut nur auf macOS.
Sie braucht Xcode für das macOS-SDK.
`mise.toml` legt die Swift-Version an jedem Task der App fest (Swift 6.4.2, Toolchain von swift.org über mise).
Das Skript `scripts/app-swift-check.sh` prüft macOS, SDK und Version.

| Task | Wirkung |
|---|---|
| `mise run app:build` | baut Debug und Release, legt `app/.build/bundle/Passierschein.app` an und prüft den Release-Build |
| `mise run app:test` | führt `swift test` aus |
| `mise run app:run:sample -- <beispiel>` | zeigt ein Beispiel im Debug-Build mit gefälschter Authentifizierung |

`app:build` legt das App-Bundle aus dem Release-Build an:

```text
Passierschein.app/
  Contents/
    Info.plist
    MacOS/
      Passierschein   die Freigabe-App
      a38             vorgesehen für das Go-Programm, fehlt noch
```

Der Platz `Contents/MacOS/a38` ist für das Go-Programm vorgesehen.
`app:build` kopiert `a38` nicht in das Bundle.
Das Packen gehört zur Arbeit am Release ({{ ref("G5") }}).
Der Daemon sucht die App als `Passierschein.app/Contents/MacOS/Passierschein` oder als `Passierschein` neben `a38`, im Bundle neben `Contents/MacOS/a38` und in `/Applications` und `~/Applications`.

Die `Info.plist` enthält die Bundle-ID `io.github.seletz.passierschein`, den Namen „Passierschein“ (`CFBundleName`, `CFBundleDisplayName`) und `LSUIElement`.
Der Dialog von Touch ID nennt diesen Namen.
Das Bundle trägt eine Ad-hoc-Signatur mit Hardened Runtime.
Danach prüft `scripts/app-release-check.sh` den Release-Build und das Bundle:

- Jeder Testschalter endet mit Exit-Code 64 und einer Entscheidung `error`.
- Das Binary enthält keinen Code der Testschalter und keine gefälschte Authentifizierung.
- `--help` schreibt nichts auf die Standardausgabe.

Die Tasks der App gehören nicht zu `mise run check`.
Die CI läuft auf Linux und kann die App nicht bauen.
Ein Job auf einem macOS-Runner ist eine spätere Entscheidung.

## Beispiele

| Datei | Fall |
|---|---|
| `first-request.json` | erste Anfrage mit zwei Einträgen |
| `bundle.json` | Bündel mit zehn Einträgen in zwei Vaults (7 und 3) |
| `deviation.json` | Abweichung mit „NEW ITEM“ und „NEW VAULT“ |
| `long-claims.json` | lange Angaben des Agents und langer Name der Sitzung |
| `notice-daemon-fault.json` | Meldung einer Störung des Daemons |
| `notice-project-config-invalid.json` | Meldung einer fehlerhaften Konfiguration eines Projekts |

Die Beispiele passen zu den Go-Typen von `internal/approval`.

## Tests

`mise run app:test` startet kein Fenster und ruft Touch ID nie auf.

| Bereich | Testdatei |
|---|---|
| Prüfung der Nachricht, jede ungültige Eingabe | `DecodingTests.swift` |
| Regeln des Popups, Fälle, Laufzeit, Fakten und Angaben, Zeitablauf | `ApprovalViewModelTests.swift` |
| Entscheidung, Exit-Codes, letzte Prüfung, Schalter, Meldung | `DecisionTests.swift` |
| Programm als Prozess wie vom Daemon gestartet ({{ ref("T14") }}, Teil 1) | `ProcessTests.swift` |

{{ ref("T14") }} Teil 2 bleibt manuell: Eingabetaste, Verzögerung am Bildschirm, Name der App im Dialog von Touch ID, Fenster unter OmniWM.
