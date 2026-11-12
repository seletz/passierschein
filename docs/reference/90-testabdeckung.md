---
title: Testabdeckung der Stufe 1
description: "Welche Testfälle des Designs ein automatischer Test belegt, welche nur teilweise und welche noch fehlen."
tags:
  - Thema/Qualität
render_macros: true
---
# Testabdeckung der Stufe 1

Diese Seite ordnet jeden Testfall des Designs ({{ ref("T1") }} bis {{ ref("T28") }}) den Go-Tests zu.
Die Zuordnung stammt aus dem Code, nicht aus den Texten der Pull Requests.
Jeder Go-Test nennt seinen Testfall im Kommentar (`// T7: …`).
Für jeden Testfall wurde der Test gelesen und mit dem Text des Designs verglichen.

## Zustände

| Zustand | Bedeutung | Wert im Register (`umsetzung`) |
|---|---|---|
| umgesetzt | Ein automatischer Test belegt jeden Fall und jede Erwartung des Designtextes. | `vorhanden` |
| teilweise | Ein Teil ist belegt. Die Spalte „Fehlt“ nennt den Rest. | `teilweise` |
| zu-bauen | Es gibt noch keinen Test. | `zu-bauen` |

Das Register kennt den Wert `umgesetzt` nicht.
Es nutzt `vorhanden` dafür.

## Stand

Von 28 Testfällen sind 25 umgesetzt und 3 teilweise.
Das 1Password-Backend ({{ ref("T15") }}) ist belegt: Die Unit-Tests laufen in jeder CI, der Integrationstest läuft automatisch im macOS-Workflow mit dem Service Account der CI ([CI](55-ci.md)).
Die Freigabe-App ({{ ref("T14") }}) ist teilweise belegt: Teil 1 ist automatisch, Teil 2 ist manuell.

Die Tests nutzen zwei Arten von Testaufbau.
Die meisten laufen im Prozess des Tests mit einer gefälschten Uhr und einer aufzeichnenden Freigabe (Paket `broker`, Hilfstyp `fakeui.Recorder`).
Diese Aufzeichnung ersetzt das Skript `--ui script` des Designs.
Die Tests `TestMCPProcessesEndToEnd`, `TestServeGlobalLock`, `TestHumanCommandsWithDaemonProcess`, `TestServeRestartEndsGrants`, `TestServeSessionNameInAuditLog`, `TestServeNoSessionNameInAuditLog` und `TestServeInvalidVaultList` starten die echten Prozesse `a38 serve` und `a38 mcp`.

## Tabelle

Die Pakete stehen ohne den Präfix `internal/`.
`cmd/a38` ist das Paket `main` des Binarys.

