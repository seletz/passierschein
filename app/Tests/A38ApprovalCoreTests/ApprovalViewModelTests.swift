// The rules of the popup (D31, D7, D4, D25, D32), without a window.
import Foundation
import Testing
@testable import A38ApprovalCore

@MainActor
@Suite("Approval view model")
struct ApprovalViewModelTests {
    let clock = TestClock()

    func model(_ request: ApprovalRequest, auth: FakeAuthenticator = FakeAuthenticator(),
               timeoutOverride: TimeInterval? = nil) -> (ApprovalViewModel, Recorder) {
        let clock = self.clock
        let m = ApprovalViewModel(request: request, authenticator: auth, startedAt: clock.now,
                                  clock: { clock.now }, timeoutOverride: timeoutOverride,
                                  timeZone: TimeZone(identifier: "Europe/Berlin")!)
        let recorder = Recorder()
        m.onFinish = { recorder.decisions.append($0) }
        return (m, recorder)
    }

    final class Recorder {
        var decisions: [Decision] = []
    }

    func armed(_ m: ApprovalViewModel) {
        clock.advance(ApprovalViewModel.armDelay)
        m.tick()
    }

    // MARK: Case a: first request

    @Test("T14 part 2 (automatic part): no entry is selected at the start")
    func noPreselection() throws {
        for name in ["first-request", "bundle", "deviation", "long-claims"] {
            let (m, _) = model(try Fixtures.sampleRequest(name))
            #expect(m.selected.isEmpty, "\(name)")
            #expect(!m.canConfirm)
            #expect(m.confirmTitle == UIText.confirmWaiting)
        }
    }

    @Test("First request: blue case, no scroll, no Select all")
    func firstRequest() throws {
        let (m, _) = model(try Fixtures.sampleRequest("first-request"))
        #expect(m.popupCase == .firstRequest)
        #expect(!m.listScrolls)
        #expect(m.groups.count == 1)
        #expect(m.groups.allSatisfy { !$0.offersSelectAll })
    }

    @Test("D31: the confirm action is active only after 1.5 s and with a selection")
    func delay() {
        let (m, _) = model(Fixtures.request(credentials: Fixtures.credentials(2)))
        m.setSelected("c1", true)
        #expect(!m.canConfirm, "no delay passed")
        clock.advance(1.4)
        m.tick()
        #expect(!m.armed)
        #expect(!m.canConfirm)
        clock.advance(0.1)
        m.tick()
        #expect(m.armed)
        #expect(m.canConfirm)
        #expect(m.confirmTitle == UIText.confirm(1))
        m.setSelected("c1", false)
        #expect(!m.canConfirm, "no selection")
        #expect(m.confirmTitle == UIText.confirmSelect)
    }

    @Test("D31: a confirm before the delay does nothing")
    func confirmTooEarly() async {
        let auth = FakeAuthenticator()
        let (m, rec) = model(Fixtures.request(credentials: Fixtures.credentials(2)), auth: auth)
        m.setSelected("c1", true)
        await m.confirm()
        #expect(auth.calls.isEmpty)
        #expect(rec.decisions.isEmpty)
    }

    @Test("A confirm without a selection does nothing")
    func confirmWithoutSelection() async {
        let auth = FakeAuthenticator()
        let (m, rec) = model(Fixtures.request(credentials: Fixtures.credentials(2)), auth: auth)
        armed(m)
        await m.confirm()
        #expect(auth.calls.isEmpty)
        #expect(rec.decisions.isEmpty)
    }

    @Test("An unknown ID does not change the selection")
    func unknownID() {
        let (m, _) = model(Fixtures.request(credentials: Fixtures.credentials(2)))
        m.setSelected("c99", true)
        #expect(m.selected.isEmpty)
    }

    // MARK: Case b: bundle

    @Test("Bundle: the list scrolls above six entries and groups by vault in request order")
    func bundle() throws {
        let (m, _) = model(try Fixtures.sampleRequest("bundle"))
        #expect(m.listScrolls)
        #expect(m.groups.map(\.vault) == ["beispiel.app.k8s-dev", "beispiel.infra.k8s-dev"])
        #expect(m.groups.map { $0.items.count } == [7, 3])
        #expect(m.groups[0].items.map(\.id) == ["c1", "c2", "c3", "c4", "c5", "c6", "c7"])
        #expect(m.selectedCountText == "0 of 10 selected")
    }

