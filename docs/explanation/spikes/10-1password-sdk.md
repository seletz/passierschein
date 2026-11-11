---
title: "Spike: 1Password-Go-SDK"
description: "Der Spike klärt, über welchen Zugang `a38 serve` auf 1Password zugreift: über die Desktop-App-Integration des Go-SDK oder über ein Service-Konto."
tags:
  - Spike
  - 1Password
---
# Spike: 1Password-Go-SDK

Stand der Recherche: 2026-11-11.

## Frage

`a38 serve` ist die einzige Komponente mit Zugriff auf 1Password.
`a38 serve` nutzt dafür das 1Password-Go-SDK (SDK: Software Development Kit).
Das Go-SDK bietet zwei Zugänge:

- die Desktop-App-Integration: Die 1Password-App auf dem Mac gibt den Zugriff nach einer Bestätigung frei.
- das Service-Konto: Ein Token gibt den Zugriff auf festgelegte Tresore frei.

Der Spike beantwortet diese Fragen:

- Welchen Zugang nutzt `a38 serve`?
- Stimmen die Annahmen „Beta“, „v0.4.0-beta“ und „benötigt CGO“ noch?
- Welche Folgen hat der Zugang für Build, Signatur und Test?

CGO ist der Mechanismus, mit dem Go-Code C-Code aufruft.

## Ergebnis

`a38 serve` kapselt 1Password hinter einer eigenen Go-Schnittstelle mit zwei möglichen Implementierungen.
Die erste Version nutzt ein Service-Konto mit dem Recht `read_items` auf den Dev-Tresoren.
Dieser Zugang benötigt kein CGO, und der statische Build mit goreleaser und quill bleibt unverändert.
1Password begrenzt das Service-Konto serverseitig auf die gewählten Tresore.
Die Desktop-App-Integration ist seit SDK v0.4.0 nicht mehr Beta, hat aber offene Fehler und gibt immer das ganze 1Password-Konto frei.
Die Desktop-App-Integration bleibt deshalb eine spätere, wählbare Implementierung hinter einem Build-Tag.
Alle Befunde beruhen auf Dokumentation und öffentlichem Quelltext.
Kein Befund ist praktisch getestet.
Die praktischen Tests stehen als Testplan im Abschnitt „Offene Punkte“.

## Befunde

Jede Aussage trägt eine Markierung:

- **[P]**: Die Aussage ist aus einer Primärquelle geprüft (Dokumentation, Repository oder Release Notes von 1Password).
- **[L]**: Die Aussage stammt aus dem gelesenen Quelltext des öffentlichen Moduls v0.4.1. Der Quelltext ist nicht ausgeführt.
- **[A]**: Die Aussage ist eine Annahme. Ein Test steht aus.

### 1. Stand des Go-SDK

- **[P]** Die neueste stabile Version ist v0.4.1 vom 2026-07-30. Der öffentliche Go-Modul-Index zeigt keine neuere Version. Quelle: [Releases][rel].
- **[P]** Die letzte Beta-Version ist v0.4.1-beta.1 vom 2026-02-11. Eine neuere Beta-Version gibt es nicht.
- **[P]** v0.4.0 vom 2026-02-10 enthält die Desktop-App-Integration als stabile Funktion. Die Beta-Versionen v0.4.0-beta.1 und v0.4.0-beta.2 stammen von Oktober und November 2025. Quelle: [Releases][rel].
- **[P]** Die Dokumentation zur Desktop-App-Integration trägt keine Beta-Kennzeichnung mehr. Quelle: [Sicherheitsmodell der Desktop-App-Integration][dai].
- **[P]** Seit v0.4.1 baut das SDK mit `CGO_ENABLED=0`. Nur die Desktop-App-Integration benötigt CGO. Quelle: [Releases][rel].
- **[L]** Ohne CGO liefert `WithDesktopAppIntegration` zur Laufzeit einen Fehler. Der Build schlägt nicht fehl. Quelle: `client_builder_no_cgo.go` in v0.4.1.
- **[P]** Das gesamte SDK steht in Version 0. 1Password erlaubt inkompatible Änderungen zwischen 0.x-Versionen und sagt drei Monate Unterstützung je Version zu. Quelle: [SDK-Übersicht][sdks].
- **[P]** Die Desktop-App-Integration unterstützt macOS, Linux und Windows. Quelle: [Releases][rel].
- **[P]** Die Dokumentation wohnt jetzt unter `www.1password.dev`. Die Adressen unter `developer.1password.com` leiten dorthin um.