| Testfall | Zustand | Testfunktionen | Fehlt oder Anmerkung |
|---|---|---|---|
| {{ ref("T1") }} | umgesetzt | `cmd/a38`: `TestMCPProcessesEndToEnd` (echte Prozesse, `auto-approve` und `auto-deny`); `broker`: `TestFlowAuditSequence`; `mcp`: `TestRequestCredentialsSuccessAndMixed`, `TestSecretOnlyInResult`; `cmd/a38`: `TestServeRuns` | – |
| {{ ref("T2") }} | umgesetzt | `broker`: `TestBundleCollectsRequestsOfTheWindow`, `TestBundleWindowEdges`, `TestBundleRequestAtDeadlineOpensNewBundle`, `TestBundleCoveredRequestOpensNoWindow`, `TestBundleTwoSessionsTwoBundlesInSequence`, `TestBundleOpenButExpiredOpensNewBundle`; `bundle`: `TestAccepts` | – |
| {{ ref("T3") }} | umgesetzt | `broker`: `TestPolicyRejectsBeforeBackendAndHuman` (zehn Fälle, auch alles oder nichts); `policy`: `TestCheck` | – |
| {{ ref("T4") }} | umgesetzt | `broker`: `TestGrantCoversUntilLifetimeEnds` (4 h, 30 min, 12 h, 15 min), `TestInvalidDecisions` (0, negativ, länger als die Konfiguration), `TestRunWritesExpiry`; `fakeui`: `TestValidateDecision`, `TestScriptApprove` (Text als Laufzeit); `config`: `TestParseDefaults`, `TestParseAccepts`, `TestParseRejects` | – |
| {{ ref("T5") }} | umgesetzt | `broker`: `TestDeviationCreatesFurtherGrant` | – |
| {{ ref("T6") }} | umgesetzt | `broker`: `TestDenialTimeoutAndUIFailure`, `TestScriptUIFailures` (Skript scheitert, kein JSON, `approved` mit Exit-Code 1); `fakeui`: `TestAutoDeny`, `TestScriptTimeout`; `queue`: `TestQueueTimeout`; `cmd/a38`: `TestMCPProcessesEndToEnd` | – |
| {{ ref("T7") }} | umgesetzt | `audit/sqlite`: `TestVerifyTamper` (zehn Einträge, alle fünf Fälle, auch „letzter Eintrag entfernt“ als bekannte Grenze); `cmd/a38`: `TestAuditVerify` | – |
| {{ ref("T8") }} | umgesetzt | `broker`: `TestSecretValueNeverLeaks` (Freigabe, Ablehnung, Zeitüberschreitung, UI-Fehler, Backend-Fehler, Audit-Datei), `TestNoValueCache`; `fake`: `TestSetValue`, `TestNoValueInErrorsOrFormat`; `protocol`: `TestDecodeErrorHidesContent`, `TestCredentialsValueOnlyOnTheWire`; `audit/sqlite`: `TestNoSecretInEntry` | – |
| {{ ref("T9") }} | umgesetzt | `cmd/a38`: `TestServeFlags` (`1password` mit `auto-approve`, `auto-deny` und `script`), `TestServeStartFailures` (`1password` mit `app` ohne gültige Token-Datei, kein Socket); `fakeui`: `TestCheckCombination` (alle Kombinationen); `approval`: `TestModeFaked`; `backend`: `TestKindReal` | Den erlaubten Fall `--backend 1password --ui app` mit gültigem Token belegt nur der Integrationstest von {{ ref("T15") }}; ein automatischer Test würde 1Password erreichen. |
| {{ ref("T10") }} | umgesetzt | `daemon`: `TestInvalidMessages` (23 Fälle), `FuzzAgentConn`; `protocol`: `TestDecodeRejects`, `TestReadLine`, `FuzzDecode` | `mise run test` führt die Fuzz-Tests nur mit ihren Startwerten aus. Ein längerer Fuzz-Lauf ist nicht Teil der CI. |
| {{ ref("T11") }} | umgesetzt | `daemon`: `TestFactsComeFromTheDaemon`, `TestProjectFromWorkingDirectory`, `TestUnknownCwdHasNoProject`, `TestNewSessionID`; `daemon`: `TestReadPeer`; `broker`: `TestApprovalRequestContent`, `TestPolicyUsesVerifiedProjectNotClaims`, `TestGrantOfOtherSessionDoesNotCover`; `config`: `TestProjectFor`; `session`: `TestSessionClaims`; `approval`: `TestRequestKeepsFactsAndClaimsApart` | – |
| {{ ref("T12") }} | umgesetzt | `daemon`: `TestSocketModes`, `TestListenRejectsSocketDir`, `TestListenCreatesSocketDir`, `TestAdminMessagesOnAgentSocket`, `TestAdminSocket`; `cmd/a38`: `TestHumanCommandsWithDaemonProcess` (Agent-Socket lehnt `grants_list` und `grant_revoke` ab); `protocol`: `TestDecodeRejects` | – |
| {{ ref("T13") }} | umgesetzt | Task `mise run deps:linux:check` (Skript `scripts/linux-deps-check.sh` mit Selbsttest, Teil von `mise run check` und der CI); `mcp`: `TestImportRule` | Die Form ist eine Prüfung der CI, kein Go-Test. Der Selbsttest prüft die Regel mit gefälschten Paketlisten, dann mit echten: Der darwin-Build enthält `internal/backend/onepassword` und das SDK, und ein linux-Build mit eingeschalteter Bedingung `darwin` (als fehlte die Build-Bedingung) fällt durch. Die Regel verbietet auch `extism` und `wazero`, die Laufzeit des SDK. |
| {{ ref("T14") }} | teilweise | Teil 1 automatisch: Swift (`mise run app:test`): `DecodingTests` (jede ungültige Eingabe), `ApprovalViewModelTests` (Regeln des Popups), `DecisionTests` (Entscheidung, Exit-Codes, letzte Prüfung, Schalter, Meldung), `ProcessTests` (Programm als Prozess wie vom Daemon gestartet); `mise run app:build` (der Release-Build lehnt jeden Testschalter ab). Go: `approval`: `TestAppSamples` (jede Datei unter `app/Samples` streng gelesen und ohne Verlust zurückgeschrieben); `appui`: `TestApprove`, `TestRequestHoldsNoSecret`, `TestApproveRejectsTimeoutOutOfRange`; `queue`: `TestQueueTimeoutSecondsInAppRange`; `config`: `TestParseRejects` (Obergrenze über 50); `cmd/a38`: `TestServeUIApp` (Programm als Kindprozess). Echte Prozesse mit der Debug-App: `cmd/a38`: `TestAppE2E` (`mise run app:e2e`, nur macOS, nicht Teil von `check`): gefälschte Authentifizierung wird abgelehnt (`ui_failed`, kein Wert, kein Grant, Eintrag `error`), `timeout`, Meldung; das JSON fließt in beide Richtungen. | In der CI (macOS-Workflow) laufen `mise run app:build`, `mise run app:test` und `mise run app:e2e` mit `TestAppE2E` in der grafischen Sitzung des Runners. Teil 2 bleibt manuell am Bildschirm: Eingabetaste und Leertaste, die Verzögerung von 1,5 Sekunden, Name der App im echten Dialog von Touch ID, Fenster unter OmniWM und über Vollbild-Apps ({{ ref("G4") }}). Das echte Touch ID hat noch niemand ausgelöst. Der Ablauf ist mit gefälschter Authentifizierung belegt, nie mit einer echten Freigabe. Signatur und Notarisierung ({{ ref("G5") }}) fehlen. |
| {{ ref("T15") }} | umgesetzt | Automatisch, ohne 1Password: `onepassword`: `TestResolveAndRead`, `TestVaultsOnlyFromConfiguration`, `TestNotFound`, `TestDuplicateTitlesFailClosed`, `TestReadByID`, `TestReadWithoutResolveAndDeletedItem`, `TestReadIsNeverCached`, `TestErrorMapping`, `TestStateErrorsOfVaults`, `TestTimeout`, `TestConnectIsLazyAndRetries`, `TestBackendNeverPrintsToken`; `tokenfile`: `TestReadAndCheck`, `TestCheckInfoOwner`, `TestTokenNeverPrints`; `doctor`: `TestTokenFileCheck`, `TestBackendCheckOnePassword`; `config`: `TestParseDefaults`, `TestParseRejects`; `broker`: `TestStatusBackendKind`. Von Hand: `onepassword`: `TestIntegration` (Build-Tags `darwin opintegration`, `mise run test:onepassword`, Vault `passierschein.dev`) | Der Integrationstest ist am 2026-11-12 bestanden: alle sieben Prüfungen gegen den Test-Vault `passierschein.dev` von Stefan, gestartet vom Koordinator mit Stefans Erlaubnis. Ein kaputtes Token endete mit `backend_locked`. Der Praxistest mit `a38 serve --backend 1password --ui app` und der echten App ist ebenfalls bestanden (Freigabe, Grant, Teilauswahl, Ablehnung, Ende der Sitzung, Hash-Kette intakt). Seit Issue 55 läuft `TestIntegration` automatisch im macOS-Workflow (`mise run test:onepassword:ci`, Service Account der CI, nur `passierschein.dev`): bei jedem Push auf `develop`, jede Nacht und in Pull Requests mit passenden Pfaden; erster Lauf in der CI am 2026-11-12 bestanden. Von Hand bleibt `mise run test:onepassword` mit dem Token des Macs. Nicht geprüft gegen 1Password: weitere Fehlermeldungen des SDK (nur „Token abgelehnt“), Netzfehler und Rate Limit. Ein Token in gültiger Form, das 1Password nicht kennt, prüft der Test nicht: Ohne ein zweites echtes Token lässt sich die Form nicht sicher nachbauen. |
| {{ ref("T16") }} | umgesetzt | `broker`: `TestGrantEnds` (Sitzung endet, Widerruf, Neustart mit dem Grund `daemon_stopped`), `TestShutdownEndsGrants`, `TestRevokeUnknownGrant`; `grant`: `TestEnds`; `cmd/a38`: `TestHumanCommandsWithDaemonProcess` (`a38 grants revoke` am echten Daemon), `TestServeRestartEndsGrants` (echter Prozess: SIGTERM, Grund im Audit-Log, neue Freigabe nach dem Neustart, `a38 audit verify` über den Neustart) | Grenze: Ein Ende ohne Signal (SIGKILL, Absturz) schreibt keinen Eintrag, denn Grants leben nur im Arbeitsspeicher. Der Test belegt nur das geordnete Ende (SIGINT, SIGTERM). Die Seite zur Architektur nennt die Grenze. |
| {{ ref("T17") }} | umgesetzt | `broker`: `TestBackendErrors` (gesperrt und nicht erreichbar, beim Auflösen und beim Lesen); `mcp`: `TestDaemonUnavailableThenReachable`; `backend`: `TestCode`; `fake`: `TestErrorStates` | – |
| {{ ref("T18") }} | teilweise | Tasks `mise run release:sbom:check` (Skript `scripts/sbom-check.sh`) und `mise run release:signatures:check` (Skript `scripts/signatures-check.sh`) | Die Prüfung der SBOMs und der Prüfsummendatei läuft in der CI nach `mise run release:snapshot`. Die Prüfung der Signatur läuft nur im manuell gestarteten Workflow `signed-snapshot.yml`, weil sie die Secrets braucht. Dieser Lauf ist hier nicht belegt. |
| {{ ref("T19") }} | umgesetzt | `broker`: `TestBundleMaximum` (Standard 10, Wert 3, genau die Obergrenze), `TestBundleMaximumOverRequests`, `TestBundleTwoSessionsTwoBundlesInSequence`; `bundle`: `TestAddMaximum`; `config`: `TestParseDefaults`, `TestParseRejects` (Fenster 0, Obergrenze 0) | – |
| {{ ref("T20") }} | umgesetzt | `cmd/a38`: `TestLLMSkillPrintsEmbeddedFile`; `skill`: `TestTextNamesCodesAndTools` (jeder Fehlercode des Protokolls, beide Werkzeuge) | – |
| {{ ref("T21") }} | teilweise | `daemon`: `TestHelloSessionName` (80 und 81 Zeichen, andere Zeichen, Steuerzeichen, Höchstlänge 20 mit Punkt), `TestSessionNameConflict`, `TestSessionNameConflictDuringRequest`, `TestSessionNameFixedPerConnection`; `config`: `TestSessionNameCheck`, `TestParseRejects` (Höchstlänge 0, leere Zeichenmenge); `cmd/a38`: `TestMCPFlags`, `TestMCPSessionNameReachesDaemon`, `TestAuditList`, `TestDashStripsControl`, `TestServeSessionNameInAuditLog` (echte Prozesse: die fünf Einträge tragen den Namen bei den Angaben, nicht bei den geprüften Fakten; `a38 audit list` zeigt ihn), `TestServeNoSessionNameInAuditLog`; `mcp`: `TestSessionName`; `broker`: `TestApprovalRequestContent`, `TestApprovalRequestWithoutSessionName`, `TestGrantEnds` | Die Freigabeanfrage ohne Namen trägt einen leeren Namen und keinen erfundenen (Test im Broker mit der aufzeichnenden Freigabe). Die ausdrückliche Leermarke zeigt erst die Freigabe-App aus Stufe 2. Der Test der Anzeige fehlt, bis es die App gibt. |
| {{ ref("T22") }} | umgesetzt | `config`: `TestParseRejects` (Umgebung fehlt, `staging`, leer, `Dev`, derselbe Vault zweimal), `TestInvalidProject`; `broker`: `TestPolicyRejectsBeforeBackendAndHuman`, `TestPolicyUsesEnvironmentNotName`, `TestConfigInvalidProject`; `cmd/a38`: `TestServeStartFailures`, `TestServeInvalidVaultList` (echter Prozess: Vault ohne Umgebung, unbekannte Umgebung, doppelter Vault; Exit-Code ungleich 0, Meldung mit dem Vault, kein Socket) | – |
| {{ ref("T23") }} | umgesetzt | `broker`: `TestPartialSelection`, `TestInvalidDecisions` (Eintrag nicht in der Anfrage, Freigabe ohne Häkchen); `fakeui`: `TestValidateDecision`, `TestScriptApprove`; `grant`: `TestCover` | – |
| {{ ref("T24") }} | umgesetzt | `broker`: `TestApprovalShowsBackendNames`, `TestNotFoundAndInvalid`, `TestPolicyRejectsBeforeBackendAndHuman`; `protocol`: `TestParseReference`; `mcp`: `TestInvalidReferences`, `TestDaemonRejectsReference`; `daemon`: `TestInvalidMessages` | Das Fake-Backend vergleicht Namen exakt. Den Fall „andere Schreibweise“ stellt der Test mit einem Backend nach, das ohne Beachtung der Groß- und Kleinschreibung findet. |
| {{ ref("T25") }} | umgesetzt | `broker`: `TestGrantStatusOwnSessionOnly`; `mcp`: `TestGrantStatusHasNoValue`; `grant`: `TestEnds`; `cmd/a38`: `TestServeRuns` | – |
| {{ ref("T26") }} | umgesetzt | `broker`: `TestT26AuditWriteFailure` (Anfrage, Grant, Auslieferung unter Grant), `TestAuditFailure`, `TestConcurrentRequestsDuringFault`; `audit/sqlite`: `TestAppendErrors`; `faultstore`: `TestFailAppends`; `fault`: `TestRaiseWithBrokenAuditLog`; `cmd/a38`: `TestServeGlobalLock` (echter Prozess) | – |
| {{ ref("T27") }} | umgesetzt | `cmd/a38`: `TestDoctor` (zwölf Fälle, Text und JSON, alle sieben Fälle des Designs), `TestHumanCommandsWithDaemonProcess`, `TestServeGlobalLock`; `doctor`: `TestSocketCheckWithRealFiles`, `TestAuditChecksWithRealFiles`, `TestSystemEnvComponents`; `broker`: `TestStatusBackendAndUI` | – |
| {{ ref("T28") }} | umgesetzt | `broker`: `TestT28GlobalLock`, `TestFailuresThatDoNotLock` (fehlerhafte Konfiguration eines Projekts sperrt nicht), `TestFaultGate`, `TestAuditFailureOnEachPathLocks`, `TestPanicLocksDelivery`; `fault`: `TestRaiseWithBrokenAuditLog`, `TestSelfCheck`, `TestConcurrentRaise`; `cmd/a38`: `TestServeGlobalLock` (echter Prozess: Admin-Socket, Agent-Socket mit `invalid_request`, Neustart bei bestehender Ursache), `TestFaultStatus`, `TestFaultClear` | – |