    @Test("Select all exists per vault only with more than three entries")
    func selectAllRule() throws {
        let (m, _) = model(try Fixtures.sampleRequest("bundle"))
        #expect(m.groups[0].offersSelectAll)
        #expect(!m.groups[1].offersSelectAll, "three entries are not more than three")
        m.selectAll(vault: "beispiel.infra.k8s-dev")
        #expect(m.selected.isEmpty, "no Select all for this vault")
        m.selectAll(vault: "beispiel.app.k8s-dev")
        #expect(m.selected == Set(["c1", "c2", "c3", "c4", "c5", "c6", "c7"]), "only the entries of this vault")
        #expect(m.selectedCountText == "7 of 10 selected")
    }

    @Test("Six entries do not scroll, seven do")
    func scrollThreshold() {
        #expect(!model(Fixtures.request(credentials: Fixtures.credentials(6))).0.listScrolls)
        #expect(model(Fixtures.request(credentials: Fixtures.credentials(7))).0.listScrolls)
    }

    // MARK: Case c: deviation

    @Test("Deviation: own case, the facts name the existing grant")
    func deviation() throws {
        let (m, _) = model(try Fixtures.sampleRequest("deviation"))
        #expect(m.popupCase == .deviation)
        let row = try #require(m.verifiedRows.first { $0.label == UIText.factActiveGrant })
        #expect(row.value == "grant-0007: 2 credentials in beispiel.app.k8s-dev, valid until 2026-11-12 16:30")
        #expect(m.request.credentials.map(\.deviation) == [.newItem, .newVault])
    }

    @Test("A first request has no row for an active grant")
    func noActiveGrantRow() throws {
        let (m, _) = model(try Fixtures.sampleRequest("first-request"))
        #expect(!m.verifiedRows.contains { $0.label == UIText.factActiveGrant })
    }

    // MARK: Facts and claims (D4, D25)

