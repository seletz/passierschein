// Decision line, exit codes, last barrier, options, start and notice.
import Foundation
import Testing
@testable import A38ApprovalCore

@Suite("Decision")
struct DecisionTests {
    func tree(_ line: String) throws -> [String: Any] {
        try #require(try JSONSerialization.jsonObject(with: Data(line.utf8)) as? [String: Any])
    }

    @Test("The approval line has the fields of the Go type Decision")
    func approvedLine() throws {
        let d = Decision(requestID: "req-a-0001", outcome: .approved, approvedCredentialIDs: ["c1"], ttlSeconds: 14400,
                         auth: AuthResult(policy: "biometrics", ok: true), elapsedMS: 8421)
        // The example line of the spike report and of TestDecisionJSON.
        #expect(encodeDecision(d) == #"{"approved_credential_ids":["c1"],"auth":{"ok":true,"policy":"biometrics"},"elapsed_ms":8421,"outcome":"approved","request_id":"req-a-0001","ttl_seconds":14400,"type":"approval_decision","version":1}"#)
    }

    @Test("A timeout line has an empty list and no lifetime")
    func timeoutLine() {
        let d = Decision(requestID: "req-a-0001", outcome: .timeout, approvedCredentialIDs: ["c1"], ttlSeconds: 60, elapsedMS: 5028)
        #expect(encodeDecision(d) == #"{"approved_credential_ids":[],"elapsed_ms":5028,"outcome":"timeout","request_id":"req-a-0001","type":"approval_decision","version":1}"#)
    }

    @Test("An error line without a request ID")
    func errorLine() throws {
        let line = encodeDecision(.failure(.invalidRequest, "type: the message type is not known", elapsedMS: 0))
        #expect(!line.contains("\n"))
        let t = try tree(line)
        #expect(t["request_id"] == nil)
        #expect(t["outcome"] as? String == "error")
        #expect((t["error"] as? [String: String])?["code"] == "invalid_request")
    }

    @Test("Exit code and outcome pair up", arguments: [
        (Outcome.approved, ErrorCode?.none, ExitCode.approved, Int32(0)),
        (.denied, nil, .denied, 10),
        (.timeout, nil, .timeout, 11),
        (.error, .authUnavailable, .authError, 12),
        (.error, .authFailed, .authError, 12),
        (.error, .invalidRequest, .invalidRequest, 65),
        (.error, .unsupportedVersion, .invalidRequest, 65),
        (.error, .internal, .internalError, 70),
    ])
    func exitCodes(outcome: Outcome, code: ErrorCode?, exit: ExitCode, raw: Int32) {
        let d = Decision(requestID: "r", outcome: outcome, error: code.map { DecisionError(code: $0, message: "m") }, elapsedMS: 0)
        #expect(ExitCode.of(d) == exit)
        #expect(exit.rawValue == raw)
    }

    static let request = Fixtures.request(credentials: Fixtures.credentials(2), ttl: 3600)

    static func approval(ids: [String] = ["c1"], ttl: Int64? = 3600, auth: AuthResult? = AuthResult(policy: "biometrics", ok: true),
                         requestID: String = "req-0001") -> Decision {
        Decision(requestID: requestID, outcome: .approved, approvedCredentialIDs: ids, ttlSeconds: ttl, auth: auth, elapsedMS: 1)
    }

    @Test("The last barrier passes a valid approval")
    func guardPasses() {
        let d = Self.approval()
        #expect(guardApproval(d, request: Self.request, testSwitches: false) == d)
    }

    @Test("The last barrier turns each invalid approval into an internal error", arguments: [
        ("no auth", approval(auth: nil)),
        ("auth not ok", approval(auth: AuthResult(policy: "biometrics", ok: false))),
        ("no entry", approval(ids: [])),
        ("unknown entry", approval(ids: ["c9"])),
        ("duplicate entry", approval(ids: ["c1", "c1"])),
        ("no lifetime", approval(ttl: nil)),
        ("zero lifetime", approval(ttl: 0)),
        ("longer lifetime", approval(ttl: 3601)),
        ("other request", approval(requestID: "req-0002")),
        ("policy skipped in a release build", approval(auth: AuthResult(policy: "skipped", ok: true))),
    ])
    func guardBlocks(name: String, decision: Decision) {
        let checked = guardApproval(decision, request: Self.request, testSwitches: false)
        #expect(checked.outcome == .error, "\(name)")
        #expect(checked.error?.code == .internal)
        #expect(ExitCode.of(checked) == .internalError)
    }

    @Test("A debug build passes the policy skipped; the daemon rejects it")
    func guardSkippedInDebug() {
        let d = Self.approval(auth: AuthResult(policy: "skipped", ok: true))
        #expect(guardApproval(d, request: Self.request, testSwitches: true) == d)
    }

    @Test("Durations", arguments: [(Int64(14400), "4 h"), (5400, "1 h 30 min"), (900, "15 min"), (45, "45 s"), (3661, "1 h 1 min 1 s")])
    func durations(seconds: Int64, text: String) {
        #expect(UIText.duration(seconds) == text)
    }
}

