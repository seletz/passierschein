---
title: "Spike: Signatur und Notarisierung der Swift-App"
description: "Der Spike klärt, mit welchen Werkzeugen und auf welchem Runner das Projekt die Swift-App signiert, notarisiert und in das Release einbindet."
tags:
  - Spike
  - Release
---
# Spike: Signatur und Notarisierung der Swift-App

Stand: 2026-11-11. Bezug: Issue seletz/passierschein#5.

## Frage

Das Projekt liefert zwei Dinge für macOS aus:

- das Kommandozeilenprogramm (CLI, Command Line Interface) `a38`, geschrieben in Go,
- die Swift-App, die der Go-Daemon startet und die eine Freigabe mit Touch ID einholt.

goreleaser signiert und notarisiert die darwin-Binaries von `a38` auf einem Linux-Runner. goreleaser nutzt dafür `notarize.macos` und damit quill.

Der Spike beantwortet diese Fragen:

1. Signiert und notarisiert das Projekt die Swift-App mit quill auf dem Linux-Runner oder mit `codesign`, `notarytool` und `stapler` auf einem macOS-Runner?
2. Wie fügt sich die Swift-App in goreleaser, die mise-Tasks, die SBOM (Software Bill of Materials, Stückliste der Software) und die Provenance-Attestierung ein?

## Ergebnis

Das Projekt baut die Swift-App als App-Bundle und signiert sie auf einem macOS-Runner mit `codesign`, `notarytool` und `stapler`. Ein macOS-Runner ist in jedem Fall nötig, weil der Linux-Runner eine Swift-App mit AppKit nicht bauen kann. Die Signatur mit den Werkzeugen von Apple kostet auf diesem Runner nur wenige zusätzliche Minuten. quill signiert App-Bundles nur im Entwicklungszweig `main`, nicht in einem Release, und goreleaser bietet App-Bundles nur in der Pro-Version an. Ein einzelnes Mach-O mit eingebetteter `Info.plist` funktioniert technisch, erhält aber keine Bundle-ID bei LaunchServices und kein angeheftetes Ticket. Der macOS-Job läuft vor dem goreleaser-Job und übergibt das signierte App-Bundle als Workflow-Artefakt. goreleaser packt das App-Bundle in die darwin-Archive neben `a38`, sodass Prüfsummen, SBOM und Attestierung das App-Bundle abdecken.

## Befunde

Jeder Befund trägt einen Status:

- **Quelle:** Eine Primärquelle belegt die Aussage. Der Link steht beim Befund.
- **Gemessen:** Ein lokaler Versuch belegt die Aussage. Umgebung: macOS 27.2, Xcode 27.2, Swift 6.4.2, arm64. Alle Versuche nutzen eine Ad-hoc-Signatur (`codesign -s -`). Kein Versuch nutzt die Developer ID oder den Notardienst von Apple.
- **Angenommen:** Die Aussage ist eine Ableitung. Sie ist nicht belegt und nicht gemessen.

### quill und App-Bundles