## Was die Lücken bedeuten

- {{ ref("T14") }} ist teilweise. Teil 1 ist automatisch und läuft im macOS-Workflow der CI (Swift-Tests, Go-Tests, `mise run app:e2e`). Teil 2 hat Stefan am 2026-11-12 am Bildschirm geprüft: echter Dialog von Touch ID, Sicht unter OmniWM, Tastaturfokus, Eingabetaste, Leertaste und Esc ({{ ref("G4") }}). Offen sind Vollbild, Wechsel des Arbeitsbereichs, Zeitablauf und die Meldungen ({{ ref("G33") }}).
- {{ ref("T15") }} läuft automatisch im macOS-Workflow mit dem Service Account der CI ([CI](55-ci.md)). Von Hand läuft er mit dem Token des Macs ([1Password-Backend einrichten](../how-to/30-1password-backend-einrichten.md#integrationstest)).
- {{ ref("T21") }} bleibt teilweise, bis die Freigabe-App die Leermarke für einen fehlenden Namen zeigt.
- {{ ref("T18") }} ist ein Fall der Release-Kette. Der Teil mit den Secrets lässt sich nur im Workflow prüfen.

Das Register trägt denselben Stand im Feld `umsetzung` jedes Testfalls und jedes Features.
`dg-register get <ID> umsetzung` zeigt ihn.
