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

!!! success "Praxistest vom 2026-11-12"
    Stefan hat die App mit dem Daemon und dem echten 1Password-Backend am Bildschirm geprüft ({{ ref("G4") }}):

    - Der echte Dialog von Touch ID gibt frei. Der Dialog nennt die App „Passierschein“.
    - Das Popup schwebt unter OmniWM über den gekachelten Fenstern. OmniWM ordnet nichts um.
    - Das Popup nimmt beim Erscheinen den Tastaturfokus nicht weg.
    - Die Eingabetaste und die Leertaste lösen nichts aus. Esc lehnt ab.
    - Die Angaben des Agents sind lesbar. Der Name der Sitzung stimmt.
    - Eine Teilauswahl liefert nur den angehakten Eintrag. Eine Wiederholung unter einem Grant zeigt kein Popup.

!!! warning "Nicht geprüft"
    - Vollbild-App, Wechsel des Arbeitsbereichs, Zeitablauf und die Meldungen am Bildschirm ({{ ref("G33") }}).
    - Der Entwicklungsbau trägt nur eine Ad-hoc-Signatur. Das Release signiert und notarisiert das Bundle mit der Developer ID ([Release](50-release.md#signatur-und-notarisierung)). Der signierte Snapshot ist am 2026-11-12 mit der echten Developer ID grün ([Lauf 43208639784](https://github.com/seletz/passierschein/actions/runs/43208639784)). Der Praxistest eines echten Release auf dem Mac steht aus ({{ ref("G5") }}).

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
| `daemon_fault` | rot, „Passierschein: daemon fault“ | Folge für alle Sitzungen, Text des Daemons, Code, Zeit, Kennung, nächste Schritte: `a38 doctor`, Ursache beheben, `a38 fault clear` oder Neustart von `a38 serve` |
| `project_config_invalid` | orange, „Passierschein: invalid project configuration“ | betroffenes Projekt, Text des Daemons, Code, Zeit, Kennung, nächste Schritte: `a38 doctor`, Projekt in der Konfiguration korrigieren, `a38 serve` neu starten |

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

Die Grenze von 50 Einträgen steht in Swift in `app/Sources/A38ApprovalCore/Decoding.swift` (`maxCredentials`).
In Go steht sie in `config.MaxBundleEntries`.
Die Konfiguration lehnt `bundle.max_entries` über 50 beim Laden ab.
Der Bereich von `timeout_seconds` (1 bis 3600) steht in Swift an derselben Stelle (`timeoutSeconds`) und in Go in `approval.MinTimeoutSeconds` und `approval.MaxTimeoutSeconds`.
Die Warteschlange des Daemons hält ihre Wartezeit in diesem Bereich.
`appui` startet die App nicht, wenn der Wert außerhalb liegt, und meldet einen Fehler.

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

### Sprache des Dialogs

macOS baut den Dialog aus seinem eigenen Rahmen in der Sprache des Systems und aus dem Text der App.
Der Text der App und der Knopf zum Abbrechen folgen deshalb der Sprache des Systems ({{ ref("G34") }}).
Ist die erste bevorzugte Sprache Deutsch, lautet der Text „1 Zugangsdatum für das Projekt … freigeben“ und der Knopf „Zurück“.
Sonst lautet der Text „approve 1 credential for project …“ und der Knopf „Back“.
Das Popup selbst bleibt englisch.

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
| `mise run app:icon` | zeichnet die drei Entwürfe des App-Icons und baut `app/.build/icon/AppIcon.icns` (siehe [App-Icon](#app-icon)) |
| `mise run app:test` | führt `swift test` aus |
| `mise run app:run:sample -- <beispiel>` | zeigt ein Beispiel im Debug-Build mit gefälschter Authentifizierung |
| `mise run app:e2e` | baut den Debug-Build und startet `a38 serve --ui app` mit der App über echte Prozesse (siehe unten) |

`app:build` legt das App-Bundle aus dem Release-Build an:

```text
Passierschein.app/
  Contents/
    Info.plist
    MacOS/
      Passierschein   die Freigabe-App
      a38             nur im Release-Bundle
    Resources/
      AppIcon.icns    das App-Icon
```

`app:build` ist der Entwicklungsbau und kopiert `a38` nicht in das Bundle.
Das Release-Bundle mit `a38` baut `mise run release:app` aus den darwin-Archiven von goreleaser ([Release](50-release.md#app-im-release)).
Der Daemon sucht die App als `Passierschein.app/Contents/MacOS/Passierschein` oder als `Passierschein` neben `a38`, im Bundle neben `Contents/MacOS/a38` und in `/Applications` und `~/Applications`.

Die `Info.plist` enthält die Bundle-ID `io.github.seletz.passierschein`, den Namen „Passierschein“ (`CFBundleName`, `CFBundleDisplayName`), das Icon (`CFBundleIconFile` = `AppIcon`) und `LSUIElement`.
Der Dialog von Touch ID nennt diesen Namen.
Das Bundle trägt eine Ad-hoc-Signatur mit Hardened Runtime.
Danach prüft `scripts/app-release-check.sh` den Release-Build und das Bundle:

- Jeder Testschalter endet mit Exit-Code 64 und einer Entscheidung `error`.
- Das Binary enthält keinen Code der Testschalter und keine gefälschte Authentifizierung.
- `--help` schreibt nichts auf die Standardausgabe.

Die Tasks der App gehören nicht zu `mise run check`.
Der Workflow `macos.yml` führt sie auf einem macOS-Runner aus ([CI](55-ci.md)).

### App-Icon

Das App-Icon zeigt einen stilisierten Reisepass ({{ ref("D69") }}).
Die Quelle ist das Swift-Programm `app/Icon/render-icon.swift`.
Es zeichnet drei Entwürfe mit Core Graphics:

| Entwurf | Motiv |
|---|---|
| 1 | Ausweiskarte mit Fotofeld und Textzeilen |
| 2 | Reisepass mit einem Siegel aus Ring und Schlüsselloch (im Bundle) |
| 3 | Passierschein mit Eselsohr und Stempel mit Haken |

`mise run app:icon` übersetzt das Programm mit der festgelegten Swift-Version.
Der Task zeichnet jeden Entwurf in zehn PNG-Dateien von 16 bis 1024 Pixel.
`iconutil` fügt die Dateien zu `AppIcon.icns` zusammen.
Jede Größe hat ein eigenes Pixelraster.
Unter 64 Pixel entfallen feine Linien.
Die Variable `APP_ICON_DRAFT` wählt den Entwurf des Bundles, Vorgabe 2.
Die Entwürfe 1 und 3 bleiben in der Quelle als verworfene Varianten.
Der Task zeichnet den gewählten Entwurf zweimal und vergleicht die beiden Dateien ({{ ref("T42") }}).

`app:build` und `release:app` rufen `scripts/app-icon.sh` auf.
Kein Schritt ist von Hand.
`scripts/app-icon-check.sh` prüft das Bundle: `CFBundleIconFile`, die Datei `Contents/Resources/AppIcon.icns` und die zehn Größen.
`app:build` und `release:app:check` führen die Prüfung aus.

Das Icon erscheint im Finder, im Launchpad, im Dialog „Informationen“ und im Dialog von Touch ID.
Die App hat kein Dock-Icon und kein Menü (`LSUIElement`).
Das Popup und die Meldung zeigen das App-Icon nicht.
Die App sendet keine Mitteilungen über die Mitteilungszentrale.

## Signatur der App

Der Daemon prüft vor jedem Start der App deren Code-Signatur ({{ ref("G5") }}, Erweiterungspunkt `appui.Verifier`).
Die Prüfung hängt von der Signatur von `a38` selbst ab.
`a38 serve` liest sie einmal beim Start und schreibt eine Zeile ins Log.

| Signatur von `a38` | Prüfung der App | Zeile im Log |
|---|---|---|
| Developer ID mit Team-ID (Release) | Die App muss die Anforderung „Developer ID des Teams“ erfüllen und die Hardened Runtime tragen. | `the approval app must carry a Developer ID signature of the team of a38` mit `team=` |
| ad-hoc oder keine (Entwicklung, CI, Demo) | aus | `the signature check of the approval app is off: a38 has no Developer ID signature` |
| nicht lesbar | Jeder Start der App scheitert (fail closed). | `the approval app is refused: cannot read the signature of a38` |

Die Anforderung lautet: Kette bis zu einer Wurzel von Apple, Zwischenzertifikat „Developer ID Certification Authority“, Blattzertifikat „Developer ID Application“, Organisationseinheit gleich der Team-ID.
Ein selbst signiertes Zertifikat mit derselben Team-ID im Namen erfüllt sie nicht.
Der Daemon ruft dafür `/usr/bin/codesign --verify --strict --deep -R …` und `/usr/bin/codesign -d` auf, mit festem Pfad und einer Zeitgrenze von zehn Sekunden je Aufruf.
Er braucht kein CGO.

Eine App, die die Prüfung nicht besteht, startet der Daemon nicht.
Die Anfrage endet mit `ui_failed`, das Log nennt den Befund.
Mit einem signierten `a38` startet eine Debug-App oder ein Entwicklungsbau also nie.
Zum Entwickeln nutzt du ein selbst gebautes `a38` (`mise run build`).

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
Der Go-Test `TestAppSamples` liest jede Datei streng (`DisallowUnknownFields`) und schreibt sie ohne Verlust zurück.
`app/Samples` ist der Vertrag zwischen Daemon und App.
Die alten Beispiele unter `spikes/ui-popup/samples` bleiben als Aufzeichnung des Spikes.

## Tests

`mise run app:test` startet kein Fenster und ruft Touch ID nie auf.

| Bereich | Testdatei |
|---|---|
| Prüfung der Nachricht, jede ungültige Eingabe | `DecodingTests.swift` |
| Regeln des Popups, Fälle, Laufzeit, Fakten und Angaben, Zeitablauf | `ApprovalViewModelTests.swift` |
| Entscheidung, Exit-Codes, letzte Prüfung, Schalter, Meldung | `DecisionTests.swift` |
| Programm als Prozess wie vom Daemon gestartet ({{ ref("T14") }}, Teil 1) | `ProcessTests.swift` |
| Beispiele gegen die Go-Typen | Go: `TestAppSamples` in `internal/approval` |
| Daemon und App über echte Prozesse | Go: `TestAppE2E` in `cmd/a38` (`mise run app:e2e`) |

### Ablauf über echte Prozesse

`mise run app:e2e` baut den Debug-Build und führt `TestAppE2E` aus (`cmd/a38/app_e2e_test.go`, Build-Tags `darwin appe2e`).
Der Task gehört nicht zu `mise run check`, weil die CI auf Linux läuft.
Der Test baut `a38` und startet `a38 serve --backend fake --ui app`.
Als App startet der Daemon ein kleines Skript.
Das Skript ruft die Debug-App mit Testschaltern auf, weil der Daemon keine Argumente übergibt.
Es zeichnet auf, was die App liest und schreibt.
Die Fälle:

| Fall | Schalter der App | Erwartung |
|---|---|---|
| gefälschte Authentifizierung | `--no-window --skip-auth --demo-select all --demo-confirm` | Die App gibt frei, Richtlinie `skipped`. Der Daemon lehnt ab: der Agent erhält `ui_failed`, keinen Wert, es gibt keinen Grant, das Audit-Log hat einen Eintrag `error`. |
| Zeit abgelaufen | `--skip-auth --auto-close 2` | Exit-Code 11, Ergebnis `timeout`, der Agent erhält `timeout`. |
| Meldung | `--skip-auth --auto-close 2` | Ein Projekt mit unbekanntem Vault löst `config_invalid` und eine Meldung aus. Die App endet mit 0 und schreibt nichts. |

Der Test prüft, dass das JSON in beide Richtungen fließt: Die App liest die Anfrage des Daemons streng als `approval.Request` oder `approval.Notice`, und der Daemon liest die Entscheidung der App.
Er prüft auch, dass die App nur die kurze Umgebung erhält und die Anfrage keinen Wert enthält.
Der Daemon kann keine ungültige Anfrage erzeugen. Dafür gelten die Tests der App (`DecodingTests.swift`).

Die Läufe rufen nie Touch ID auf.
Jeder Lauf nutzt `--skip-auth` oder endet durch `--auto-close`, bevor jemand bestätigt.
Zwei Fenster erscheinen für je etwa zwei Sekunden auf dem Bildschirm.
Der Task braucht eine angemeldete Sitzung am Bildschirm.

{{ ref("T14") }} Teil 2 bleibt manuell: Eingabetaste, Verzögerung am Bildschirm, Name der App im Dialog von Touch ID, Fenster unter OmniWM.
