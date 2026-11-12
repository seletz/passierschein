---
title: "Spike: Token des Service Accounts im Schlüsselbund"
description: "Der Spike klärt, wie a38 das Token des 1Password-Service-Accounts im macOS-Schlüsselbund ablegt, sodass ein anderer Prozess desselben Benutzers es nicht unbemerkt liest."
render_macros: true
tags:
  - Spike
  - Sicherheit
  - 1Password
---
# Spike: Token des Service Accounts im Schlüsselbund

Stand: 2026-11-12. Bezug: Issue seletz/passierschein#53, Teil von seletz/passierschein#49.

Der Spike hat 1Password nicht benutzt. Er hat kein Token gelesen und keine Token-Datei geöffnet. Alle Versuche nutzen den Dummy-Wert `spike-dummy-wert-1` in Einträgen mit dem Dienstnamen `a38-spike-…`. Der Spike hat jeden Eintrag am Ende gelöscht. Der Spike hat keinen Eintrag mit Benutzerpräsenz angelegt und keine Abfrage von Touch ID ausgelöst.

## Frage

Heute liegt das Token des Service Accounts in einer Datei mit dem Modus `0600`. Diese Arbeitsannahme heißt im Design D51. D51 liegt auf einem anderen Zweig und ist hier nicht verlinkt. Der Agent läuft in den Stufen 1 bis 4 unter demselben Benutzer wie der Daemon ({{ ref("RI5") }}). Er kann die Datei lesen und den Daemon umgehen ({{ ref("G3") }}). Stefan hat am 2026-11-12 festgelegt: Das Ziel ist der macOS-Schlüsselbund. Der Mensch fügt das Token über einen Befehl von `a38` im Terminal ein.

Der Spike beantwortet die Punkte des Issues:

1. Welcher Weg in den Schlüsselbund trägt ohne CGO?
2. Wie bindet der Schlüsselbund den Eintrag an das Programm `a38`?
3. Geht Touch ID beim Lesen für ein Kommandozeilenprogramm oder einen Daemon?
4. Wie sieht der Befehl für den Menschen aus?
5. Wie verhält sich der Daemon als LaunchAgent?
6. Welche Bedrohungen bleiben?
7. Was empfiehlt der Spike, in welchen Stufen?
8. Wie testet Stefan die offenen Varianten?

## Kurzantwort

- Ein Go-Programm mit `CGO_ENABLED=0` ruft Security.framework über `purego` direkt auf. Das funktioniert auf diesem Rechner. Der Eintrag liegt dann im dateibasierten Schlüsselbund `login.keychain-db`.
- Der Schlüsselbund trägt das aufrufende Programm in die Zugriffsliste des Eintrags ein. Ein anderes Programm desselben Benutzers liest den Wert nicht still. Es löst einen Dialog von macOS aus. Das ist gemessen.
- Der Weg über `/usr/bin/security` schützt nicht. Der Schlüsselbund trägt dann `security` als vertrautes Programm ein. Jeder Prozess des Benutzers liest den Wert danach mit `security find-generic-password -w` still. Das ist gemessen.
- Ohne Developer ID bindet der Schlüsselbund den Eintrag an den Hash der Code-Signatur (`cdhash`). Jeder neue Build von `a38` mit geändertem Code bekommt einen neuen Hash. `a38` selbst löst dann nach jedem Update einen Dialog aus.
- Mit der Developer ID bindet der Schlüsselbund den Eintrag an die Team-ID und den Bezeichner. Ein Update von `a38` liest dann ohne Dialog. Jedes andere Programm löst einen Dialog aus. Das ist aus der Doku belegt und mit Stefan zu testen.
- Der Data-Protection-Schlüsselbund und Touch ID beim Lesen brauchen eine eingeschränkte Berechtigung und ein Provisioning-Profil. Ein ad-hoc signiertes Programm mit dieser Berechtigung beendet macOS beim Start. Das ist gemessen. Touch ID beim Lesen passt nicht zu einem Daemon, der beim Anmelden startet.
- Der Schlüsselbund hebt die Hürde von „Datei lesen“ auf „Code-Signatur überwinden oder einen sichtbaren Dialog bestätigen lassen“. Er stoppt nicht den Angriff auf den laufenden Daemon und nicht einen Menschen, der einen Dialog vorschnell bestätigt.
- Empfehlung: jetzt `a38 token set|status|delete` mit `purego` bauen, die Token-Datei als Rückfall für die Entwicklung behalten. Mit dem signierten Release ({{ ref("G5") }}) die Hardened Runtime und die Developer ID für `a38` einführen. `CGO_ENABLED=0` bleibt.

## Kennzeichnung der Befunde

Jeder Befund trägt eine Kennzeichnung:

- **[Doku]** Aus der Doku belegt. Die Quelle steht beim Befund oder unter [Quellen](#quellen).
- **[Gemessen]** Auf diesem Rechner gemessen. Umgebung: macOS 27.2 (26B123), arm64, Go 1.27.4, `purego` v0.11.3, Schlüsselbund `login.keychain-db`. Die Programme sind ad-hoc signiert. Kein Versuch nutzt die Developer ID.
- **[Annahme]** Ableitung ohne Beleg. Mit Stefan zu testen.

## Versuche auf diesem Rechner

Der Wegwerfcode liegt unter `spikes/token-keychain/`. Das Programm `kcspike` ruft `SecItemAdd`, `SecItemCopyMatching`, `SecItemDelete` und die Funktionen für Zugriffslisten über `purego` auf. Es liest einen Wert nur von stdin und gibt nur die Länge eines gelesenen Werts aus. Der Go-Linker signiert jedes darwin/arm64-Binary ad-hoc ([Go-Issue 42684](https://golang.org/issue/42684)) (`flags=0x20002(adhoc,linker-signed)`, `Identifier=a.out`). Die festgelegte Anforderung (designated requirement) ist der Hash: `designated => cdhash H"cbc3…b178"`.

| Nr. | Versuch | Ergebnis | Dialog |
|---|---|---|---|
| V1 | `kcspike-a add` legt `a38-spike-ffi` an, `kcspike-a read` liest | `SecItemAdd: status 0`, `SecItemCopyMatching: status 0`, Länge 18 | nein |
| V2 | `kcspike-a acl` liest die Zugriffsliste von `a38-spike-ffi` | Eintrag 0 (`Decrypt`, `ExportClear` u. a.): nur der Pfad von `kcspike-a`. Partitionsliste: `cdhash:cbc3…b178`. `Encrypt` und `Integrity`: jedes Programm. | nein |
| V3 | `security find-generic-password -s a38-spike-ffi` ohne `-w` | Die Attribute (Dienst, Konto, Datum) erscheinen ohne Dialog. Der Wert erscheint nicht. | nein |
| V4 | zweiter Build `kcspike-b` (anderer Hash) liest `a38-spike-ffi` mit `kSecUseAuthenticationUIFail` | Der Aufruf blockiert. Der Prozess `SecurityAgent` startet in derselben Sekunde (07:27:24). Der Spike beendet `kcspike-b` nach 8 Sekunden. | **ja, sehr wahrscheinlich** |
| V5 | `kcspike-a add -dp` (Data-Protection-Schlüsselbund, ohne Berechtigung) | `SecItemAdd: status -34018 (A required entitlement is not present.)` | nein |
| V6 | Kopie von `kcspike-a`, ad-hoc signiert mit Hardened Runtime und `keychain-access-groups` | Der Prozess endet beim Start mit Exit-Code 137 (SIGKILL), ohne eine Zeile Ausgabe. Dieselbe Kopie ohne Berechtigung startet. | nein |
| V7 | `security add-generic-password … -w` ohne Wert, Wert auf stdin (kein Terminal) | `security` fragt zweimal und liest beide Zeilen von stdin. Der Eintrag entsteht. Der Wert steht nicht in argv. | nein |
| V8 | `security find-generic-password -w` auf den Eintrag aus V7, aus einer anderen Shell | Der Wert kommt still zurück, Länge 18, gleich dem Dummy. | nein |
| V9 | `kcspike-a acl` auf den Eintrag aus V7 | Eintrag 0: nur `/usr/bin/security`. Partitionsliste: `apple-tool:`. | nein |
| V10 | `security add-generic-password … -T <pfad von kcspike-a> -w` | Eintrag 0: nur `kcspike-a`, nicht `security`. Partitionsliste bleibt `apple-tool:`. | nein |
| V11 | `security -i` mit dem Befehl `add-generic-password … -w <wert>` auf stdin | Der Eintrag entsteht. argv enthält nur `security -i`. | nein |
| V12 | derselbe Quelltext, ein zweites Mal gebaut | gleicher `cdhash` wie der erste Build | nein |
| V13 | Löschen aller Einträge, danach `security find-generic-password -s a38-spike-…` | jedes Löschen `status 0`; jede Suche endet mit Exit-Code 44 (nicht gefunden) | nein |

Zum Dialog in V4: Der Spike hat den Dialog nicht gesehen. Der Start von `SecurityAgent` im selben Moment und das Blockieren belegen ihn. Nach dem Löschen der Einträge lief `SecurityAgent` nicht mehr. Der Spike hat deshalb keinen weiteren Leseversuch mit einem fremden Programm gemacht: nicht `security find-generic-password -w` auf den Eintrag aus V1 und nicht `kcspike-a read` auf den Eintrag aus V10. Beide hätten nach der Zugriffsliste ebenfalls einen Dialog gezeigt. Sie stehen im [Testplan](#testplan-fur-den-praxistest).

**[Gemessen]** `kSecUseAuthenticationUIFail` verhindert den Dialog der Zugriffsliste im dateibasierten Schlüsselbund nicht (V4). Ein Programm kann einen Dialog also nicht sicher unterdrücken. `a38 doctor` darf deshalb nie einen Lesetest mit einem fremden Programm machen.

**[Gemessen]** Das Lesen der Zugriffsliste mit `SecKeychainItemCopyAccess` zeigt keinen Dialog, auch für einen Eintrag, den ein anderes Programm angelegt hat (V9, V10).

## 1. Zugriff ohne CGO

### Weg A: Security.framework über `purego`

- **[Gemessen]** `purego` lädt `Security.framework` und `CoreFoundation.framework` mit `Dlopen`. Ein Binary mit `CGO_ENABLED=0` ruft `SecItemAdd`, `SecItemCopyMatching` und `SecItemDelete` auf (V1). Das Binary hängt nur von `libSystem` und `libresolv` ab.
- **[Gemessen]** Der Schlüsselbund trägt das aufrufende Binary als einziges vertrautes Programm für das Lesen ein. Die Partitionsliste nennt seinen `cdhash` (V2).
- **[Gemessen]** Ein anderes Programm löst beim Lesen einen Dialog aus (V4).
- **[Doku]** `purego` ruft C-Funktionen ohne CGO auf. darwin auf amd64 und arm64 ist Tier 1 und läuft mit `CGO_ENABLED=0` ([purego auf pkg.go.dev](https://pkg.go.dev/github.com/ebitengine/purego)). Die aktuelle Version ist v0.11.3 vom November 2026.
- **[Doku]** Die Standardbibliothek von Go ruft Security.framework auf darwin selbst ohne CGO auf, für die Prüfung von Zertifikaten in `crypto/x509`. Sie nutzt dafür `//go:cgo_import_dynamic` ([security.go](https://go.googlesource.com/go/+/HEAD/src/crypto/x509/internal/macos/security.go)). Das Paket ist intern. Dieser Weg steht nur der Standardbibliothek offen.
- **[Doku]** Ohne eigene Zugriffsliste (`kSecAttrAccess`) vertraut ein neuer Eintrag nur dem aufrufenden Prozess. Die Zugriffsliste entsteht beim Anlegen ([Apple-Forum, Quinn, Thread 836816](https://developer.apple.com/forums/thread/836816)). V2 bestätigt das.
- **Schwächen:** `purego` ist eine neue Abhängigkeit im Daemon. Fehler im Umgang mit Zeigern stürzen ab, statt einen Fehler zu melden. `go vet` meldet zwei Stellen mit `unsafe.Pointer`. Die Funktionen für Zugriffslisten (`SecKeychainItemCopyAccess`, `SecAccessCopyACLList`) sind veraltet, funktionieren aber auf macOS 27.2.

### Weg B: `/usr/bin/security`

- **[Gemessen]** `security add-generic-password` trägt `security` als vertrautes Programm ein, Partition `apple-tool:` (V9). Jeder Prozess des Benutzers liest den Wert danach still (V8). Für dieses Ziel ist Weg B damit wertlos.
- **[Gemessen]** Mit `-T <pfad>` nennt die Zugriffsliste nur das genannte Programm (V10). Die Partitionsliste bleibt `apple-tool:`. **[Annahme]** `a38` ist kein Werkzeug von Apple. Ein Lesen durch `a38` löst deshalb den Dialog der Partitionsliste aus. Der Dialog verlangt das Passwort des Schlüsselbunds.
- **[Gemessen]** `-w` ohne Wert liest den Wert von stdin, wenn stdin kein Terminal ist (V7). `security -i` liest ganze Befehle von stdin (V11). Beide Varianten halten den Wert aus argv heraus.
- **[Doku]** Die Man-Page empfiehlt `-w` am Ende ohne Wert, damit `security` fragt ([security(1)](https://keith.github.io/xcode-man-pages/security.1.html)). **[Annahme]** `-w <wert>` als Argument macht den Wert in der Prozessliste sichtbar. Ein Prozess desselben Benutzers liest argv jedes anderen Prozesses dieses Benutzers.
- **[Doku]** „Immer erlauben“ lässt das Programm den Wert künftig ohne Rückfrage lesen ([Apple Support: Schlüsselbundverwaltung](https://support.apple.com/en-us/guide/keychain-access/kyca17141/11.0/mac/15.0)). Bestätigt der Mensch das für `security`, liest danach jeder Prozess den Wert still über `security`.
- **[Doku]** `github.com/zalando/go-keyring` (v0.2.8) nutzt auf macOS genau diesen Weg. Seit v0.2.0 schreibt es den Befehl über `security -i` auf stdin, der Wert steht also nicht in argv ([keyring_darwin.go](https://github.com/zalando/go-keyring/blob/master/keyring_darwin.go)). Die Einträge gehören trotzdem `security`. Die Bibliothek erbt damit die Schwäche aus V8.

### Weg C: Helfer in der signierten App

- **[Annahme]** Die Swift-App `Passierschein` oder ein Helfer in ihrem Bundle hält den Eintrag. Der Daemon startet den Helfer und liest das Token von dessen stdout. Das funktioniert technisch.
- **[Annahme]** Der Helfer weiß nicht, wer ihn startet. Jeder Prozess des Benutzers kann den Helfer starten und das Token von stdout lesen. Der Helfer muss deshalb den Aufrufer prüfen (Code-Signatur des Elternprozesses über `SecCodeCopyGuestWithAttributes` mit der Audit-Token). Das ist eigener sicherheitskritischer Code in Swift.
- **[Annahme]** Der Gewinn gegenüber Weg A mit Developer ID ist klein: Beide binden den Eintrag an eine Signatur des Teams. Weg C braucht zusätzlich einen Kanal und eine Prüfung des Aufrufers.
- Weg C lohnt sich nur zusammen mit dem Data-Protection-Schlüsselbund und Touch ID (Abschnitt 3).

### Weitere Go-Bibliotheken

- **[Doku]** `github.com/keybase/go-keychain` nutzt CGO (`import "C"`) ([go-keychain](https://github.com/keybase/go-keychain)).
- **[Doku]** `github.com/99designs/keyring` baut das Schlüsselbund-Backend nur mit `darwin && cgo` ([keyring](https://github.com/99designs/keyring)). Der gepflegte Fork `github.com/byteness/keyring` nutzt `byteness/go-keychain`, ebenfalls mit CGO ([byteness/keyring](https://github.com/byteness/keyring)).
- **[Doku]** Es gibt junge Bindungen über `purego`: `github.com/go-macos/keychain` v0.3.1 vom Oktober 2026 ([go-macos/keychain](https://github.com/go-macos/keychain)) und `github.com/Fabianexe/purekeychain` v1.0.1 ([purekeychain](https://github.com/Fabianexe/purekeychain)). Beide haben kaum Nutzer. **[Annahme]** `go-macos/keychain` bietet Touch ID an, setzt aber `kSecUseDataProtectionKeychain` nicht. Touch ID wirkt dort deshalb vermutlich nicht.
- Für `CGO_ENABLED=0` bleibt `purego`. Der eigene Code ist klein (V1 braucht etwa 20 Funktionen). Der Spike empfiehlt eigenen Code statt einer jungen Bibliothek im Daemon.

## 2. Bindung an das Programm

### Dateibasierter Schlüsselbund und Zugriffsliste

- **[Doku]** macOS hat zwei Schlüsselbund-Implementierungen: den dateibasierten Schlüsselbund mit Zugriffslisten und den Data-Protection-Schlüsselbund mit Zugriffsgruppen. Ohne `kSecUseDataProtectionKeychain` landet ein Eintrag eines Mac-Programms im dateibasierten Schlüsselbund ([TN3137](https://developer.apple.com/documentation/technotes/tn3137-on-mac-keychains)).
- **[Doku]** Die Zugriffsliste nennt vertraute Programme. Der Schlüsselbund erkennt ein Programm an der festgelegten Anforderung (designated requirement) der Code-Signatur. Ohne stabile Anforderung erkennt der Schlüsselbund Version N+1 nicht als Version N ([Apple-Forum, Quinn, Thread 746931](https://developer.apple.com/forums/thread/746931)).
- **[Doku]** Ad-hoc signierter Code hat eine Anforderung, die an genau diese Version gebunden ist ([TN3127](https://developer.apple.com/documentation/technotes/tn3127-inside-code-signing-requirements)).
- **[Doku]** Die Partitionsliste (seit macOS 10.12) verhindert, dass Code von Team A die Einträge von Team B liest ([Thread 746931](https://developer.apple.com/forums/thread/746931)). Ein Wechsel der Partitionsliste verlangt das Passwort des Schlüsselbunds ([security(1)](https://keith.github.io/xcode-man-pages/security.1.html)).
- **[Gemessen]** Ein ad-hoc signiertes Programm hat als Anforderung nur seinen `cdhash`. Seine Partition heißt `cdhash:<hash>` (V2). Ein Build aus demselben Quelltext hat denselben Hash (V12). Jede Änderung am Code oder an den Abhängigkeiten ändert den Hash.
- **[Annahme]** Nach einem Update von `a38` passt der Hash nicht mehr. `a38` löst beim ersten Lesen einen Dialog aus. Der Dialog nennt `a38`. Bestätigt der Mensch mit „Immer erlauben“, trägt macOS den neuen Build ein. Zusätzlich passt die Partitionsliste (`cdhash:…`) nicht mehr. Der Dialog verlangt dann das Passwort des Schlüsselbunds.
- **[Doku]** Mit einer Developer ID lautet die Anforderung etwa `identifier "…" and anchor apple generic and certificate leaf[subject.OU] = "<TEAM>"` ([TN3127](https://developer.apple.com/documentation/technotes/tn3127-inside-code-signing-requirements)). Jeder Build desselben Teams mit demselben Bezeichner erfüllt sie. Jeder Code mit stabiler Signatur kann den dateibasierten Schlüsselbund ohne eingeschränkte Berechtigung nutzen ([Thread 695598](https://developer.apple.com/forums/thread/695598)).
- **[Annahme]** Die Partition heißt dann `teamid:<TEAM>`. Die Werte `apple-tool:` und `teamid:` nennt nur die Community; `apple-tool:` und `cdhash:` sind hier gemessen (V2, V9).
- **[Annahme]** Dann liest nur `a38` mit Developer ID ohne Dialog. Ein Update von `a38` liest ohne Dialog. Jedes andere Programm, auch ein Programm mit dem Namen `a38`, löst einen Dialog aus.

### Data-Protection-Schlüsselbund und Zugriffsgruppen

- **[Gemessen]** Ohne Berechtigung lehnt der Data-Protection-Schlüsselbund ab: `-34018` (V5).
- **[Gemessen]** Mit `keychain-access-groups` und Ad-hoc-Signatur beendet macOS das Programm beim Start (V6).
- **[Doku]** `keychain-access-groups` ist eine eingeschränkte Berechtigung. Ein Provisioning-Profil muss sie freigeben. Ein einzelnes Programm kann keine eingeschränkte Berechtigung beanspruchen, weil es kein Profil enthalten kann ([TN3125](https://developer.apple.com/documentation/technotes/tn3125-inside-code-signing-provisioning-profiles)). Apple beschreibt dafür eine Bundle-Struktur `X.app/Contents/{Info.plist, MacOS/X, embedded.provisionprofile}` um das Programm ([Signing a daemon with a restricted entitlement](https://developer.apple.com/documentation/xcode/signing-a-daemon-with-a-restricted-entitlement)).
- **[Doku]** Der Data-Protection-Schlüsselbund steht auch Programmen mit Developer ID außerhalb des App Store offen. Die Gruppe beginnt mit der Team-ID ([Thread 826107](https://developer.apple.com/forums/thread/826107)). Er steht nur Programmen in einem Benutzerkontext offen; ein LaunchAgent ist ein solcher Kontext ([TN3137](https://developer.apple.com/documentation/technotes/tn3137-on-mac-keychains)).
- **[Doku]** macOS gibt einem Prozess die Berechtigungen, die die Signatur seines eigenen Programms nennt. Ein Hilfsprogramm in einer App gilt nicht als gebündelter Code. Ein Programm mit eingeschränkter Berechtigung außerhalb der Hauptdatei braucht eine eigene App-ähnliche Struktur ([Creating distribution-signed code for the Mac](https://developer.apple.com/documentation/xcode/creating-distribution-signed-code-for-the-mac)).
- **[Doku]** Apple schlägt für ein Kommandozeilenprogramm vor: das Programm in eine App-ähnliche Hülle legen und im `PATH` einen symbolischen Link auf das Programm in der Hülle anlegen ([Thread 836816](https://developer.apple.com/forums/thread/836816)).
- Folge: `Passierschein.app/Contents/MacOS/a38` erbt die Berechtigungen der App nicht. Für eine Zugriffsgruppe bräuchte `a38` eine eigene Hülle, etwa `Passierschein.app/Contents/Helpers/a38.app` mit eigenem Profil. **[Annahme]** Ob diese Hülle dieselbe Gruppe wie die App nutzen kann, ist offen.
- Folge: Für das Ziel „nur `a38` liest still“ genügt der dateibasierte Schlüsselbund mit Developer ID. Der Data-Protection-Schlüsselbund ist nur für Touch ID nötig.

## 3. Touch ID beim Lesen

- **[Doku]** Ein Eintrag mit `kSecAttrAccessControl` und `.userPresence` oder `.biometryCurrentSet` verlangt Touch ID oder das Passwort bei jedem Lesen ([Accessing keychain items with Face ID or Touch ID](https://developer.apple.com/documentation/localauthentication/accessing-keychain-items-with-face-id-or-touch-id)).
- **[Doku]** Auf dem Mac gilt das nur im Data-Protection-Schlüsselbund ([TN3137](https://developer.apple.com/documentation/technotes/tn3137-on-mac-keychains)). Der braucht die Berechtigung aus Abschnitt 2.
- **[Doku]** `.userPresence` heißt Biometrie oder Gerätepasswort ([userPresence](https://developer.apple.com/documentation/security/secaccesscontrolcreateflags/userpresence)). Ein Lesen ohne Dialog verlangt ein `LAContext` mit `interactionNotAllowed`; dann endet es mit `errSecInteractionNotAllowed` ([kSecUseAuthenticationUIFail](https://developer.apple.com/documentation/security/ksecuseauthenticationuifail), [kSecUseAuthenticationContext](https://developer.apple.com/documentation/security/ksecuseauthenticationcontext)).
- **[Annahme]** Ein Kommandozeilenprogramm kann den Dialog von Touch ID anzeigen. Die App `Passierschein` zeigt ihn bereits über `LocalAuthentication`. Das Hindernis ist die Berechtigung, nicht die fehlende Oberfläche.
- **[Annahme]** Ein LaunchAgent, der beim Anmelden startet, liest das Token beim Start. Der Dialog erscheint dann gleich nach der Anmeldung, ohne Zusammenhang für den Menschen. Bis zur Bestätigung wartet der Daemon. Lehnt der Mensch ab oder ist er nicht da, startet das Backend nicht. Jedes Lesen nach einem Neustart des Daemons fordert wieder Touch ID.
- **[Annahme]** Der Gewinn ist klein. Mit Developer ID liest ohnehin nur `a38` still. Touch ID schützt zusätzlich nur gegen einen Angreifer, der Code in `a38` einschleust. Dieser Angreifer liest das Token auch nach dem Lesen aus dem Speicher.
- Folge: Touch ID beim Lesen des Tokens empfiehlt der Spike nicht. Touch ID bleibt bei der Freigabe ({{ ref("D7") }}).

## 4. Der Befehl für den Menschen

Vorschlag: eine Gruppe `a38 token` mit drei Befehlen. Nur der Build für darwin enthält sie. Der linux-Build enthält kein 1Password-Backend ({{ ref("D42") }}).

| Befehl | Verhalten |
|---|---|
| `a38 token set` | Liest das Token und legt es im Schlüsselbund ab. Ein vorhandener Eintrag wird ersetzt. |
| `a38 token status` | Zeigt, ob ein Eintrag existiert, das Datum der letzten Änderung, die Quelle (Schlüsselbund oder Datei) und die Zugriffsliste. Liest den Wert nicht. |
| `a38 token delete` | Löscht den Eintrag nach einer Rückfrage im Terminal. |

Regeln für `a38 token set`:

- Das Token kommt nie als Argument. Es gibt keinen Schalter für den Wert und keine Umgebungsvariable.
- Ist stdin ein Terminal, liest der Befehl ohne Echo. `golang.org/x/sys/unix` schaltet das Echo über `termios` ab. Das braucht kein CGO.
- Ist stdin kein Terminal, liest der Befehl stdin bis zum Ende, höchstens 4 KiB. So geht die Übernahme aus der alten Datei: `a38 token set < ~/.config/a38/onepassword-token`. Danach löscht der Mensch die Datei.
- Der Befehl entfernt nur ein abschließendes Zeilenende. Ein Leerzeichen oder ein Steuerzeichen im Token ist ein Fehler.
- Die Prüfung ohne Ausgabe: Länge in einem plausiblen Bereich, Präfix `ops_`. **[Annahme]** Tokens von Service Accounts beginnen mit `ops_`; Stefan bestätigt das am echten Token.
- Die Ausgabe nennt nie den Wert. Sie nennt die Länge und einen kurzen Fingerabdruck (die ersten 8 Hex-Zeichen von SHA-256). Der Mensch erkennt damit eine Rotation.
- Eine Fehlermeldung nennt nie den Wert.
- Der Eintrag: Klasse `genp`, Dienst `a38.onepassword`, Konto `service-account`, Bezeichnung „a38: 1Password Service Account“. Der Name nennt nur den Zweck, keinen Vault.
- `a38` legt den Eintrag selbst über `purego` an. Die Zugriffsliste nennt dann `a38`. `a38 token set` und `a38 serve` sind dasselbe Binary und lesen ohne Dialog.
- Der echte Test des Tokens gegen 1Password bleibt beim Daemon: Er meldet beim Start `backend_unavailable`, wenn 1Password das Token ablehnt.

Rotation:

1. Stefan erzeugt in 1Password ein neues Token mit einer Frist für das alte Token.
2. `a38 token set` ersetzt den Eintrag.
3. `a38 service restart` startet den Daemon neu. Der Daemon liest das Token nur beim Start.
4. `a38 doctor` zeigt den neuen Fingerabdruck und einen erfolgreichen Start des Backends.

Was `a38 doctor` zeigt:

- Quelle des Tokens: Schlüsselbund, Datei oder keine.
- Existenz des Eintrags, Datum der letzten Änderung. Kein Lesen des Werts.
- Die Zugriffsliste: Ein Eintrag, der `security` oder ein anderes Programm als `a38` nennt, ist ein Befund. Der Befund nennt das fremde Programm.
- Die Partitionsliste: `cdhash:` ergibt einen Hinweis „Nach jedem Update fragt macOS nach“. `teamid:` mit der erwarteten Team-ID ist in Ordnung.
- Die Signatur von `a38` selbst: ad-hoc oder Developer ID mit Team-ID.
- Eine noch vorhandene Token-Datei ist ein Befund, wenn der Schlüsselbund die Quelle ist.

Wo das Token sonst landen kann:

- Shell-History und Prozessliste: nie, weil das Token nie ein Argument ist.
- Zwischenablage: Der Mensch kopiert das Token meist aus 1Password. **[Annahme]** Jeder Prozess des Benutzers liest die Zwischenablage, solange das Token dort steht. 1Password leert die Zwischenablage nach einer einstellbaren Zeit. Das Einfügen per Hand bleibt ein kurzes Fenster.

## 5. Daemon als LaunchAgent

- **[Doku]** Ein LaunchAgent läuft im Benutzerkontext. Ein LaunchDaemon läuft im Systemkontext und sieht nur den System-Schlüsselbund ([Thread 685967](https://developer.apple.com/forums/thread/685967), [Thread 794151](https://developer.apple.com/forums/thread/794151)).
- **[Doku]** Ein Prozess ohne Oberfläche, der einen Dialog bräuchte, bekommt `errSecInteractionNotAllowed` ([Thread 794151](https://developer.apple.com/forums/thread/794151)).
- **[Annahme]** macOS entsperrt den Anmelde-Schlüsselbund bei der Anmeldung mit dem Passwort des Benutzers.
- **[Annahme]** Der erste Start nach der Anmeldung findet einen entsperrten Schlüsselbund. Der Daemon liest das Token ohne Dialog, wenn die Zugriffsliste ihn nennt.
- **[Annahme]** Ist der Schlüsselbund gesperrt oder fehlt ein vertrautes Programm, kann der Daemon keinen Dialog sinnvoll erwarten. Er braucht eine Frist. Nach der Frist startet er ohne Backend, meldet den Grund im Status und zeigt eine Meldung über die App (`notice`). Er liefert dann nichts aus (fail closed, {{ ref("D35") }}).
- **[Annahme]** In einer SSH-Sitzung ohne Anmeldung am Bildschirm ist der Anmelde-Schlüsselbund gesperrt. Ein Lesen scheitert mit `errSecInteractionNotAllowed` (`-25308`) oder fordert in der Sitzung am Bildschirm einen Dialog an. `a38 token set` über SSH ist deshalb nicht vorgesehen. Der Daemon läuft als LaunchAgent ohnehin in der Sitzung am Bildschirm.
- Der Daemon liest das Token nur beim Start. Ein gesperrter Bildschirm danach ändert nichts. Das deckt sich mit D51.

## 6. Bedrohungen, die bleiben

Was der Schlüsselbund leistet:

- Ein Prozess desselben Benutzers liest das Token nicht mehr mit `cat`. Er braucht entweder ein Programm, das die Zugriffsliste erfüllt, oder einen Dialog, den der Mensch bestätigt.
- Mit Developer ID erfüllt nur ein Programm des Teams die Zugriffsliste. Ein Agent kann so ein Programm nicht bauen.
- Das Token gelangt nicht mehr als Datei in eine Datensicherung im Klartext.

Was der Schlüsselbund nicht leistet:

| Angriff | Wirkung | Gegenmittel |
|---|---|---|
| Der Agent liest das Token aus dem Speicher des laufenden Daemons (Debugger, `task_for_pid`). | Token offen. | **[Doku]** Ein Debugger bekommt den Task-Port eines Prozesses ohne `get-task-allow` nicht ([com.apple.security.cs.debugger](https://developer.apple.com/documentation/bundleresources/entitlements/com.apple.security.cs.debugger)). Die Hardened Runtime schützt gegen das Einschleusen von Code und das Ändern des Speichers ([Hardened Runtime](https://developer.apple.com/documentation/security/hardened-runtime)). Ein ad-hoc signiertes `a38` ohne Hardened Runtime hat diesen Schutz nicht. **[Annahme]** Ohne Hardened Runtime verlangt macOS für das Anhängen meist eine Bestätigung als Administrator. |
| Der Agent startet `a38` mit `DYLD_INSERT_LIBRARIES` und eigenem Code. | Der fremde Code läuft als `a38` und erfüllt die Zugriffsliste. Token offen. | **[Doku]** Die Hardened Runtime ignoriert DYLD-Variablen ohne `allow-dyld-environment-variables` ([Doku](https://developer.apple.com/documentation/bundleresources/entitlements/com.apple.security.cs.allow-dyld-environment-variables)). Die Library Validation lädt nur Bibliotheken von Apple oder vom eigenen Team ([Doku](https://developer.apple.com/documentation/bundleresources/entitlements/com.apple.security.cs.disable-library-validation)). Ohne Hardened Runtime kein Schutz. |
| Der Agent ersetzt das Binary von `a38` oder ändert die plist des LaunchAgents. | Mit Ad-hoc-Signatur passt der Hash nicht. Mit Developer ID passt die Signatur nicht. Beide Male erscheint ein Dialog, der `a38` nennt. | Der Mensch bestätigt nie einen Dialog für den Eintrag von `a38`. Mit Developer ID fragt das echte `a38` nie. Jeder Dialog ist dann ein Alarm. |
| Der Agent baut ein Programm mit dem Namen `a38` und liest den Eintrag. | Dialog mit dem Namen `a38`. Bestätigt der Mensch, ist das Token offen. | wie oben; ohne Developer ID fragt auch das echte `a38` nach jedem Update. Der Mensch kann beide nicht unterscheiden. |
| Der Agent fragt den Daemon nach Secrets. | Das ist der vorgesehene Weg mit Freigabe ({{ ref("D7") }}). | unverändert |
| Der Agent löscht oder ersetzt den Eintrag. | Kein Backend oder ein fremdes Token. Kein Abfluss des echten Tokens. | **[Annahme]** Das Löschen braucht kein vertrautes Programm. `a38 doctor` zeigt den Fingerabdruck. |
| Der Agent liest die Zwischenablage beim Einfügen. | Token offen. | kurzes Fenster; Hinweis in der Anleitung |
| Der Agent nutzt 1Password direkt (Desktop-App, `op`). | unabhängig vom Token | {{ ref("RI5") }}, unverändert |

Folge für die Security Boundaries: Die Zeile „Das Token des Service Accounts ist vor anderen Prozessen des Benutzers geschützt“ unter B3 wechselt mit Developer ID und Hardened Runtime von **not-provided** zu **inherited** (macOS erzwingt die Zugriffsliste). Ohne Developer ID bleibt sie **not-provided**: Der Mensch kann den Dialog nach einem Update nicht von einem Angriff unterscheiden. {{ ref("RI5") }} bleibt bestehen. Ein Agent kann `a38` in den Stufen 1 bis 4 weiter auf anderen Wegen angreifen.

## Vergleich der Wege

| Weg | ohne CGO | Was ein fremder Prozess bekommt | Nach einem Update von `a38` | Aufwand |
|---|---|---|---|---|
| Token-Datei `0600` (D51) | ja | den Wert, still | – | vorhanden |
| A: `purego`, dateibasierter Schlüsselbund, ad-hoc | ja | einen Dialog (gemessen) | Dialog für `a38` | klein |
| A: `purego`, dateibasierter Schlüsselbund, Developer ID und Hardened Runtime | ja | einen Dialog (Annahme) | kein Dialog (Annahme) | klein, plus Signatur aus {{ ref("G5") }} |
| B: `/usr/bin/security` | ja | den Wert, still, über `security` (gemessen) | – | klein |
| B mit `-T a38` | ja | einen Dialog | Dialog für `a38`, mit Passwort (Annahme) | klein |
| C: Helfer in `Passierschein.app` | ja | was der Helfer ausgibt; Prüfung des Aufrufers nötig | kein Dialog mit Developer ID | mittel bis groß |
| Data-Protection-Schlüsselbund mit Touch ID | ja über `purego`, aber Profil nötig | Fehler oder Touch ID | kein Dialog | groß; Bundle für `a38`, Profil |

## Empfehlung

### Stufe 1: jetzt, ohne Developer ID

- `internal/keychain` (nur darwin) über `purego`: `Set`, `Get`, `Delete`, `Status` für einen festen Eintrag. Das Paket gibt den Wert nur als Typ mit der Ausgabe `[redacted]` zurück.
- `a38 token set|status|delete` nach Abschnitt 4.
- Konfiguration: `backend.onepassword.token_source` mit `keychain` als Standard und `file` als Rückfall. Der Schlüssel `token_file` aus D51 bleibt für `file`.
- `a38 doctor` nach Abschnitt 4.
- Wert: Der Agent liest das Token nicht mehr mit `cat`. Ein Lesen über `security` geht nicht still. Ein fremdes Programm löst einen Dialog aus.
- Grenze: Nach jedem Update fragt macOS bei `a38` nach. Der Mensch gewöhnt sich an diesen Dialog. Die Security Boundaries bleiben bei **not-provided**.
- Für die Entwicklung bleibt die Token-Datei der bequeme Weg. Jeder Neubau würde sonst einen Dialog auslösen.

### Stufe 2: mit dem signierten Release ({{ ref("G5") }})

- `a38` mit Developer ID, Hardened Runtime und Zeitstempel signieren. Fester Bezeichner, zum Beispiel die Bundle-ID der App mit `.a38`.
- Keine Berechtigung `get-task-allow`. Keine Ausnahme der Hardened Runtime.
- `a38 doctor` prüft die Team-ID von `a38` und die Partitionsliste des Eintrags.
- Anleitung: „Bestätige nie einen Dialog für den Eintrag `a38.onepassword`.“
- Danach stuft das Design die Zeile unter B3 neu ein.

### Stufe 3: nur bei Bedarf

- Data-Protection-Schlüsselbund mit Zugriffsgruppe und Touch ID. Das braucht eine Bundle-Struktur mit Provisioning-Profil für `a38` oder einen Helfer mit Prüfung des Aufrufers. Der Spike empfiehlt diese Stufe nicht.

### Folgen für das Design

- D51 (Token-Datei) wird ergänzt: Die Datei bleibt als Rückfall `token_source: file`. Ein neuer Beschluss im Lauf 2 legt den Schlüsselbund als Standard fest. Stefan entscheidet; der Spike legt kein Artefakt an.
- `CGO_ENABLED=0` bleibt. `purego` ist die einzige neue Abhängigkeit.
- {{ ref("G3") }} verliert die Frage „Wo liegt das Token?“.
- {{ ref("G5") }} bekommt eine neue Frage: Signiert der Release-Job auch `a38` mit Hardened Runtime, und mit welchem Bezeichner?

## Risiken

- `purego` ist eine fremde Abhängigkeit im Prozess mit dem höchsten Vertrauen. Ein Fehler im Umgang mit Zeigern stürzt den Daemon ab.
- Die Funktionen für Zugriffslisten sind veraltet. Apple kann sie entfernen. `a38 doctor` verliert dann nur die Anzeige der Zugriffsliste, nicht das Lesen.
- Ohne Developer ID gewöhnt sich der Mensch an Dialoge. Ein Dialog eines Angreifers sieht gleich aus.
- Die Aussagen zu Developer ID, Partition `teamid:` und Update ohne Dialog sind nicht gemessen.
- Bleibt `a38` ein einzelnes Binary außerhalb des Bundles, braucht es eine eigene Signatur mit Developer ID. quill signiert ein einzelnes Binary ({{ ref("D46") }}). Die Hardened Runtime und der Bezeichner sind dort zu prüfen.

## Testplan für den Praxistest

Voraussetzungen: Developer-ID-Zertifikat im Schlüsselbund, `spikes/token-keychain/` gebaut. Alle Einträge heißen `a38-spike-…`. Stefan bestätigt **keinen** Dialog mit „Erlauben“; er wählt „Verweigern“ und notiert den Text.

1. Signierte Variante bauen.

    ```sh
    cd spikes/token-keychain
    CGO_ENABLED=0 go build -o bin/kcspike-a .
    CGO_ENABLED=0 go build -ldflags "-X main.variant=b" -o bin/kcspike-b .
    for b in a b; do
      cp bin/kcspike-$b bin/kcspike-$b-signed
      codesign -f -s "Developer ID Application" -o runtime --timestamp \
        --identifier example.a38spike bin/kcspike-$b-signed
      codesign -d -r- bin/kcspike-$b-signed
    done
    ```

    Erwartung: Die Anforderung nennt `anchor apple generic` und die Team-ID.

2. Eintrag mit dem signierten Programm anlegen und die Zugriffsliste lesen.

    ```sh
    printf %s spike-dummy-wert-1 | bin/kcspike-a-signed add -s a38-spike-signed
    bin/kcspike-a-signed read -s a38-spike-signed
    bin/kcspike-a-signed acl  -s a38-spike-signed
    ```

    Erwartung: Lesen ohne Dialog. Partitionsliste `teamid:<TEAM>`.

3. Update simulieren: der zweite signierte Build liest.

    ```sh
    bin/kcspike-b-signed read -s a38-spike-signed
    ```

    Erwartung: kein Dialog. Das belegt das Update ohne Dialog.

4. Fremdes Programm liest.

    ```sh
    bin/kcspike-b read -s a38-spike-signed
    security find-generic-password -s a38-spike-signed -w
    ```

    Erwartung: je ein Dialog. Stefan notiert den Text und wählt „Verweigern“. Notieren: Verlangt der Dialog das Passwort?

5. Ad-hoc-Update ohne Developer ID.

    ```sh
    printf %s spike-dummy-wert-1 | bin/kcspike-a add -s a38-spike-adhoc
    bin/kcspike-b read -s a38-spike-adhoc
    ```

    Erwartung: Dialog für `kcspike-b`. Notieren: Text, Passwortfeld. „Verweigern“.

6. `security` mit `-T`.

    ```sh
    printf 'spike-dummy-wert-1\nspike-dummy-wert-1\n' | \
      security add-generic-password -s a38-spike-t -a a38-spike -T "$PWD/bin/kcspike-a" -w
    bin/kcspike-a read -s a38-spike-t
    ```

    Erwartung: Dialog der Partitionsliste mit Passwortfeld. „Verweigern“.

7. LaunchAgent bei der Anmeldung. Eine plist `~/Library/LaunchAgents/example.a38spike.plist` startet `bin/kcspike-a-signed read -s a38-spike-signed` mit `RunAtLoad` und schreibt stdout nach `/tmp/a38spike.log`.

    ```sh
    launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/example.a38spike.plist
    # abmelden, anmelden
    cat /tmp/a38spike.log
    launchctl bootout gui/$(id -u)/example.a38spike
    rm ~/Library/LaunchAgents/example.a38spike.plist /tmp/a38spike.log
    ```

    Erwartung: `status 0` nach der Anmeldung, ohne Dialog.

8. SSH-Sitzung.

    ```sh
    ssh localhost "$PWD/bin/kcspike-a-signed read -noui -s a38-spike-signed"
    ```

    Einmal mit Anmeldung am Bildschirm, einmal nach einem Neustart ohne Anmeldung. Notieren: Status, Dialog am Bildschirm.

9. Hardened Runtime gegen Debugger und Einschleusen.

    ```sh
    bin/kcspike-a-signed read -s a38-spike-signed & lldb -p $!
    DYLD_INSERT_LIBRARIES=/usr/lib/libgmalloc.dylib bin/kcspike-a-signed read -s a38-spike-signed
    DYLD_INSERT_LIBRARIES=/usr/lib/libgmalloc.dylib bin/kcspike-a read -s a38-spike-adhoc
    ```

    Erwartung: `lldb` scheitert beim signierten Build. Die Variable wirkt nur beim ad-hoc signierten Build ohne Hardened Runtime.

10. Touch ID beim Lesen (nur falls Stufe 3 in Frage kommt). Eine Swift-Test-App im Bundle mit Provisioning-Profil und `keychain-access-groups` legt `a38-spike-touchid` mit `SecAccessControlCreateWithFlags(.userPresence)` an. Notieren: Dialog beim Lesen aus dem Terminal, aus einem LaunchAgent und über SSH.

11. Aufräumen und Nachweis.

    ```sh
    for s in a38-spike-signed a38-spike-adhoc a38-spike-t a38-spike-touchid; do
      security delete-generic-password -s "$s" >/dev/null 2>&1
      security find-generic-password -s "$s" >/dev/null 2>&1 && echo "left: $s"
    done
    ```

    Erwartung: keine Ausgabe.

12. Echtes Token (mit Stefans Freigabe für 1Password): `a38 token set` aus der Zwischenablage, `a38 token status`, Start des Daemons, `a38 doctor`, Rotation nach Abschnitt 4.

## Quellen

- [TN3137: On Mac keychain APIs and implementations](https://developer.apple.com/documentation/technotes/tn3137-on-mac-keychains)
- [Signing a daemon with a restricted entitlement](https://developer.apple.com/documentation/xcode/signing-a-daemon-with-a-restricted-entitlement)
- [Accessing keychain items with Face ID or Touch ID](https://developer.apple.com/documentation/localauthentication/accessing-keychain-items-with-face-id-or-touch-id)
- [Hardened Runtime](https://developer.apple.com/documentation/security/hardened-runtime)
- [TN3125: Inside Code Signing: Provisioning Profiles](https://developer.apple.com/documentation/technotes/tn3125-inside-code-signing-provisioning-profiles)
- [TN3127: Inside Code Signing: Requirements](https://developer.apple.com/documentation/technotes/tn3127-inside-code-signing-requirements)
- [Creating distribution-signed code for the Mac](https://developer.apple.com/documentation/xcode/creating-distribution-signed-code-for-the-mac)
- [kSecUseDataProtectionKeychain](https://developer.apple.com/documentation/security/ksecusedataprotectionkeychain), [errSecMissingEntitlement](https://developer.apple.com/documentation/security/errsecmissingentitlement), [kSecAttrAccessControl](https://developer.apple.com/documentation/security/ksecattraccesscontrol)
- Apple-Developer-Forum (Quinn, Apple DTS): [836816](https://developer.apple.com/forums/thread/836816), [826107](https://developer.apple.com/forums/thread/826107), [746931](https://developer.apple.com/forums/thread/746931), [695598](https://developer.apple.com/forums/thread/695598), [794151](https://developer.apple.com/forums/thread/794151); Apple DTS: [685967](https://developer.apple.com/forums/thread/685967)
- [Apple Support: Schlüsselbundverwaltung, Zugriff erlauben](https://support.apple.com/en-us/guide/keychain-access/kyca17141/11.0/mac/15.0)
- [Go-Issue 42684: Code-Signatur auf darwin/arm64](https://golang.org/issue/42684)
- [security(1)](https://keith.github.io/xcode-man-pages/security.1.html)
- [purego](https://pkg.go.dev/github.com/ebitengine/purego)
- [go-keyring](https://github.com/zalando/go-keyring/blob/master/keyring_darwin.go), [go-keychain](https://github.com/keybase/go-keychain), [99designs/keyring](https://github.com/99designs/keyring), [byteness/keyring](https://github.com/byteness/keyring), [go-macos/keychain](https://github.com/go-macos/keychain), [purekeychain](https://github.com/Fabianexe/purekeychain)
- [Go: crypto/x509/internal/macos/security.go](https://go.googlesource.com/go/+/HEAD/src/crypto/x509/internal/macos/security.go)
- [Spike 1Password-Go-SDK](10-1password-sdk.md), [Spike Signatur der Swift-App](30-signatur-swift-app.md)