    @Test("D4: the verified facts contain no claim; the session name is a claim only")
    func factsAndClaimsApart() throws {
        let (m, _) = model(try Fixtures.sampleRequest("first-request"))
        let facts = m.verifiedRows.map(\.value).joined(separator: "\n")
        let claims = m.request.claims
        for claim in [claims.sessionName, claims.agent, claims.task, claims.reason].compactMap({ $0 }) {
            #expect(!facts.contains(claim), "the facts contain the claim \(claim)")
        }
        #expect(m.verifiedRows.map(\.label) == [UIText.factProject, UIText.factSession, UIText.factOrigin,
                                                UIText.factProcess, UIText.factDirectory])
        #expect(m.verifiedRows[1].value == "s-7f3a9c21", "the session ID of the daemon is a fact")
        #expect(m.claimRows.map(\.label) == [UIText.claimSession, UIText.claimAgent, UIText.claimTask, UIText.claimReason])
        #expect(m.claimRows[0].value == "beispiel-koordinator")
    }

    @Test("D25: the session name is never cut; the other claims are cut by the view")
    func sessionNameUnabridged() throws {
        let (m, _) = model(try Fixtures.sampleRequest("long-claims"))
        #expect(m.claimRows[0].unabridged)
        #expect(m.claimRows[0].value == "beispiel-koordinator (Feature: Migration der Datenbank Teil 2)")
        #expect(m.claimRows.dropFirst().allSatisfy { !$0.unabridged })
    }

    @Test("A missing claim shows a mark, never an invented value")
    func missingClaims() {
        let (m, _) = model(Fixtures.request(credentials: Fixtures.credentials(1), claims: Claims()))
        #expect(m.claimRows.allSatisfy { $0.missing && $0.value == UIText.claimMissing })
    }

    @Test("A control or bidi character of a claim becomes visible")
    func cleanClaims() {
        let (m, _) = model(Fixtures.request(credentials: Fixtures.credentials(1),
                                            claims: Claims(agent: "agent\u{202E}txt.exe", task: "line 1\nline 2")))
        #expect(m.claimRows[1].value == "agent\u{FFFD}txt.exe")
        #expect(m.claimRows[2].value == "line 1 line 2")
        #expect(m.verifiedRows[0].value == "beispielprojekt")
    }

    @Test("The Touch ID reason names count and project, never a claim")
    func authReason() async throws {
        let auth = FakeAuthenticator()
        let (m, _) = model(try Fixtures.sampleRequest("first-request"), auth: auth)
        armed(m)
        m.setSelected("c1", true)
        m.setSelected("c2", true)
        await m.confirm()
        #expect(auth.calls == [UIText.authReason(count: 2, project: "beispielprojekt")])
    }

    @Test("The Touch ID texts follow the language of the system")
    func authTextsFollowSystemLanguage() {
        #expect(UIText.authReason(count: 2, project: "beispielprojekt", language: .english)
            == "approve 2 credentials for project beispielprojekt")
        #expect(UIText.authReason(count: 1, project: "beispielprojekt", language: .english)
            == "approve 1 credential for project beispielprojekt")
        #expect(UIText.authReason(count: 2, project: "beispielprojekt", language: .german)
            == "2 Zugangsdaten für das Projekt beispielprojekt freigeben")
        #expect(UIText.authReason(count: 1, project: "beispielprojekt", language: .german)
            == "1 Zugangsdatum für das Projekt beispielprojekt freigeben")
        #expect(UIText.authCancelTitle(language: .german) == "Zurück")
        #expect(UIText.authCancelTitle(language: .english) == "Back")
        #expect(SystemLanguage.from(preferredLanguages: ["de-DE", "en-US"]) == .german)
        #expect(SystemLanguage.from(preferredLanguages: ["de"]) == .german)
        #expect(SystemLanguage.from(preferredLanguages: ["en-DE", "de-DE"]) == .english)
        #expect(SystemLanguage.from(preferredLanguages: ["fr-FR"]) == .english)
        #expect(SystemLanguage.from(preferredLanguages: []) == .english)
    }

    // MARK: Lifetime (D32)

    nonisolated static let lifetimeCases: [(Int64, [Int64])] = [
        (14400, [14400, 7200, 900]),
        (3600, [3600, 1800, 900]),
        (1800, [1800, 900]),
        (1200, [1200, 900, 600]),
        (600, [600, 300]),
        (90, [90]),
        (30, [30]),
    ]

    @Test("The lifetime choices never exceed the proposal", arguments: lifetimeCases)
    func lifetimeChoicesSet(proposal: Int64, expected: [Int64]) {
        let choices = lifetimeChoices(proposal: proposal)
        #expect(choices == expected)
        #expect(choices.allSatisfy { $0 <= proposal })
        #expect(choices.first == proposal)
    }

    @Test("The proposal is the start value; only a value of the set is possible")
    func selectLifetime() async {
        let auth = FakeAuthenticator()
        let (m, rec) = model(Fixtures.request(credentials: Fixtures.credentials(1)), auth: auth)
        #expect(m.lifetime == 14400)
        #expect(m.lifetimeText == "4 h (proposed)")
        m.selectLifetime(28800)
        #expect(m.lifetime == 14400, "longer than the proposal")
        m.selectLifetime(1234)
        #expect(m.lifetime == 14400, "not in the set")
        m.openLifetimeChoice()
        #expect(m.lifetimeChoiceOpen)
        m.selectLifetime(900)
        #expect(m.lifetimeText == "15 min")
        armed(m)
        m.setSelected("c1", true)
        await m.confirm()
        #expect(rec.decisions.first?.ttlSeconds == 900)
        #expect(!m.lifetimeChoiceOpen)
    }

    // MARK: Decisions

    @Test("Approval: the selected IDs in request order, the lifetime and the policy")
    func approve() async throws {
        let auth = FakeAuthenticator()
        let (m, rec) = model(try Fixtures.sampleRequest("bundle"), auth: auth)
        armed(m)
        m.setSelected("c9", true)
        m.setSelected("c2", true)
        clock.advance(2)
        await m.confirm()
        let d = try #require(rec.decisions.first)
        #expect(rec.decisions.count == 1)
        #expect(d.outcome == .approved)
        #expect(d.requestID == "req-bundle-0001")
        #expect(d.approvedCredentialIDs == ["c2", "c9"])
        #expect(d.ttlSeconds == 14400)
        #expect(d.auth == AuthResult(policy: skippedPolicy, ok: true))
        #expect(d.elapsedMS == 3500)
    }

    @Test("A stopped Touch ID dialog keeps the popup open with a message")
    func authCancelled() async {
        let auth = FakeAuthenticator(outcome: .cancelled)
        let (m, rec) = model(Fixtures.request(credentials: Fixtures.credentials(1)), auth: auth)
        armed(m)
        m.setSelected("c1", true)
        await m.confirm()
        #expect(rec.decisions.isEmpty)
        #expect(m.message == UIText.authStopped)
        #expect(m.canConfirm, "the human can try again")
        auth.outcome = .success
        await m.confirm()
        #expect(rec.decisions.first?.outcome == .approved)
    }

    @Test("Touch ID not available or failed: error with exit code 12, never approved", arguments: [
        (AuthOutcome.unavailable("no biometry"), ErrorCode.authUnavailable),
        (AuthOutcome.failed("no match"), ErrorCode.authFailed),
    ])
    func authError(outcome: AuthOutcome, code: ErrorCode) async throws {
        let (m, rec) = model(Fixtures.request(credentials: Fixtures.credentials(1)), auth: FakeAuthenticator(outcome: outcome))
        armed(m)
        m.setSelected("c1", true)
        await m.confirm()
        let d = try #require(rec.decisions.first)
        #expect(d.outcome == .error)
        #expect(d.error?.code == code)
        #expect(d.approvedCredentialIDs.isEmpty)
        #expect(d.auth == AuthResult(policy: skippedPolicy, ok: false))
        #expect(ExitCode.of(d) == .authError)
    }

    @Test("Deny needs no delay")
    func deny() throws {
        let (m, rec) = model(Fixtures.request(credentials: Fixtures.credentials(1)))
        m.setSelected("c1", true)
        m.deny()
        let d = try #require(rec.decisions.first)
        #expect(d.outcome == .denied)
        #expect(d.approvedCredentialIDs.isEmpty)
        #expect(d.auth == nil)
        m.deny()
        m.tick()
        #expect(rec.decisions.count == 1, "one decision only")
    }

    @Test("T14 part 1: without a confirmation the popup ends with timeout")
    func timeout() throws {
        let (m, rec) = model(Fixtures.request(credentials: Fixtures.credentials(1), timeout: 120))
        m.setSelected("c1", true)
        clock.advance(60)
        m.tick()
        #expect(m.secondsLeft == 60)
        #expect(rec.decisions.isEmpty)
        clock.advance(60)
        m.tick()
        let d = try #require(rec.decisions.first)
        #expect(d.outcome == .timeout)
        #expect(d.approvedCredentialIDs.isEmpty)
        #expect(ExitCode.of(d) == .timeout)
    }

    @Test("The test switch --auto-close shortens the timeout, never extends it")
    func timeoutOverride() {
        #expect(model(Fixtures.request(credentials: Fixtures.credentials(1)), timeoutOverride: 5).0.timeout == 5)
        #expect(model(Fixtures.request(credentials: Fixtures.credentials(1)), timeoutOverride: 500).0.timeout == 120)
    }

    @Test("The timeout during Touch ID stops the dialog and ends with timeout")
    func timeoutDuringAuth() async throws {
        let auth = FakeAuthenticator(hold: true)
        let (m, rec) = model(Fixtures.request(credentials: Fixtures.credentials(2)), auth: auth)
        armed(m)
        m.setSelected("c1", true)
        let task = Task { await m.confirm() }
        while !auth.isWaiting { await Task.yield() }
        #expect(m.authInProgress)
        #expect(!m.canEditSelection, "the selection is fixed during Touch ID")
        m.setSelected("c2", true)
        #expect(m.selected == ["c1"])
        clock.advance(200)
        m.tick()
        await task.value
        #expect(auth.cancelCount == 1)
        #expect(rec.decisions.map(\.outcome) == [.timeout])
    }

    @Test("A late success after the timeout gives no approval")
    func lateSuccess() async throws {
        let auth = FakeAuthenticator(hold: true)
        let (m, rec) = model(Fixtures.request(credentials: Fixtures.credentials(1)), auth: auth)
        armed(m)
        m.setSelected("c1", true)
        let task = Task { await m.confirm() }
        while !auth.isWaiting { await Task.yield() }
        // The dialog ignores cancel and reports success later.
        auth.hold = false
        clock.advance(200)
        m.tick()
        auth.resume(with: .success)
        await task.value
        #expect(rec.decisions.map(\.outcome) == [.timeout])
    }

    @Test("Deny during Touch ID stops the dialog")
    func denyDuringAuth() async throws {
        let auth = FakeAuthenticator(hold: true)
        let (m, rec) = model(Fixtures.request(credentials: Fixtures.credentials(1)), auth: auth)
        armed(m)
        m.setSelected("c1", true)
        let task = Task { await m.confirm() }
        while !auth.isWaiting { await Task.yield() }
        m.deny()
        await task.value
        #expect(auth.cancelCount == 1)
        #expect(rec.decisions.map(\.outcome) == [.denied])
    }
}