- **Quelle:** Der Zweig `main` von quill signiert und notarisiert App-Bundles seit dem 2026-09-21 ([PR 815](https://github.com/anchore/quill/pull/815), [PR 878](https://github.com/anchore/quill/pull/878)). quill signiert dabei eingebettete Binaries, schreibt `Contents/_CodeSignature/CodeResources` und bindet `Info.plist` und `CodeResources` in die Signatur der Hauptdatei ([README von quill](https://github.com/anchore/quill/blob/main/README.md)).
- **Quelle:** Das letzte Release von quill ist v0.7.1 vom 2026-03-10 ([Releases von quill](https://github.com/anchore/quill/releases)). Dieses Release enthält die Signatur von App-Bundles nicht. Das Projekt pinnt quill 0.7.1.
- **Quelle:** quill heftet kein Ticket an. Der PR 815 nennt das als Einschränkung: „Stapling is not implemented“.
- **Quelle:** quill unterstützt keine verschachtelten Bundles. Ein offener PR ergänzt das ([PR 883](https://github.com/anchore/quill/pull/883)). Die Swift-App enthält kein verschachteltes Bundle, daher trifft diese Einschränkung nicht zu.
- **Quelle:** quill nimmt den Notarschlüssel als Dateipfad oder als base64-kodierten Inhalt an ([README von quill](https://github.com/anchore/quill/blob/main/README.md)).
- **Angenommen:** Der Quelltext von quill enthält keinen Verweis auf den Abschnitt `__info_plist`. quill bindet eine eingebettete `Info.plist` eines einzelnen Mach-O daher vermutlich nicht in die Signatur. Der Abschnitt liegt im Segment `__TEXT`. Die Seiten-Hashes der Signatur decken ihn trotzdem ab.
- **Nicht geprüft:** Ein Versuch mit quill aus dem Zweig `main` fand nicht statt. Die Arbeitsumgebung des Spikes erlaubte den Bau von fremdem Quelltext nicht.

### goreleaser und App-Bundles

- **Quelle:** `notarize.macos` verarbeitet nur Binaries und Universal-Binaries. Die Dokumentation warnt: „App Bundles in which only the binary is signed/notarized are deemed damaged by macOS“ ([goreleaser: Notarize](https://goreleaser.com/customization/notarize/)).
- **Quelle:** Der Quelltext von goreleaser filtert auf die Artefakttypen `Binary` und `UniversalBinary` ([macos.go](https://github.com/goreleaser/goreleaser/blob/main/internal/pipe/notary/macos.go)).
- **Quelle:** goreleaser nutzt einen eigenen Fork von quill mit dem Stand vom 2026-06-30 ([go.mod von goreleaser](https://github.com/goreleaser/goreleaser/blob/main/go.mod)). Dieser Stand liegt vor der Signatur von App-Bundles in quill.
- **Quelle:** Diese Funktionen gibt es nur in GoReleaser Pro:
    - `app_bundles` ([goreleaser: App Bundles](https://goreleaser.com/customization/app_bundles/)),
    - `dmg` ([goreleaser: DMG](https://goreleaser.com/customization/dmg/)),
    - `notarize.macos_native` mit `codesign` und `xcrun` ([goreleaser: Notarize](https://goreleaser.com/customization/notarize/)),
    - der Builder `prebuilt` für fertig gebaute Binaries ([goreleaser: Prebuilt](https://goreleaser.com/customization/builds/builders/prebuilt/)).
- **Quelle:** Diese Funktionen gibt es in der freien Version:
    - `notarize.macos` mit quill,
    - `universal_binaries` ([goreleaser: Universal Binaries](https://goreleaser.com/customization/builds/universalbinaries/)),
    - `release.extra_files`, `release.draft` und `release.use_existing_draft` ([goreleaser: Release](https://goreleaser.com/customization/release/)).
- **Angenommen:** `archives.files` mit einem Filter über `ids` und `checksum.extra_files` gehören zur freien Version. Der Spike hat das nicht an einem Lauf geprüft.
- **Folge:** goreleaser in der freien Version kann die Swift-App weder bauen noch als App-Bundle signieren. goreleaser kann ein fertig signiertes App-Bundle aber als Datei in ein Archiv aufnehmen.

### rcodesign auf Linux

- **Quelle:** `rcodesign` aus dem Paket apple-codesign signiert, notarisiert und heftet Tickets an App-Bundles. Linux, Windows und macOS sind unterstützt ([README von apple-codesign](https://github.com/indygreg/apple-platform-rs/blob/main/apple-codesign/README.md)).
- **Quelle:** `rcodesign notary-submit --api-key-file <json> --staple <pfad>` notarisiert mit einem App-Store-Connect-API-Key und heftet das Ticket an. `rcodesign encode-app-store-connect-api-key` erzeugt die JSON-Datei aus Issuer-ID, Key-ID und der `.p8`-Datei ([rcodesign: Notarizing](https://gregoryszorc.com/docs/apple-codesign/stable/apple_codesign_rcodesign_notarizing.html)).
- **Quelle:** Die Lizenz ist MPL-2.0 ([Cargo.toml](https://github.com/indygreg/apple-platform-rs/blob/main/apple-codesign/Cargo.toml)).
- **Quelle:** Das letzte Release ist 0.29.0 vom 2024-11-29. Das Repository erhält weiter Commits für Abhängigkeiten. 122 Issues sind offen ([Releases von apple-platform-rs](https://github.com/indygreg/apple-platform-rs/releases)).
- **Folge:** `rcodesign` kann ein App-Bundle mit genau einem Mach-O und einer `Info.plist` von Linux aus gültig signieren. Das Werkzeug ist seit zwei Jahren ohne Release. Das Projekt müsste ein zweites Signaturwerkzeug neben quill pflegen.

### Bau der Swift-App ohne Xcode-Projekt

- **Gemessen:** `swiftc` baut je Architektur ein Binary, und `lipo -create` fügt beide zu einem Universal-Binary zusammen. `lipo -info` meldet `x86_64 arm64`.
- **Gemessen:** `swift build -c release --arch arm64 --arch x86_64` baut ein SwiftPM-Paket (Swift Package Manager) als Universal-Binary in 8,8 Sekunden. Das Ergebnis liegt unter `.build/out/Products/Release/`. Dieser Pfad hängt von der Swift-Version ab. Der Task fragt den Pfad daher mit `swift build --show-bin-path` ab.
- **Gemessen:** Der Compiler warnt bei x86_64: „The x86_64 architecture is deprecated for your deployment target“.
- **Gemessen:** Ein von Hand angelegtes App-Bundle genügt. Das App-Bundle enthält `Contents/Info.plist` und `Contents/MacOS/<Programm>`. `codesign` meldet dafür `Format=app bundle with Mach-O thin (arm64)`, `Info.plist entries=8` und `Sealed Resources version=2 rules=13 files=0`.
- **Folge:** Das Projekt braucht kein Xcode-Projekt und kein `xcodebuild`. Ein SwiftPM-Paket und ein Skript für das App-Bundle genügen.
- **Angenommen:** Der Bau braucht das macOS-SDK mit AppKit. Ein Linux-Runner hat dieses SDK nicht. Der Bau der Swift-App braucht daher immer einen macOS-Runner.

### Signatur auf dem macOS-Runner

- **Quelle:** Apple nennt für Developer ID diese Optionen von `codesign`: `--timestamp` für den Zeitstempel und `-o runtime` für die Hardened Runtime. Apple rät von `--deep` beim Signieren ab ([Apple: Creating distribution-signed code for the Mac](https://developer.apple.com/documentation/xcode/creating-distribution-signed-code-for-the-mac)).
- **Quelle:** Der Notardienst verlangt ein Developer-ID-Zertifikat, die Hardened Runtime und einen sicheren Zeitstempel. Die Berechtigung `com.apple.security.get-task-allow` darf nicht gesetzt sein ([Apple: Notarizing macOS software before distribution](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution)).
- **Quelle:** GitHub beschreibt den Import eines P12-Zertifikats in einen temporären Schlüsselbund auf dem macOS-Runner ([GitHub: Sign Xcode applications](https://docs.github.com/en/actions/how-tos/deploy/deploy-to-third-party-platforms/sign-xcode-applications)). Die Schritte sind:
    1. `security create-keychain -p <passwort> <pfad>` legt den Schlüsselbund an.
    2. `security set-keychain-settings -lut 21600 <pfad>` setzt die Sperrzeit.
    3. `security unlock-keychain -p <passwort> <pfad>` entsperrt den Schlüsselbund.
    4. `security import <p12> -P <p12-passwort> -A -t cert -f pkcs12 -k <pfad>` importiert das Zertifikat.
    5. `security set-key-partition-list -S apple-tool:,apple: -k <passwort> <pfad>` erlaubt `codesign` den Zugriff ohne Dialog.
    6. `security list-keychain -d user -s <pfad>` trägt den Schlüsselbund in die Suchliste ein.
    7. `security delete-keychain <pfad>` löscht den Schlüsselbund am Ende des Jobs.
- **Folge:** Das Secret `MACOS_SIGN_P12` ist base64-kodiert. Der Task dekodiert es in eine temporäre Datei. Das Secret `MACOS_SIGN_PASSWORD` ist das Passwort für Schritt 4. Beide Secrets sind ohne Änderung wiederverwendbar.

### Berechtigungen und Info.plist

- **Quelle:** Touch ID braucht keinen Hinweistext: „The system doesn’t require a comparable usage description for Touch ID“ ([Apple: Logging a user into your app with Face ID or Touch ID](https://developer.apple.com/documentation/localauthentication/logging-a-user-into-your-app-with-face-id-or-touch-id)).
- **Quelle:** Der Schlüssel `NSFaceIDUsageDescription` gilt laut den Metadaten der Dokumentation nur für iOS und iPadOS ([Apple: NSFaceIDUsageDescription](https://developer.apple.com/documentation/bundleresources/information-property-list/nsfaceidusagedescription)).
- **Quelle:** Die Hardened Runtime verbietet nur wenige Fähigkeiten, zum Beispiel JIT-Kompilierung. Ein Programm braucht eine Berechtigung nur für eine solche Ausnahme ([Apple: Hardened Runtime](https://developer.apple.com/documentation/security/hardened-runtime)).
- **Gemessen:** `LAContext.canEvaluatePolicy` liefert `true` ohne jede Berechtigung. Das gilt für das einzelne Mach-O und für das App-Bundle, beide mit Hardened Runtime.
- **Gemessen:** `LAContext.evaluatePolicy` öffnet den Touch-ID-Dialog ohne jede Berechtigung. Das einzelne Mach-O erhält nach dem programmierten Abbruch den Fehler `-9` (`appCancel`). Dieser Fehler belegt, dass der Dialog offen war. Beim App-Bundle meldet der Aufruf nach 1,39 Sekunden Erfolg, weil eine Person am Rechner den Sensor berührt hat.
- **Folge:** Die Swift-App braucht keine Datei mit Berechtigungen und keinen Hinweistext für Touch ID.
- **Quelle:** Eine eingeschränkte Berechtigung wie `keychain-access-groups` braucht ein Provisioning-Profil. Ein Programm ohne Bundle kann kein Provisioning-Profil enthalten ([Apple: Signing a daemon with a restricted entitlement](https://developer.apple.com/documentation/xcode/signing-a-daemon-with-a-restricted-entitlement)).
- **Angenommen:** Legt die Swift-App später einen Schlüssel mit Biometrie-Schutz im Schlüsselbund ab, braucht sie eine eingeschränkte Berechtigung und damit ein App-Bundle.

### Notarisierung und Ticket

- **Quelle:** `notarytool submit` nimmt den App-Store-Connect-API-Key mit `--key <pfad>`, `--key-id <id>` und `--issuer <uuid>` an. `--key` ist ein Dateipfad. `--wait` wartet auf das Urteil. `--timeout` begrenzt die Wartezeit (`man notarytool`, `xcrun notarytool submit --help`, lokal gelesen).
- **Folge:** Die Secrets `MACOS_NOTARY_KEY_ID` und `MACOS_NOTARY_ISSUER_ID` sind ohne Änderung wiederverwendbar. Das Secret `MACOS_NOTARY_KEY` enthält die `.p8`-Datei base64-kodiert. Der Task dekodiert das Secret in eine temporäre Datei und übergibt den Pfad an `--key`.
- **Quelle:** Der Notardienst nimmt ZIP-Archive, Disk-Images und signierte Installer-Pakete an. Apple erzeugt das ZIP-Archiv mit `ditto -c -k --keepParent` ([Apple: Customizing the notarization workflow](https://developer.apple.com/documentation/security/customizing-the-notarization-workflow)).
- **Quelle:** `stapler` heftet das Ticket an App-Bundles, Disk-Images und Installer-Pakete. An ein ZIP-Archiv und an ein einzelnes Binary heftet `stapler` kein Ticket: „Although tickets are created for standalone binaries, it’s not currently possible to staple tickets to them“ (gleiche Seite, außerdem `man stapler`).
- **Quelle:** Ohne angeheftetes Ticket sucht Gatekeeper das Ticket online. Ohne Netzverbindung blockiert Gatekeeper das Programm möglicherweise ([Apple: Packaging Mac software for distribution](https://developer.apple.com/documentation/xcode/packaging-mac-software-for-distribution)).
- **Quelle:** quill schließt `Contents/CodeResources` aus dem Siegel aus. In dieser Datei liegt das angeheftete Ticket ([PR 815](https://github.com/anchore/quill/pull/815)). Das Ticket ist damit eine normale Datei im App-Bundle.

### Verpackung

- **Gemessen:** Ein App-Bundle übersteht `tar czf` und `tar xzf` mit gültiger Signatur. `codesign --verify --deep --strict` meldet danach `valid on disk`.
- **Gemessen:** Ein App-Bundle übersteht `ditto -c -k --keepParent` und `ditto -x -k` mit gültiger Signatur.
- **Angenommen:** Das angeheftete Ticket übersteht `tar`, weil das Ticket eine normale Datei ist. Der Spike konnte das ohne Notarisierung nicht messen.
- **Angenommen:** `actions/upload-artifact` erhält das Ausführungsrecht von Dateien nicht. Der macOS-Job übergibt das App-Bundle daher als `tar.gz`-Datei.
- **Folge:** Das Projekt braucht `ditto` nur für die Einreichung beim Notardienst. Das Release-Archiv bleibt `tar.gz`. Die Swift-App nutzt keine erweiterten Attribute.

### Einzelnes Mach-O mit eingebetteter Info.plist

Der Versuch baut ein Swift-Programm mit dieser Linker-Option:

```sh
swiftc main.swift -o approve \
  -Xlinker -sectcreate -Xlinker __TEXT -Xlinker __info_plist -Xlinker Info.plist
```

Die `Info.plist` enthält `CFBundleIdentifier`, `CFBundleName`, zwei Versionsschlüssel, `LSUIElement` und `LSMinimumSystemVersion`.

Das funktioniert:

- **Gemessen:** `otool -l` zeigt den Abschnitt `__info_plist` im Segment `__TEXT`.
- **Gemessen:** `Bundle.main.infoDictionary` liest die eingebettete `Info.plist`. `Bundle.main.bundleIdentifier` liefert die Bundle-ID. Das gilt für beide Architekturen und für den Start über einen symbolischen Link aus einem anderen Verzeichnis.
- **Gemessen:** Ein Programm ohne eingebettete `Info.plist` liefert `nil`. Liegt eine Datei `Info.plist` im Verzeichnis des Programms, liest `Bundle.main` diese Datei. Der Versuch lief deshalb in einem leeren Verzeichnis.
- **Gemessen:** `codesign` bindet die eingebettete `Info.plist` in die Signatur (`Info.plist entries=6`) und übernimmt die Bundle-ID als `Identifier`. Die Hardened Runtime ist gesetzt (`flags=0x10002(adhoc,runtime)`). `codesign --verify --strict` meldet `valid on disk`.
- **Gemessen:** Ein `NSPanel` ist sichtbar (`isVisible=true`).
- **Gemessen:** Der Touch-ID-Dialog öffnet sich.

Das funktioniert nicht oder fehlt:

- **Gemessen:** LaunchServices kennt keine Bundle-ID. `lsappinfo` zeigt `bundleID=[ NULL ]`. `NSRunningApplication.current.bundleIdentifier` liefert `nil`.
- **Gemessen:** `LSUIElement` wirkt nicht. Die Aktivierungsrichtlinie ist `prohibited` (Wert 2), mit und ohne eingebettete `Info.plist`. Das Programm muss `setActivationPolicy(.accessory)` selbst aufrufen. Der Aufruf gelingt.
- **Gemessen:** Das App-Bundle erhält dagegen die Bundle-ID bei LaunchServices und die Aktivierungsrichtlinie `accessory` (Wert 1). Das gilt auch, wenn ein Prozess die Datei `Contents/MacOS/<Programm>` direkt startet.
- **Quelle:** `stapler` heftet kein Ticket an ein einzelnes Binary (siehe [Notarisierung und Ticket](#notarisierung-und-ticket)).
- **Quelle:** Apple empfiehlt die eingebettete `Info.plist`, um einem Kommandozeilenprogramm eine Bundle-ID für TCC (Transparency, Consent and Control, die Rechteverwaltung von macOS) zu geben ([Apple Developer Forums 698337](https://developer.apple.com/forums/thread/698337)).
- **Angenommen:** Werkzeuge, die Fenster über die Bundle-ID zuordnen, erkennen das einzelne Mach-O nicht. Dazu gehören Fensterverwalter.
- **Nicht geprüft:** Der Spike hat nicht geprüft, welchen Namen der Touch-ID-Dialog für das einzelne Mach-O zeigt.
- **Nicht geprüft:** Der Spike hat nicht geprüft, ob das `NSPanel` des einzelnen Mach-O Tastatureingaben erhält.

Der Weg zur Signatur mit quill ist bei dieser Option nicht frei:

- goreleaser signiert nur Binaries aus eigenen Builds. Der Builder `prebuilt` gehört zu GoReleaser Pro. Ein Task muss `quill sign-and-notarize` daher direkt aufrufen.
- Der Bau braucht trotzdem einen macOS-Runner.

### SBOM für die Swift-App

- **Gemessen:** Swift 6.4.2 erzeugt eine CycloneDX-SBOM mit `swift build --sbom-spec cyclonedx --sbom-output-dir <verzeichnis>`. Die Datei hat das Format CycloneDX 1.7. Die Datei enthält das Paket und das Produkt als Komponenten vom Typ `application`.
- **Gemessen:** Die Version der Komponenten ist `unknown`. Der Dateiname enthält einen Zeitstempel. Swift meldet: „Bundle 'SwiftPM_SBOMModel' with schemas not found - skipping SBOM validation“.
- **Nicht geprüft:** Der Spike hat nicht geprüft, ab welcher Swift-Version die Option vorhanden ist. Der GitHub-Runner `macos-26` liefert Xcode 26.6 als Standard.
- **Quelle:** mise kennt `syft` und `cyclonedx` (cyclonedx-cli) in der Registry (`mise registry`, lokal gelesen).
- **Angenommen:** `syft` liest für Swift die Datei `Package.resolved`. Ein Paket ohne fremde Abhängigkeiten hat diese Datei nicht. `syft` liefert dann keine Komponenten.
- **Folge:** Die Swift-App hat keine fremden Abhängigkeiten. Die SBOM enthält daher genau eine Komponente. Das vorhandene Werkzeug `tools/sbom` kann diese Komponente selbst erzeugen: Name, Version aus dem Tag, SHA-256 des Mach-O.

### Runner, Kosten und Versionen

- **Quelle:** Ein macOS-Runner mit 3 oder 4 Kernen kostet 0,062 USD je Minute. Ein Linux-Runner mit 2 Kernen kostet 0,006 USD je Minute. GitHub rundet jede Job-Laufzeit auf volle Minuten auf ([GitHub: Actions runner pricing](https://docs.github.com/en/billing/reference/actions-runner-pricing)).
- **Quelle:** Für private Repositorys enthält GitHub Free 12 USD und GitHub Pro 18 USD Rechenleistung je Monat ([GitHub: Billing for GitHub Actions](https://docs.github.com/en/billing/concepts/product-billing/github-actions)). Das entspricht rechnerisch 193 oder 290 macOS-Minuten.
- **Angenommen:** Der macOS-Job dauert 4 bis 8 Minuten. Der größte Anteil ist die Wartezeit auf den Notardienst. Ein Release kostet damit 0,25 bis 0,50 USD.
- **Quelle:** Das Runner-Label `macos-26` ist ein arm64-Runner. Xcode 26.6 ist der Standard. Weitere Versionen liegen unter `/Applications/Xcode_<version>.app` ([runner-images: macOS 26 arm64](https://github.com/actions/runner-images/blob/main/images/macos/macos-26-arm64-Readme.md)).
- **Quelle:** mise installiert mit dem Werkzeug `swift` die Toolchains von swift.org für macOS und Linux ([mise: Swift](https://mise.jdx.dev/lang/swift.html)).
- **Angenommen:** Eine Toolchain von swift.org braucht für AppKit weiterhin das macOS-SDK aus Xcode. Die Auswahl von Xcode über die Umgebungsvariable `DEVELOPER_DIR` im mise-Task ist deshalb der einfachere Pin. `DEVELOPER_DIR` braucht kein `sudo xcode-select`.

### Attestierung

- **Quelle:** `actions/attest-build-provenance` ist seit Version 4 eine Hülle um `actions/attest`. Neue Workflows nutzen `actions/attest` ([README von attest-build-provenance](https://github.com/actions/attest-build-provenance)).
- **Quelle:** Artefakt-Attestierungen in privaten Repositorys setzen GitHub Enterprise Cloud voraus. In den Plänen Free, Pro und Team gibt es Attestierungen nur für öffentliche Repositorys (gleiche Seite).
- **Quelle:** Der Job braucht die Rechte `id-token: write`, `contents: read` und `attestations: write` ([GitHub: Use artifact attestations](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations)).
- **Quelle:** mise prüft Attestierungen nur auf dem öffentlichen GitHub ([mise: GitHub-Backend](https://mise.jdx.dev/dev-tools/backends/github.html)).
- **Angenommen:** `actions/attest` nimmt mit `subject-checksums` eine Prüfsummendatei an und attestiert jede Datei darin.

### Installation über mise und Fundort der Swift-App

- **Quelle:** Das `github:`-Backend von mise wählt je Plattform genau ein Asset. Die Bewertung bevorzugt das passende Betriebssystem und die passende Architektur. Die Bewertung wertet Assets ab, die ein macOS-App-Bundle sind ([mise: GitHub-Backend](https://mise.jdx.dev/dev-tools/backends/github.html)).
- **Quelle:** mise sucht das Verzeichnis der Programme in dieser Reihenfolge: `bin/`, `Contents/MacOS` eines entpackten App-Bundles, das Installationsverzeichnis selbst, direkte Unterverzeichnisse (gleiche Seite).
- **Folge:** Die Swift-App liegt im selben Archiv wie `a38`. Ein eigenes Asset für die Swift-App erreicht den Nutzer über mise nicht.
- **Angenommen:** Liegt `a38` im Wurzelverzeichnis des Archivs und das App-Bundle daneben, legt mise das Wurzelverzeichnis auf den `PATH`. Der Go-Daemon findet die Swift-App dann relativ zum eigenen Programm: `os.Executable()`, danach `filepath.EvalSymlinks`, danach `<verzeichnis>/<Name>.app/Contents/MacOS/<Name>`.
- **Gemessen:** Ein Prozess kann `Contents/MacOS/<Programm>` direkt starten und dabei stdin und stdout nutzen. Das App-Bundle behält dabei seine Bundle-ID.

### Gatekeeper und Quarantäne

- **Gemessen:** Eine mit `curl` geladene Datei trägt kein Attribut `com.apple.quarantine`.
- **Gemessen:** Von mise installierte Programme (`gh`, `flux-operator`) tragen kein erweitertes Attribut.
- **Quelle:** Das herunterladende Programm setzt das Attribut. `curl` und `scp` setzen das Attribut nicht. Gatekeeper prüft in der Regel nur Dateien mit diesem Attribut ([Apple Developer Forums 706273](https://developer.apple.com/forums/thread/706273), [Apple Developer Forums 666452](https://developer.apple.com/forums/thread/666452)). Der Spike hat diese beiden Seiten nur über die Websuche gelesen, nicht im Original.
- **Quelle:** Gatekeeper prüft Software aus dem Internet beim ersten Öffnen und prüft dabei die Notarisierung ([Apple Platform Security: Gatekeeper and runtime protection](https://support.apple.com/guide/security/gatekeeper-and-runtime-protection-sec5599b66df/web)).
- **Folge:** Bei einer Installation über mise oder `curl` findet die Prüfung beim ersten Öffnen nicht statt. Die Notarisierung wirkt in diesen Fällen:
    - Ein Nutzer lädt das Archiv mit einem Browser.
    - Eine Richtlinie des Unternehmens verlangt notarisierte Software.
    - Apple widerruft ein Ticket nach einem Missbrauch des Schlüssels.
- **Folge:** Das angeheftete Ticket nützt nur beim Download mit einem Browser ohne Netzverbindung beim ersten Start. Das Anheften kostet einen Befehl.

## Optionen

| Option | Vorteile | Nachteile | Aufwand |
|---|---|---|---|
| A: quill signiert das App-Bundle | Ein Signaturwerkzeug für Go und Swift. Kein Schlüsselbund nötig. Dieselben Secrets. | Nur im Zweig `main` von quill, nicht in einem Release. goreleaser bindet die Funktion nicht ein. Kein angeheftetes Ticket. Der Bau braucht trotzdem einen macOS-Runner. | Mittel. Pin auf einen Commit statt auf ein Release. Eigener Task für `quill sign-and-notarize`. |
| B: `rcodesign` signiert das App-Bundle auf Linux | Signatur, Notarisierung und Anheften von Linux aus. API-Key wird unterstützt. | Zweites Signaturwerkzeug. Letztes Release vom 2024-11-29. Eigenes JSON-Format für den API-Key. Der Bau braucht trotzdem einen macOS-Runner. | Mittel bis hoch. Neues Werkzeug, neue Prüfung, Übergabe des unsignierten App-Bundles an Linux. |
| C: macOS-Runner mit `codesign`, `notarytool`, `stapler` | Werkzeuge von Apple. Vollwertiges App-Bundle mit Bundle-ID und angeheftetem Ticket. Prüfung mit `codesign`, `spctl` und `stapler validate` im selben Job. Dieselben Secrets. | Import des Zertifikats in einen temporären Schlüsselbund. Die Wartezeit auf den Notardienst kostet macOS-Minuten. Zwei Signaturwege im Projekt. | Mittel. Vier mise-Tasks, ein zusätzlicher Job, eine Übergabe als Artefakt. |
| D: einzelnes Mach-O mit eingebetteter `Info.plist`, signiert von quill 0.7.1 | Kein Bundle. Die Swift-App liegt als eine Datei neben `a38`. quill 0.7.1 genügt. Kein Schlüsselbund nötig. | Keine Bundle-ID bei LaunchServices. `LSUIElement` wirkt nicht. Kein angeheftetes Ticket. Kein Weg zu eingeschränkten Berechtigungen. quill bindet die `Info.plist` vermutlich nicht. Der Bau braucht trotzdem einen macOS-Runner. goreleaser signiert das fremde Binary nicht. | Niedrig bis mittel. Linker-Option, `setActivationPolicy` im Code, eigener Task für `quill sign-and-notarize`. |

## Empfehlung

Das Projekt wählt Option C.

Gründe:

- Jede Option braucht einen macOS-Runner für den Bau. Option C ergänzt diesen Job nur um Signatur und Notarisierung.
- Option C liefert das einzige Ergebnis, das Apple vollständig dokumentiert und das der Job selbst prüfen kann.
- Option C hält den Weg zu eingeschränkten Berechtigungen offen.
- Option D ist die Rückfallebene. Der lokale Versuch belegt, dass Option D technisch trägt.
- Option A wird attraktiv, sobald quill die Signatur von App-Bundles in einem Release ausliefert.

### Aufbau der Swift-App

- Die Swift-App ist ein SwiftPM-Paket mit einem ausführbaren Ziel.
- Ein Skript legt das App-Bundle an: `Contents/Info.plist` und `Contents/MacOS/<Name>`.
- Die `Info.plist` enthält mindestens `CFBundleIdentifier`, `CFBundleExecutable`, `CFBundlePackageType` mit dem Wert `APPL`, `CFBundleShortVersionString`, `CFBundleVersion`, `LSUIElement` und `LSMinimumSystemVersion`.
- Das Binary ist ein Universal-Binary, solange das Projekt `a38` für darwin/amd64 ausliefert.
- Der Name `A38Approve.app` in den folgenden Skizzen ist ein Platzhalter.

### Skizze der mise-Tasks

```toml
[tasks."app:build"]
description = "Build the Swift app as universal binary and assemble A38Approve.app in build/app/"
env = { DEVELOPER_DIR = "/Applications/Xcode_26.6.app/Contents/Developer" }
run = "bash scripts/app-build.sh"

[tasks."app:sign"]
description = "CI only: import the Developer ID certificate into a temporary keychain and sign A38Approve.app (hardened runtime, timestamp)"
run = "bash scripts/app-sign.sh"

[tasks."app:notarize"]
description = "CI only: submit A38Approve.app to the Apple notary service, wait, staple the ticket"
run = "bash scripts/app-notarize.sh"

[tasks."app:check"]
description = "Check signature, Gatekeeper verdict and stapled ticket of A38Approve.app"
run = "bash scripts/app-check.sh"

[tasks."app:package"]
description = "Pack A38Approve.app into build/app/a38-approve-darwin-universal.tar.gz for the release job"
run = "COPYFILE_DISABLE=1 tar -czf build/app/a38-approve-darwin-universal.tar.gz -C build/app A38Approve.app"
```

Kern von `scripts/app-sign.sh`:

```sh
codesign --sign "Developer ID Application" --force --timestamp --options runtime \
  --keychain "$KEYCHAIN" build/app/A38Approve.app
```

Kern von `scripts/app-notarize.sh`:

```sh
printf '%s' "$MACOS_NOTARY_KEY" | base64 --decode > "$KEY_FILE"
ditto -c -k --keepParent build/app/A38Approve.app "$ZIP"
xcrun notarytool submit "$ZIP" --key "$KEY_FILE" \
  --key-id "$MACOS_NOTARY_KEY_ID" --issuer "$MACOS_NOTARY_ISSUER_ID" \
  --wait --timeout 20m
xcrun stapler staple build/app/A38Approve.app
```

Kern von `scripts/app-check.sh`:

```sh
codesign --verify --deep --strict --verbose=2 build/app/A38Approve.app
spctl -a -vv -t exec build/app/A38Approve.app
xcrun stapler validate build/app/A38Approve.app
```

Die Skripte löschen den temporären Schlüsselbund und die Schlüsseldatei über `trap` am Ende. Die Skripte geben keine Secrets aus.

### Skizze der Workflow-Jobs

Der Workflow `release.yml` erhält drei Jobs:

1. Job `app` auf `macos-26`:
    1. `mise run app:build`
    2. `mise run app:sign`
    3. `mise run app:notarize`
    4. `mise run app:check`
    5. `mise run app:package`
    6. `actions/attest` attestiert die `tar.gz`-Datei.
    7. `actions/upload-artifact` lädt die `tar.gz`-Datei als Workflow-Artefakt hoch.
2. Job `release` auf `ubuntu-latest`, mit `needs: app`:
    1. `actions/download-artifact` lädt die `tar.gz`-Datei.
    2. `mise run release:publish` entpackt die Datei nach `build/app/` und ruft goreleaser auf.
    3. goreleaser nimmt `build/app/A38Approve.app` über `archives.files` in die darwin-Archive auf.
    4. goreleaser erzeugt SBOM und Prüfsummen und legt das Release als Entwurf an (`release.draft: true`).
    5. `actions/attest` attestiert alle Dateien der Prüfsummendatei.
3. Job `verify` auf `macos-26`, mit `needs: release`:
    1. `mise run release:verify` lädt die darwin-Archive des Entwurfs, entpackt sie mit `tar xzf` und ruft die Prüfungen aus `app:check` und die Prüfung von `a38` auf.
    2. `mise run release:finalize` veröffentlicht den Entwurf mit `gh release edit <tag> --draft=false`.

Folgen für goreleaser:

- goreleaser braucht zwei Einträge unter `archives`: einen für darwin mit dem App-Bundle, einen für linux ohne das App-Bundle. Die Builds erhalten dafür getrennte IDs.
- Die Prüfsummendatei deckt das App-Bundle ab, weil das App-Bundle im darwin-Archiv liegt.
- `tools/sbom` ergänzt die SBOM jedes darwin-Archivs um die Komponente der Swift-App mit dem SHA-256 des Mach-O. Die Regel „eine SBOM je Archiv“ bleibt damit erhalten.
- Der Workflow `signed-snapshot.yml` erhält dieselben Jobs `app` und `release` ohne Veröffentlichung.

Eine einfachere Variante führt alle Schritte in einem einzigen Job auf `macos-26` aus. goreleaser und quill laufen auch auf macOS. Diese Variante braucht keine Übergabe als Artefakt und keinen Job `verify`. Die Variante kostet mehr macOS-Minuten, weil auch die Wartezeit für die Go-Binaries auf dem macOS-Runner anfällt.

### Einträge für den Design-Lauf

Entscheidungen:

- Die Swift-App ist ein App-Bundle. Das Projekt signiert die Swift-App auf einem macOS-Runner mit `codesign`, `notarytool` und `stapler`.
- Das App-Bundle liegt im darwin-Archiv neben `a38`. Der Go-Daemon sucht das App-Bundle relativ zum eigenen Programm. Eine Umgebungsvariable überschreibt den Pfad.
- Der Go-Daemon startet `Contents/MacOS/<Name>` direkt und nicht über `open`.
- Die Swift-App braucht keine Berechtigungen und keinen Hinweistext für Touch ID.
- Die fünf Secrets `MACOS_SIGN_*` und `MACOS_NOTARY_*` gelten für beide Signaturwege.
- Drei Jobs oder ein Job auf macOS: Der Design-Lauf entscheidet zwischen beiden Varianten.
- Universal-Binary oder nur arm64: Der Design-Lauf entscheidet, wie lange das Projekt darwin/amd64 ausliefert.

Lücken:

- quill liefert die Signatur von App-Bundles noch nicht in einem Release aus. Das Projekt prüft Option A erneut nach dem nächsten Release von quill.
- Der Pin der Swift-Version ist offen: `DEVELOPER_DIR` auf eine Xcode-Version oder das mise-Werkzeug `swift`. Der Task `tools:check` muss den Pin prüfen.
- Die SBOM-Erzeugung für die Swift-App ist offen: `tools/sbom` oder `swift build --sbom-spec`.
- Der Linux-Job kann die Signatur des App-Bundles nicht mit den Werkzeugen von Apple prüfen. Die Prüfung nach dem Packen braucht macOS.

Risiken:

- Attestierungen in einem privaten Repository setzen GitHub Enterprise Cloud voraus. Ohne diesen Plan schlägt der Schritt `actions/attest` fehl.
- Der Notardienst antwortet gelegentlich langsam. Jede Minute Wartezeit kostet auf dem macOS-Runner das Zehnfache einer Linux-Minute.
- Das Runner-Image ändert die Standardversion von Xcode ohne Ankündigung im Repository. Ein fester Pfad in `DEVELOPER_DIR` bricht, sobald GitHub diese Xcode-Version entfernt.
- Ein Schritt zwischen Signatur und Archiv darf keine Datei im App-Bundle ändern oder ergänzen. Jede Änderung macht die Signatur ungültig.
- Zwei Signaturwege (quill für Go, `codesign` für Swift) verdoppeln die Prüfskripte.

## Offene Punkte

Diese Punkte brauchen die Developer ID von Stefan oder einen echten Release-Lauf:

- Der Import des P12-Zertifikats in den temporären Schlüsselbund auf `macos-26`. Offen ist, ob das Zwischenzertifikat von Apple auf dem Runner vorhanden ist oder im P12 liegen muss.
- Die Signatur mit `codesign --timestamp --options runtime` und die Ausgabe von `codesign -dvvv` mit Team-ID.
- Die Notarisierung mit dem vorhandenen App-Store-Connect-API-Key über `notarytool` und die tatsächliche Wartezeit.
- Das Anheften des Tickets und die Prüfung mit `stapler validate` nach `tar czf`, Übergabe als Artefakt, Aufnahme in das goreleaser-Archiv und `tar xzf`.
- Das Urteil von `spctl -a -vv -t exec` für das entpackte App-Bundle.
- Die Installation über das `github:`-Backend von mise aus dem privaten Repository: Wahl des Assets, erkanntes Programmverzeichnis, Fundort des App-Bundles, Token.
- Das Verhalten von `archives.files` mit einem Verzeichnis: Dateirechte und Pfade im Archiv.
- Die Verfügbarkeit von `swift build --sbom-spec` in der Swift-Version des Runners.
- Die Verfügbarkeit von Attestierungen im privaten Repository mit dem aktuellen GitHub-Plan.
- Der Name, den der Touch-ID-Dialog für das App-Bundle zeigt, und die Tastatureingabe im `NSPanel`.
- Ein Versuch mit quill aus dem Zweig `main` an dem App-Bundle, als Vorbereitung für Option A.

## Quellen

quill:

- [README von quill](https://github.com/anchore/quill/blob/main/README.md)
- [Releases von quill](https://github.com/anchore/quill/releases)
- [PR 815: support signing and notarizing app bundles](https://github.com/anchore/quill/pull/815)
- [PR 878: App bundle signing adjustments](https://github.com/anchore/quill/pull/878)
- [PR 883: seal nested bundles](https://github.com/anchore/quill/pull/883)
- [Issue 550: signing app bundles](https://github.com/anchore/quill/issues/550)

goreleaser:

- [Notarize](https://goreleaser.com/customization/notarize/)
- [App Bundles](https://goreleaser.com/customization/app_bundles/)
- [DMG](https://goreleaser.com/customization/dmg/)
- [Universal Binaries](https://goreleaser.com/customization/builds/universalbinaries/)
- [Prebuilt](https://goreleaser.com/customization/builds/builders/prebuilt/)
- [Release](https://goreleaser.com/customization/release/)
- [Archives](https://goreleaser.com/customization/package/archives/)
- [Checksums](https://goreleaser.com/customization/package/checksum/)
- [Quelltext macos.go](https://github.com/goreleaser/goreleaser/blob/main/internal/pipe/notary/macos.go)
- [go.mod](https://github.com/goreleaser/goreleaser/blob/main/go.mod)

apple-codesign:

- [README von apple-codesign](https://github.com/indygreg/apple-platform-rs/blob/main/apple-codesign/README.md)
- [rcodesign: Notarizing](https://gregoryszorc.com/docs/apple-codesign/stable/apple_codesign_rcodesign_notarizing.html)
- [Releases von apple-platform-rs](https://github.com/indygreg/apple-platform-rs/releases)

Apple:

- [Creating distribution-signed code for the Mac](https://developer.apple.com/documentation/xcode/creating-distribution-signed-code-for-the-mac)
- [Packaging Mac software for distribution](https://developer.apple.com/documentation/xcode/packaging-mac-software-for-distribution)
- [Notarizing macOS software before distribution](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution)
- [Customizing the notarization workflow](https://developer.apple.com/documentation/security/customizing-the-notarization-workflow)
- [Hardened Runtime](https://developer.apple.com/documentation/security/hardened-runtime)
- [Signing a daemon with a restricted entitlement](https://developer.apple.com/documentation/xcode/signing-a-daemon-with-a-restricted-entitlement)
- [Logging a user into your app with Face ID or Touch ID](https://developer.apple.com/documentation/localauthentication/logging-a-user-into-your-app-with-face-id-or-touch-id)
- [NSFaceIDUsageDescription](https://developer.apple.com/documentation/bundleresources/information-property-list/nsfaceidusagedescription)
- [LSUIElement](https://developer.apple.com/documentation/bundleresources/information-property-list/lsuielement)
- [Apple Platform Security: Gatekeeper and runtime protection](https://support.apple.com/guide/security/gatekeeper-and-runtime-protection-sec5599b66df/web)
- [Apple Developer Forums 698337: TCC und Bundle-ID](https://developer.apple.com/forums/thread/698337)
- [Apple Developer Forums 706273: Download mit curl](https://developer.apple.com/forums/thread/706273)
- [Apple Developer Forums 666452: Gatekeeper bei Entwicklungsständen](https://developer.apple.com/forums/thread/666452)
- Handbuchseiten `man notarytool`, `man stapler` und `xcrun notarytool submit --help` aus Xcode 27.2

GitHub:

- [Actions runner pricing](https://docs.github.com/en/billing/reference/actions-runner-pricing)
- [Billing for GitHub Actions](https://docs.github.com/en/billing/concepts/product-billing/github-actions)
- [Sign Xcode applications](https://docs.github.com/en/actions/how-tos/deploy/deploy-to-third-party-platforms/sign-xcode-applications)
- [Use artifact attestations](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations)
- [README von attest-build-provenance](https://github.com/actions/attest-build-provenance)
- [runner-images: macOS 26 arm64](https://github.com/actions/runner-images/blob/main/images/macos/macos-26-arm64-Readme.md)

mise:

- [GitHub-Backend](https://mise.jdx.dev/dev-tools/backends/github.html)
- [GitHub-Tokens](https://mise.jdx.dev/dev-tools/github-tokens.html)
- [Swift](https://mise.jdx.dev/lang/swift.html)

Referenz im eigenen Bestand:

- `~/develop/total-recall/.goreleaser.yml`
- `~/develop/total-recall/.github/workflows/signed-snapshot.yml`
- `~/develop/total-recall/docs/reference/50-release.md`
- `~/develop/vault-mcp/mise.toml`
- `~/develop/vault-mcp/scripts/signatures-check.sh`