Korrektur der Projektannahmen:

| Annahme | Stand am 2026-11-11 |
| --- | --- |
| Die Desktop-App-Integration ist Beta. | Falsch. Die Funktion ist seit v0.4.0 stabil. Das SDK selbst bleibt in Version 0. |
| Die Version ist v0.4.0-beta. | Veraltet. Die aktuelle Version ist v0.4.1. |
| Das SDK benötigt CGO. | Nur teilweise richtig. Nur die Desktop-App-Integration benötigt CGO. Das Service-Konto arbeitet ohne CGO. |

Offene Fehler im Repository, die `a38 serve` betreffen (alle **[P]**, Stand 2026-11-11, alle ohne Antwort von 1Password):

| Fehler | Zugang | Inhalt |
| --- | --- | --- |
| [#266][i266] | Desktop-App | Nach dem Sperren der 1Password-App hängt der Aufruf dauerhaft. Der Aufruf beachtet die Frist im `context` nicht. |
| [#290][i290] | Desktop-App | Gleichzeitige Aufrufe liefern beschädigte Antworten. Der Aufruf der Bibliothek hat keine Sperre. |
| [#250][i250] | Desktop-App | `Vaults().List` liefert als Tresortitel den Text `[Encrypted]`. |
| [#292][i292] | Desktop-App | Ein Client für ein zweites Konto nutzt das erste Konto. |
| [#289][i289] | Service-Konto | Ein Token, das die 1Password-CLI erzeugt, scheitert in `NewClient`. |
| [#209][i209] | beide | Der Client belegt viel Arbeitsspeicher. |

### 2. Autorisierung der Desktop-App-Integration

- **[P]** Der Benutzer schaltet in der 1Password-App unter „Settings > Developer“ die Option „Integrate with other apps“ ein. Quelle: [README][readme].
- **[P]** Die 1Password-App zeigt eine Abfrage, sobald ein neuer Prozess die Verbindung öffnet. Die Abfrage nennt das Konto, den anfragenden Prozess, den Umfang und die Dauer des Zugriffs. Quelle: [Sicherheitsmodell][dai].
- **[P]** Der Benutzer bestätigt die Abfrage mit der Methode, mit der er die App entsperrt. Auf dem Mac ist das zum Beispiel Touch ID. Quelle: [Sicherheitsmodell][dai].
- **[P]** Die App ermittelt den Aufrufer über die Prozess-ID (PID). Die App liest den Namen und den absoluten Pfad der ausführbaren Datei. Die App prüft keine Code-Signatur des Aufrufers. Quelle: [Sicherheitsmodell][dai].
- **[P]** Die Freigabe gilt für genau ein Konto und genau einen Prozess. Ein anderer Prozess benötigt eine eigene Freigabe. Quelle: [Sicherheitsmodell][dai].
- **[P]** Die Freigabe endet nach zehn Minuten ohne Aktivität. Die Freigabe endet sofort, wenn der Benutzer das Konto in der App sperrt. Quelle: [Sicherheitsmodell][dai].
- **[P]** Die Freigabe umfasst das gesamte Konto. Dazu gehören auch die Tresore „Private“ und „Employee“. Eine Begrenzung auf einzelne Tresore gibt es nicht. Quelle: [SDK-Konzepte][concepts].
- **[P]** Der Kanal auf dem Mac nutzt Mach-Ports. Quelle: [Sicherheitsmodell][dai].
- **[P]** Wenn die App nicht läuft, meldet das SDK schnell den Fehler „desktop app connection channel is closed“. Wenn die App läuft und gesperrt ist, erscheint die Touch-ID-Abfrage, und der Aufruf gelingt. Quelle: Beobachtung in [#266][i266].
- **[L]** Das SDK baut nach dem Fehler `DesktopSessionExpiredError` den Client einmal neu auf und wiederholt den Aufruf. Quelle: `client_builder.go` in v0.4.1.
- **[P]** Ein Benutzer mit Root-Rechten kann den Schutz umgehen. 1Password nennt das als akzeptiertes Risiko. Quelle: [Sicherheitsmodell][dai].

Folgen für `a38 serve`:

- **[A]** Die Abfrage nennt `a38` und den Installationspfad. Ob die Abfrage auch den Namen aus `WithIntegrationInfo` zeigt, ist nicht dokumentiert.
- **[A]** Die Bindung an ein Terminal gilt nur für die 1Password-CLI, nicht für das SDK. Ein LaunchAgent in der Benutzersitzung erreicht den Mach-Port. Ein Test steht aus.
- **[A]** Jeder Neustart von `a38 serve` erzeugt eine neue Abfrage, weil die Freigabe am Prozess hängt.
- **[A]** Nach zehn Minuten ohne Anfrage sieht der Benutzer zwei Abfragen: die Abfrage der 1Password-App und die Abfrage der Approval-App.
- Die Begrenzung auf Tresore mit `k8s` im Namen liegt bei diesem Zugang allein im Code von `a38 serve`.

### 3. Folgen von CGO

CGO betrifft nur die Desktop-App-Integration.

- **[L]** Mit CGO lädt das SDK zur Laufzeit die Bibliothek `libop_sdk_ipc_client.dylib` aus `/Applications/1Password.app/Contents/Frameworks/` mit `dlopen`. Quelle: `internal/shared_lib_core.go` und `internal/shared_lib_core_unix.go` in v0.4.1.
- **[A]** Die Bibliothek ist mit der Team-ID von 1Password signiert.
- **[A]** Eine notarisierte Binärdatei mit Hardened Runtime lädt nur Bibliotheken von Apple oder mit derselben Team-ID. `a38` benötigt deshalb das Entitlement `com.apple.security.cs.disable-library-validation`. Quelle: [Apple-Dokumentation][applelv]. Ein Test steht aus.
- **[P]** quill kann beim Signieren eine Entitlements-Datei setzen (`--entitlements`). Quelle: [quill, signing.go][quillsrc]. Die aktuelle Version ist v0.7.1.
- **[P]** Ein Cross-Build mit CGO benötigt `CGO_ENABLED=1` und einen passenden C-Cross-Compiler. Quelle: [README][readme].
- **[A]** Ein CGO-Build für darwin auf einem Linux-Runner benötigt zusätzlich ein macOS-SDK, weil der Linker die Frameworks `CoreFoundation` und `Security` braucht. Die Alternative ist ein macOS-Runner für die darwin-Builds.
- **[A]** Eine darwin-Binärdatei ist auch ohne CGO dynamisch an `libSystem` gebunden. Der statische Build betrifft nur linux.

Trennung von `a38 mcp` und dem SDK:

- **[A]** Eine Datei mit `//go:build darwin` hält das SDK vollständig aus dem linux-Build. Die Aussage folgt aus den Regeln für Build-Bedingungen in Go.
- **[A]** Die darwin-Binärdatei wächst durch das SDK um mehr als 10 MB. Der eingebettete Kern hat allein 9,5 MB.
- **[L]** Auf darwin enthält dieselbe Binärdatei das SDK auch für `a38 mcp`. Das SDK lädt seinen Kern erst beim Aufruf von `NewClient`. Quelle: `internal/extism_core.go`.
- **[A]** Ein eigenes Build-Tag (zum Beispiel `desktopapp`) kann zusätzlich die Desktop-App-Integration vom Standard-Build trennen.

### 4. Service-Konto

Rechte:

- **[P]** Ein Service-Konto erhält je Tresor die Rechte `read_items`, `write_items` und `share_items`. `write_items` und `share_items` setzen `read_items` voraus. Ein Zugriff nur mit `read_items` ist möglich. Quelle: [Erste Schritte][sags].
- **[P]** Ein Service-Konto liest nur Tresore, für die es `read_items` hat. Ein Service-Konto lässt sich so auf die Dev-Tresore begrenzen. Quelle: [Sicherheit von Service-Konten][sasec].
- **[P]** Rechte, Tresorzugriff und Zugriff auf Environments sind nach dem Anlegen unveränderlich. Ein weiterer Tresor erfordert ein neues Service-Konto. Quelle: [Service-Konten verwalten][samgmt].
- **[P]** Ein Service-Konto erhält keinen Zugriff auf die Tresore „Personal“, „Private“, „Employee“ und den Standard-Tresor „Shared“. Quelle: [Erste Schritte][sags].
- **[P]** Der Ersteller gibt nur Tresore frei, für die er das Recht „Manage Vault“ hat. Ein Tresor kann Service-Konten ganz abschalten. Quelle: [Sicherheit von Service-Konten][sasec].
- Eine Obergrenze für die Zahl der Tresore je Service-Konto steht in keiner der gelesenen Primärquellen. Der Punkt ist offen.

Rate Limits (alle **[P]**, Quelle: [Rate Limits][sarl]):

| Kontotyp | Lesen je Stunde und Token | Schreiben je Stunde und Token | Anfragen je Tag und Konto |
| --- | --- | --- | --- |
| 1Password Business | 10.000 | 1.000 | 50.000 |
| 1Password Teams | 1.000 | 100 | 5.000 |
| 1Password, 1Password Families | 1.000 | 100 | 1.000 |

- **[P]** Die Limits gelten auch für das SDK. Quelle: [SDK-Konzepte][concepts].
- **[L]** Das SDK meldet eine Überschreitung mit dem Typ `RateLimitExceededError`.
- Die Zahl der Anfragen je SDK-Aufruf ist nicht dokumentiert.

Token:

- **[P]** Das Token enthält das Schlüsselmaterial des Service-Kontos. Dazu gehören der Account Unlock Key und die Daten für Secure Remote Password. Quelle: [Sicherheit von Service-Konten][sasec].
- **[P]** 1Password zeigt das Token nur einmal. Quelle: [Erste Schritte][sags].
- **[P]** Die Rotation erzeugt ein neues Token mit denselben Rechten. Der Benutzer wählt, wann das alte Token abläuft, zum Beispiel sofort, nach einer Stunde oder nach drei Tagen. Quelle: [Service-Konten verwalten][samgmt].
- **[P]** Der Widerruf entzieht dem Token sofort den Zugriff. Quelle: [Service-Konten verwalten][samgmt].
- **[P]** `op service-account create --expires-in` legt ein Service-Konto mit Ablaufzeit an. Quelle: [CLI-Referenz][clisa].
- **[A]** Ein entwendetes Token legt alle Einträge der freigegebenen Tresore mit den gewählten Rechten offen. Die Offenlegung dauert bis zum Widerruf oder bis zum Ablauf. Die Aussage folgt aus dem Rechtemodell.

Ablage des Tokens auf dem Mac (Bewertung, alle **[A]**):

| Ablage | Bewertung |
| --- | --- |
| macOS-Schlüsselbund | Empfohlen. Der Zugriff ohne CGO läuft über `/usr/bin/security`. Die Zugriffsliste des Schlüsselbund-Eintrags nennt dann `security` und nicht `a38`. |
| Datei mit Modus 0600 | Einfach. Jeder Prozess des Benutzers liest die Datei. Die Datei gelangt leicht in eine Datensicherung. |
| Abruf über die Desktop-App-Integration beim Start | Das Token liegt nicht auf der Platte. Die Lösung benötigt CGO und erbt alle Folgen aus Abschnitt 3. |
| Umgebungsvariable in der LaunchAgent-Datei | Nicht empfohlen. Das Token steht dann im Klartext in einer plist-Datei. |

### 5. API-Umfang des SDK

Alle Aussagen sind **[L]** (Quelltext v0.4.1) und durch die Dokumentation gestützt ([Secrets laden][load], [Tresore und Einträge auflisten][list]).

| Aufgabe | Aufruf | Ergebnis |
| --- | --- | --- |
| Eine Secret-Referenz auflösen | `client.Secrets().Resolve(ctx, "op://beispiel.app.k8s-dev/datenbank/password")` | Wert des Felds |
| Mehrere Secret-Referenzen auflösen | `client.Secrets().ResolveAll(ctx, refs)` | Je Referenz der Wert, `ItemID`, `VaultID` oder ein Fehler |
| Syntax einer Secret-Referenz prüfen | `onepassword.Secrets.ValidateSecretReference(ctx, ref)` | Fehler bei falscher Syntax, kein Netzwerkzugriff |
| Tresore auflisten | `client.Vaults().List(ctx)` | `VaultOverview` mit `ID`, `Title`, `Description`, `ActiveItemCount` |
| Einträge eines Tresors auflisten | `client.Items().List(ctx, vaultID)` | `ItemOverview` mit `ID`, `Title`, `Category`, `VaultID`, `Tags`, `Websites`, `State`, Zeitstempeln |
| Einen Eintrag lesen | `client.Items().Get(ctx, vaultID, itemID)` | `Item` mit allen Feldern und allen Werten |

- `ItemOverview` enthält keine Feldwerte und keine Feldnamen. Das SDK listet also Metadaten der Einträge, ohne Secrets zu lesen.
- Feldnamen liefert nur `Items().Get`. Dieser Aufruf lädt auch die geheimen Werte in den Prozess.
- Eine Secret-Referenz akzeptiert Namen oder IDs. 1Password empfiehlt IDs, wenn Namen mehrfach vorkommen.
- `Vaults().List` nimmt den Parameter `VaultListParams.DecryptDetails` an. **[A]** Der Parameter behebt den Titel `[Encrypted]` aus [#250][i250]. Ein Test steht aus.
- Der Kern für das Service-Konto serialisiert Aufrufe mit einer Sperre. Der Kern für die Desktop-App-Integration hat keine Sperre (siehe [#290][i290]).

Folge für die Abfrage der Approval-App: `a38 serve` löst Tresortitel und Eintragstitel über `Vaults().List` und `Items().List` auf und zeigt diese geprüften Titel an. Den Feldnamen übernimmt `a38 serve` aus der Secret-Referenz.

### 6. Testbarkeit

- **[L]** `SecretsAPI`, `ItemsAPI` und `VaultsAPI` sind exportierte Go-Interfaces. Eine eigene Fake-Implementierung ist ohne Änderung am SDK möglich.
- **[P]** 1Password liefert kein Test-Double. Der Wunsch ist seit 2024 offen. Quelle: [#98][i98].
- **[P]** 1Password testet das SDK selbst gegen ein echtes Testkonto mit einem Service-Konto-Token aus den CI-Secrets. Quelle: [validate.yml][validate].
- Empfehlung: `a38 serve` definiert eine eigene schmale Schnittstelle. Die Schnittstelle enthält nur die Aufrufe aus Abschnitt 5 und keine SDK-Typen.
- Die CI (Continuous Integration) testet gegen eine Fake-Implementierung im Arbeitsspeicher. Die CI benötigt dann kein Token.
- Ein Vertragstest gegen ein echtes Service-Konto ist optional. Der Test läuft nur, wenn `OP_SERVICE_ACCOUNT_TOKEN` gesetzt ist.

### 7. Lizenz und mitgelieferter Kern

- **[L]** Der Go-Code des SDK steht unter der MIT-Lizenz. Quelle: `LICENSE` in v0.4.1.
- **[P]** Die Nutzung der 1Password-Dienste unterliegt zusätzlich den „1Password API Terms of Service“. Quelle: [README][readme].
- **[L]** Das Modul enthält einen vorgebauten Kern: `internal/wasm/core.wasm` mit 9,5 MB. Der Go-Code bettet die Datei mit `go:embed` ein. Der Quelltext des Kerns liegt nicht im Repository.
- **[L]** Die SHA-256-Prüfsumme der Datei in v0.4.1 ist `23d115f4ac7519b48172df3e8615945572dbda7033d51b44c9490fd533ae0f23`.
- **[L]** Der Kern läuft in der WebAssembly-Laufzeit wazero über `github.com/extism/go-sdk`. extism steht unter BSD-3-Clause. wazero steht unter Apache-2.0.
- **[L]** Der Kern erreicht nur Hosts von 1Password, zum Beispiel `*.1password.com` und `*.1password.eu`. Quelle: `internal/extism_core.go`.
- **[L]** Die Bibliothek der Desktop-App-Integration ist nicht Teil des Moduls. Das SDK lädt sie aus der installierten 1Password-App.

Folgen für SBOM (Software Bill of Materials) und Lieferkette:

- Eine SBOM auf Modulebene nennt das Modul `github.com/1password/onepassword-sdk-go`, aber nicht die Datei `core.wasm`.
- `go.sum` deckt die Datei über die Prüfsumme des Moduls ab.
- Ein Quelltext-Scan sieht den Kern nicht. Der Kern ist ein nicht prüfbarer Binärbestandteil.
- Bei der Desktop-App-Integration führt `a38` zusätzlich Code aus, dessen Version die installierte 1Password-App bestimmt.

## Optionen

| Option | Vorteile | Nachteile | Aufwand |
| --- | --- | --- | --- |
| A: nur Service-Konto | Kein CGO. Der Build bleibt statisch und auf dem Linux-Runner. 1Password begrenzt den Zugriff auf die Dev-Tresore. Nur `read_items`. Keine zweite Abfrage. Aufrufe sind serialisiert. | Ein Token liegt dauerhaft auf dem Mac. Neue Tresore erfordern ein neues Service-Konto. Rate Limits gelten. Das Anlegen erfordert Rechte im 1Password-Konto. | Klein |
| B: nur Desktop-App-Integration | Kein Token auf dem Mac. Das Sperren von 1Password entzieht den Zugriff sofort. Neue Tresore sind sofort sichtbar. Zugriffe sind dem Benutzer zugeordnet. | CGO für darwin. Vermutlich macOS-Runner und zusätzliches Entitlement. Freigabe für das ganze Konto. Zweite Abfrage nach zehn Minuten ohne Aktivität. Offene Fehler #266, #290, #250, #292. | Groß |
| C: Schnittstelle mit beiden Zugängen, Service-Konto zuerst | Die Vorteile von A gelten sofort. B lässt sich später ohne Umbau ergänzen. Die CI nutzt dieselbe Schnittstelle für die Fake-Implementierung. | Zwei Implementierungen erfordern Pflege. Die Konfiguration erhält einen Schalter. | Klein, später mittel |
| D: Service-Konto, Token-Abruf über die Desktop-App-Integration | Das Token liegt nicht auf der Platte. | Vereint die Nachteile von A und B. | Groß |

## Empfehlung

Der Design-Lauf wählt Option C.

Entscheidungen für den Design-Lauf:

- `a38 serve` greift nur über eine eigene Go-Schnittstelle auf 1Password zu. Die Schnittstelle enthält keine SDK-Typen.
- Die erste Implementierung nutzt ein Service-Konto mit `read_items` auf den Dev-Tresoren.
- Die Dateien mit dem SDK tragen `//go:build darwin`. Der linux-Build enthält keinen Code von 1Password.
- Der Release-Build bleibt bei `CGO_ENABLED=0`.
- `a38 serve` speichert das Token im macOS-Schlüsselbund und gibt es nie aus.
- `a38 serve` prüft zusätzlich selbst, dass der Tresorname `k8s` enthält.
- `a38 serve` löst Tresortitel und Eintragstitel über `Vaults().List` und `Items().List` auf. `a38 serve` ruft `Items().Get` nicht auf.
- Die Version des SDK ist in `go.mod` fest. Eine Aktualisierung ist eine bewusste Änderung mit Prüfung der Release Notes.

Lücken für den Design-Lauf:

- Das Verfahren für einen neuen Dev-Tresor fehlt. Ein neuer Tresor erfordert ein neues Service-Konto und ein neues Token.
- Das Verfahren für die Rotation des Tokens fehlt.
- Die Obergrenze für Tresore je Service-Konto ist unbekannt.
- Die Zahl der Anfragen je SDK-Aufruf ist unbekannt. Ein Zwischenspeicher für Tresortitel und Eintragstitel senkt den Verbrauch.

Risiken für den Design-Lauf:

- Das Token gewährt dauerhaften Lesezugriff auf alle freigegebenen Dev-Tresore. Ein Prozess des Benutzers auf dem Mac kann das Token lesen.
- Das SDK steht in Version 0. Inkompatible Änderungen sind möglich.
- Der Kern `core.wasm` ist ein nicht prüfbarer Binärbestandteil.
- Die Desktop-App-Integration hat offene Fehler ohne Antwort von 1Password. Eine spätere Ergänzung hängt von deren Behebung ab.
- Fehler [#289][i289] kann das Anlegen des Tokens über die 1Password-CLI verhindern.

## Offene Punkte

Diese Punkte prüft nur Stefan auf seinem Mac, nach Rücksprache. Kein Test gibt ein Secret aus.

Tests zum Build (ohne Zugriff auf 1Password):

1. Testprogramm mit dem SDK hinter `//go:build darwin` anlegen.
2. Für linux mit `CGO_ENABLED=0` bauen. Mit `go list -deps` prüfen, dass kein Paket von 1Password, extism oder wazero enthalten ist.
3. Für darwin mit `CGO_ENABLED=0` bauen. Die Größe der Binärdatei mit und ohne SDK vergleichen.
4. Auf einem Linux-Runner für darwin mit `CGO_ENABLED=1` bauen. Prüfen, welcher C-Cross-Compiler und welches macOS-SDK nötig sind.
5. Die Signatur von `libop_sdk_ipc_client.dylib` in der 1Password-App mit `codesign -dv` lesen. Team-ID und Hardened-Runtime-Flag notieren.

Tests zum Service-Konto:

1. Service-Konto in der Weboberfläche anlegen, mit `read_items` auf `beispiel.app.k8s-dev`.
2. Prüfen, ob die Weboberfläche eine Obergrenze für die Zahl der Tresore nennt.
3. Token im macOS-Schlüsselbund speichern.
4. Testprogramm mit `CGO_ENABLED=0` bauen. Das Testprogramm ruft `Vaults().List` und `Items().List` auf und gibt nur Titel aus.
5. Prüfen, ob die Liste nur die freigegebenen Tresore enthält.
6. Eine Secret-Referenz auflösen und nur die Länge des Werts ausgeben.
7. Verbrauch der Rate Limits vor und nach dem Test in der Weboberfläche vergleichen.
8. Token rotieren und prüfen, ob das alte Token nach der gewählten Frist scheitert.

Tests zur Desktop-App-Integration (nur für die spätere Ergänzung):

1. Option „Integrate with other apps“ einschalten.
2. Testprogramm mit `CGO_ENABLED=1` bauen und im Terminal starten. Den Text der Abfrage notieren: Prozessname, Pfad, Name aus `WithIntegrationInfo`.
3. Dasselbe Testprogramm als LaunchAgent starten. Prüfen, ob die Abfrage erscheint.
4. Elf Minuten warten und erneut aufrufen. Prüfen, ob eine neue Abfrage erscheint.
5. 1Password sperren und erneut aufrufen. Prüfen, ob der Aufruf hängt (Fehler #266).
6. 1Password beenden und erneut aufrufen. Die Fehlermeldung notieren.
7. `Vaults().List` ohne und mit `DecryptDetails` aufrufen. Prüfen, ob der Titel `[Encrypted]` lautet (Fehler #250).
8. Testprogramm mit quill signieren, einmal ohne und einmal mit dem Entitlement `com.apple.security.cs.disable-library-validation`. Prüfen, ob `dlopen` jeweils gelingt.
9. Zehn gleichzeitige Aufrufe von `Items().List` starten. Prüfen, ob Fehler auftreten (Fehler #290).

## Quellen

Primärquellen von 1Password:

- [Repository des Go-SDK][repo]
- [README des Go-SDK][readme]
- [Releases des Go-SDK][rel]
- [SDK-Übersicht][sdks]
- [SDK-Konzepte][concepts]
- [Sicherheitsmodell der Desktop-App-Integration][dai]
- [Secrets laden][load]
- [Tresore und Einträge auflisten][list]
- [Service-Konten: Erste Schritte][sags]
- [Service-Konten: Sicherheit][sasec]
- [Service-Konten: Verwalten][samgmt]
- [Service-Konten: Rate Limits][sarl]
- [CLI-Referenz `op service-account`][clisa]
- [CI-Workflow des Go-SDK][validate]
- Fehlerberichte: [#98][i98], [#209][i209], [#250][i250], [#266][i266], [#289][i289], [#290][i290], [#292][i292]

Weitere Quellen:

- [quill, Quelltext der Signatur-Optionen][quillsrc]
- [Apple: Disable Library Validation Entitlement][applelv]

Gelesener Quelltext:

- Öffentliches Modul `github.com/1password/onepassword-sdk-go@v0.4.1`, bezogen über den Go-Modul-Proxy. Der Quelltext ist nicht ausgeführt.

[repo]: https://github.com/1Password/onepassword-sdk-go
[readme]: https://github.com/1Password/onepassword-sdk-go/blob/v0.4.1/README.md
[rel]: https://github.com/1Password/onepassword-sdk-go/releases
[validate]: https://github.com/1Password/onepassword-sdk-go/blob/v0.4.1/.github/workflows/validate.yml
[sdks]: https://www.1password.dev/sdks
[concepts]: https://www.1password.dev/sdks/concepts
[dai]: https://www.1password.dev/sdks/desktop-app-integrations
[load]: https://www.1password.dev/sdks/load-secrets
[list]: https://www.1password.dev/sdks/list-vaults-items
[sags]: https://www.1password.dev/service-accounts/get-started
[sasec]: https://www.1password.dev/service-accounts/security
[samgmt]: https://www.1password.dev/service-accounts/manage-service-accounts
[sarl]: https://www.1password.dev/service-accounts/rate-limits
[clisa]: https://www.1password.dev/cli/reference/management-commands/service-account
[i98]: https://github.com/1Password/onepassword-sdk-go/issues/98
[i209]: https://github.com/1Password/onepassword-sdk-go/issues/209
[i250]: https://github.com/1Password/onepassword-sdk-go/issues/250
[i266]: https://github.com/1Password/onepassword-sdk-go/issues/266
[i289]: https://github.com/1Password/onepassword-sdk-go/issues/289
[i290]: https://github.com/1Password/onepassword-sdk-go/issues/290
[i292]: https://github.com/1Password/onepassword-sdk-go/issues/292
[quillsrc]: https://github.com/anchore/quill/blob/main/cmd/quill/cli/options/signing.go
[applelv]: https://developer.apple.com/documentation/bundleresources/entitlements/com.apple.security.cs.disable-library-validation