@Suite("Options and start")
struct OptionsTests {
    static let switches: [[String]] = [
        ["--request", "x.json"], ["--skip-auth"], ["--fake-auth", "success"], ["--auto-close", "5"], ["--no-window"],
        ["--demo-select", "all"], ["--demo-scroll-end"], ["--demo-arm-delay", "0"], ["--demo-confirm"], ["--debug-window"],
        ["--demo-appearance", "dark"],
    ]

    @Test("T14: a release build rejects each test switch", arguments: switches)
    func releaseRejects(args: [String]) {
        #expect(throws: UsageError.self) { try parseOptions(args, testSwitches: false) }
        let launch = prepare(arguments: args, testSwitches: false, readInput: { Data() })
        guard case let .exit(stdout, _, code) = launch else {
            Issue.record("the release build accepted \(args)")
            return
        }
        #expect(code == .usage)
        #expect(stdout?.contains(#""outcome":"error""#) == true)
    }

    @Test("Each test switch has a name in the list", arguments: switches)
    func switchNames(args: [String]) {
        #expect(TestSwitches.names.contains(args[0]))
    }

    @Test("A debug build reads the test switches")
    func debugReads() throws {
        let o = try parseOptions(["--skip-auth", "--auto-close", "2.5", "--demo-select", "c1,c2", "--no-window"], testSwitches: true)
        #expect(o.fakeAuth == .success)
        #expect(o.autoClose == 2.5)
        #expect(o.demoSelect == ["c1", "c2"])
        #expect(o.noWindow)
    }

    @Test("This test build has the test switches compiled in")
    func compiledIn() {
        #expect(TestSwitches.compiledIn)
    }

    @Test("Bad values are usage errors", arguments: [
        ["--auto-close"], ["--auto-close", "0"], ["--auto-close", "x"], ["--fake-auth", "maybe"], ["--nope"],
    ])
    func badValues(args: [String]) {
        #expect(throws: UsageError.self) { try parseOptions(args, testSwitches: true) }
    }

    @Test("--help and --version work in a release build and write nothing to the standard output")
    func helpInRelease() {
        for args in [["--help"], ["--version"]] {
            guard case let .exit(stdout, _, code) = prepare(arguments: args, testSwitches: false, readInput: { Data() }) else {
                Issue.record("no exit")
                continue
            }
            #expect(stdout == nil)
            #expect(code == .success)
        }
    }

    @Test("Without arguments the app reads the standard input (the daemon passes none)")
    func noArguments() throws {
        let data = try Fixtures.sampleData("first-request")
        guard case .approval(let r, let o) = prepare(arguments: [], testSwitches: false, readInput: { data }) else {
            Issue.record("no approval")
            return
        }
        #expect(r.requestID == "req-first-0001")
        #expect(o == Options())
    }

    @Test("An invalid request ends with 65 and an error decision, before a window exists")
    func invalidStart() throws {
        guard case let .exit(stdout, stderr, code) = prepare(arguments: [], readInput: { Data("{}".utf8) }) else {
            Issue.record("no exit")
            return
        }
        #expect(code == .invalidRequest)
        let line = try #require(stdout)
        #expect(line.contains(#""code":"invalid_request""#))
        #expect(!stderr.isEmpty)
    }

    @Test("An input larger than the limit ends with 65")
    func tooLargeStart() {
        guard case let .exit(_, _, code) = prepare(arguments: [], readInput: { nil }) else {
            Issue.record("no exit")
            return
        }
        #expect(code == .invalidRequest)
    }
}

@MainActor
@Suite("Notice")
struct NoticeTests {
    @Test("A daemon fault names what happened and the next steps")
    func daemonFault() throws {
        let m = NoticeViewModel(notice: try Fixtures.sampleNotice("notice-daemon-fault"), timeZone: TimeZone(identifier: "Europe/Berlin")!)
        #expect(m.title == UIText.noticeDaemonFaultTitle)
        #expect(m.detail == UIText.noticeDaemonFaultDetail)
        #expect(m.message.hasPrefix("The daemon cannot write the audit log"))
        #expect(m.nextSteps.joined(separator: " ").contains("a38 doctor"))
        #expect(m.nextSteps.joined(separator: " ").contains("a38 fault clear"))
        #expect(m.rows.map(\.label) == [UIText.noticeCode, UIText.noticeTime, UIText.noticeID])
        #expect(m.rows[0].value == "audit_failed")
        #expect(m.rows[1].value == "2026-11-12 09:41:07")
    }

    @Test("An invalid project names the project")
    func projectConfig() throws {
        let m = NoticeViewModel(notice: try Fixtures.sampleNotice("notice-project-config-invalid"))
        #expect(m.title == UIText.noticeProjectTitle)
        #expect(m.detail.contains("beispielprojekt"))
        #expect(m.rows.first == FactRow(label: UIText.noticeProject, value: "beispielprojekt"))
        #expect(m.nextSteps.joined(separator: " ").contains("a38 doctor"))
    }

    @Test("Close is the only action and runs once")
    func close() throws {
        let m = NoticeViewModel(notice: try Fixtures.sampleNotice("notice-daemon-fault"))
        var count = 0
        m.onClose = { count += 1 }
        m.close()
        m.close()
        #expect(count == 1)
        #expect(m.closed)
    }
}
