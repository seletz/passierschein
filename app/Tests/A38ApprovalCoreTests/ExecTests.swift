// The approval request of the exec way (D65, F23): decoding, the state of
// the window, the last barrier and the display of the command.
import Foundation
import Testing
@testable import A38ApprovalCore

/// A valid exec request (test, grant) as a JSON tree; tests change it.
func execTree() -> [String: Any] {
    var tree = Fixtures.requestTree()
    tree["credentials"] = [
        ["id": "c1", "vault": "beispiel.app.k8s-test", "title": "datenbank-test", "field": "password", "environment": "test"],
    ]
    tree["grant"] = [
        "ttl_seconds": 1800,
        "scope": ["project": "beispielprojekt", "vaults": ["beispiel.app.k8s-test"]] as [String: Any],
    ] as [String: Any]
    tree["exec"] = [
        "environment": "test",
        "program": "/opt/homebrew/bin/psql",
        "args": ["-h", "datenbank-test.beispiel.internal", "-c", "select count(*) from auftrag"],
        "cwd": "/Users/beispiel/develop/beispielprojekt",
        "env": [["name": "PGPASSWORD", "credential_id": "c1"]],
        "approval": ["kind": "grant", "binding": "program", "lifetime_choices_seconds": [1800, 900, 300]] as [String: Any],
    ] as [String: Any]
    return tree
}

/// The exec tree changed to prod with "only this command".
func prodTree() -> [String: Any] {
    var tree = execTree()
    setPath(&tree, ["credentials"][...], [
        ["id": "c1", "vault": "beispiel.app.k8s-prod", "title": "datenbank-prod", "field": "password", "environment": "prod"],
    ])
    setPath(&tree, ["grant", "ttl_seconds"][...], 0)
    setPath(&tree, ["exec", "environment"][...], "prod")
    setPath(&tree, ["exec", "approval"][...], ["kind": "once", "binding": "command"])
    return tree
}

/// The exec tree in the environment with "only this command" (lifetime 0,
/// D76).
func onceTree(_ environment: String) -> [String: Any] {
    var tree = execTree()
    setPath(&tree, ["credentials"][...], [
        ["id": "c1", "vault": "beispiel.app.k8s-" + environment, "title": "datenbank-" + environment,
         "field": "password", "environment": environment],
    ])
    setPath(&tree, ["grant", "ttl_seconds"][...], 0)
    setPath(&tree, ["exec", "environment"][...], environment)
    setPath(&tree, ["exec", "approval"][...], ["kind": "once", "binding": "command"])
    return tree
}

@Suite("Exec request: decoding")
struct ExecDecodingTests {
    @Test("T38 part (window): the exec samples decode with environment, command and variables",
          arguments: [("exec-dev", "dev"), ("exec-test", "test"), ("exec-prod", "prod"), ("exec-long", "test"), ("exec-dev-once", "dev")])
    func samples(name: String, environment: String) throws {
        let r = try Fixtures.sampleRequest(name)
        let exec = try #require(r.exec)
        #expect(exec.environment.rawValue == environment)
        #expect(exec.program.hasPrefix("/"))
        #expect(exec.cwd == "/Users/beispiel/develop/beispielprojekt")
        #expect(!exec.env.isEmpty)
        #expect(r.credentials.allSatisfy { $0.environment == environment })
        switch (environment, name) {
        case ("prod", _), (_, "exec-dev-once"):
            #expect(exec.approval.kind == .once)
            #expect(exec.approval.lifetimeChoices.isEmpty)
            #expect(r.grant.ttlSeconds == 0)
        default:
            #expect(exec.approval.kind == .grant)
            #expect(exec.approval.lifetimeChoices.first == r.grant.ttlSeconds)
        }
    }

    @Test("T38 part (window): a full exec request keeps each field")
    func fullExec() throws {
        guard case .request(let r) = try decodeMessage(Fixtures.json(execTree())) else {
            Issue.record("not a request")
            return
        }
        let exec = try #require(r.exec)
        #expect(exec == ExecCommand(
            environment: .test, program: "/opt/homebrew/bin/psql",
            args: ["-h", "datenbank-test.beispiel.internal", "-c", "select count(*) from auftrag"],
            cwd: "/Users/beispiel/develop/beispielprojekt",
            env: [ExecVariable(name: "PGPASSWORD", credentialID: "c1")],
            approval: ExecApproval(kind: .grant, binding: .program, lifetimeChoices: [1800, 900, 300])))
    }

    @Test("T38 part (window): a prod request with \"only this command\" decodes")
    func prod() throws {
        guard case .request(let r) = try decodeMessage(Fixtures.json(prodTree())) else {
            Issue.record("not a request")
            return
        }
        #expect(r.exec?.approval == ExecApproval(kind: .once, binding: .command, lifetimeChoices: []))
    }

    @Test("A plain request keeps its rules: no test, no prod, a lifetime")
    func plainUnchanged() {
        var tree = Fixtures.requestTree()
        setPath(&tree, ["grant", "ttl_seconds"][...], 0)
        #expect(throws: ProtocolError.self) { try decodeMessage(Fixtures.json(tree)) }
    }

    static func change(_ path: [String], _ value: Any?) -> @Sendable (inout [String: Any]) -> Void {
        DecodingTests.set(path, value)
    }

    static let invalid: [DecodingTests.Invalid] = [
        .init(name: "exec not an object", code: .invalidRequest, change: change(["exec"], "psql")),
        .init(name: "unknown key in exec", code: .invalidRequest, change: change(["exec", "shell"], "sh")),
        .init(name: "environment missing", code: .invalidRequest, change: change(["exec", "environment"], nil)),
        .init(name: "environment unknown", code: .invalidRequest, change: change(["exec", "environment"], "stage")),
        .init(name: "environment not the strictest", code: .invalidRequest, change: change(["exec", "environment"], "dev")),
        .init(name: "program relative", code: .invalidRequest, change: change(["exec", "program"], "psql")),
        .init(name: "program not clean", code: .invalidRequest, change: change(["exec", "program"], "/opt/../psql")),
        .init(name: "program with a control character", code: .invalidRequest, change: change(["exec", "program"], "/opt/ps\u{1b}ql")),
        .init(name: "cwd relative", code: .invalidRequest, change: change(["exec", "cwd"], "beispielprojekt")),
        .init(name: "args not a list", code: .invalidRequest, change: change(["exec", "args"], "-h")),
        .init(name: "arg not a string", code: .invalidRequest, change: change(["exec", "args"], [1])),
        .init(name: "arg with escape", code: .invalidRequest, change: change(["exec", "args"], ["a\u{1b}[2Jb"])),
        .init(name: "arg with bidi override", code: .invalidRequest, change: change(["exec", "args"], ["a\u{202E}b"])),
        .init(name: "too many args", code: .invalidRequest, change: change(["exec", "args"], Array(repeating: "x", count: 257))),
        .init(name: "args too long", code: .invalidRequest, change: change(["exec", "args"], [String(repeating: "x", count: 16 * 1024 + 1)])),
        .init(name: "no variable", code: .invalidRequest, change: change(["exec", "env"], [Any]())),
        .init(name: "forbidden name", code: .invalidRequest, change: change(["exec", "env"], [["name": "PATH", "credential_id": "c1"]])),
        .init(name: "forbidden prefix", code: .invalidRequest, change: change(["exec", "env"], [["name": "DYLD_X", "credential_id": "c1"]])),
        .init(name: "name not upper case", code: .invalidRequest, change: change(["exec", "env"], [["name": "pgpassword", "credential_id": "c1"]])),
        .init(name: "name twice", code: .invalidRequest, change: change(["exec", "env"], [
            ["name": "PGPASSWORD", "credential_id": "c1"], ["name": "PGPASSWORD", "credential_id": "c1"]])),
        .init(name: "unknown credential", code: .invalidRequest, change: change(["exec", "env"], [["name": "PGPASSWORD", "credential_id": "c9"]])),
        .init(name: "credential without variable", code: .invalidRequest, change: change(["credentials"], [
            ["id": "c1", "vault": "beispiel.app.k8s-test", "title": "datenbank-test", "field": "password", "environment": "test"],
            ["id": "c2", "vault": "beispiel.app.k8s-test", "title": "cache-test", "field": "password", "environment": "test"]])),
        .init(name: "kind unknown", code: .invalidRequest, change: change(["exec", "approval", "kind"], "always")),
        .init(name: "binding unknown", code: .invalidRequest, change: change(["exec", "approval", "binding"], "argument")),
        .init(name: "test with binding none", code: .invalidRequest, change: change(["exec", "approval", "binding"], "none")),
        .init(name: "grant without lifetime", code: .invalidRequest, change: change(["grant", "ttl_seconds"], 0)),
        .init(name: "no choices", code: .invalidRequest, change: change(["exec", "approval", "lifetime_choices_seconds"], nil)),
        .init(name: "first choice is not the lifetime", code: .invalidRequest,
              change: change(["exec", "approval", "lifetime_choices_seconds"], [900, 300])),
        .init(name: "choice longer than the lifetime", code: .invalidRequest,
              change: change(["exec", "approval", "lifetime_choices_seconds"], [1800, 3600])),
        .init(name: "choice twice", code: .invalidRequest,
              change: change(["exec", "approval", "lifetime_choices_seconds"], [1800, 900, 900])),
        .init(name: "choice zero", code: .invalidRequest,
              change: change(["exec", "approval", "lifetime_choices_seconds"], [1800, 0])),
    ]

    @Test("T38 part (window): each invalid exec request ends before a window exists", arguments: invalid)
    func rejects(_ c: DecodingTests.Invalid) {
        var tree = execTree()
        c.change(&tree)
        do {
            _ = try decodeMessage(Fixtures.json(tree))
            Issue.record("accepted: \(c.name)")
        } catch {
            #expect(error.code == c.code, "\(c.name)")
            #expect(error.requestID == "req-0001", "\(c.name)")
        }
    }

    static let invalidProd: [DecodingTests.Invalid] = [
        .init(name: "prod with grant", code: .invalidRequest, change: { tree in
            setPath(&tree, ["grant", "ttl_seconds"][...], 300)
            setPath(&tree, ["exec", "approval"][...], ["kind": "grant", "binding": "command", "lifetime_choices_seconds": [300]] as [String: Any])
        }),
        .init(name: "once with lifetime", code: .invalidRequest, change: change(["grant", "ttl_seconds"], 60)),
        .init(name: "once with choices", code: .invalidRequest, change: change(["exec", "approval", "lifetime_choices_seconds"], [60])),
        .init(name: "once with binding program", code: .invalidRequest, change: change(["exec", "approval", "binding"], "program")),
    ]

    @Test("T36 part (window, D76): with the lifetime 0, dev and test use \"only this command\"",
          arguments: ["dev", "test"])
    func onceOutsideProd(environment: String) throws {
        let r = try decodeMessage(Fixtures.json(onceTree(environment)))
        guard case let .request(req) = r else {
            Issue.record("not a request")
            return
        }
        #expect(req.exec?.approval == ExecApproval(kind: .once, binding: .command, lifetimeChoices: []))
        #expect(req.grant.ttlSeconds == 0)
    }

    @Test("T36 part (window, D76): \"only this command\" in test keeps its rules", arguments: invalidProd.dropFirst())
    func rejectsOnceInTest(_ c: DecodingTests.Invalid) {
        var tree = onceTree("test")
        c.change(&tree)
        #expect(throws: ProtocolError.self, "\(c.name)") { try decodeMessage(Fixtures.json(tree)) }
    }

    @Test("T38 part (window): prod allows only \"only this command\"", arguments: invalidProd)
    func rejectsProd(_ c: DecodingTests.Invalid) {
        var tree = prodTree()
        c.change(&tree)
        #expect(throws: ProtocolError.self, "\(c.name)") { try decodeMessage(Fixtures.json(tree)) }
    }

    @Test("An error message names the field, never a value of the command")
    func errorHidesContent() {
        var tree = execTree()
        setPath(&tree, ["exec", "args"][...], ["geheim-beispiel\u{1b}"])
        do {
            _ = try decodeMessage(Fixtures.json(tree))
            Issue.record("accepted")
        } catch {
            #expect(!error.message.contains("geheim"))
            #expect(error.message.contains("exec.args[0]"))
        }
    }
}

@MainActor
@Suite("Exec request: window state")
struct ExecViewModelTests {
    let clock = TestClock()

    func model(_ name: String, auth: FakeAuthenticator = FakeAuthenticator()) throws -> (ApprovalViewModel, [Decision]) {
        let clock = self.clock
        let m = ApprovalViewModel(request: try Fixtures.sampleRequest(name), authenticator: auth,
                                  startedAt: clock.now, clock: { clock.now },
                                  timeZone: TimeZone(identifier: "Europe/Berlin")!)
        return (m, [])
    }

    func arm(_ m: ApprovalViewModel) {
        clock.advance(ApprovalViewModel.armDelay)
        m.tick()
    }

    @Test("T38 part (window): the popup case names the environment", arguments: [
        ("exec-dev", ExecEnvironment.dev), ("exec-test", .test), ("exec-prod", .prod),
    ])
    func popupCase(name: String, env: ExecEnvironment) throws {
        let (m, _) = try model(name)
        #expect(m.popupCase == .command(env))
        #expect(m.environmentHighlighted == (env != .dev))
    }

    @Test("A plain request keeps its popup case")
    func plainCase() throws {
        let (m, _) = try model("first-request")
        #expect(m.popupCase == .firstRequest)
        #expect(m.commandRows.isEmpty)
        #expect(m.variableRows.isEmpty)
    }

    @Test("T38 part (window): the command rows show program, arguments and directory in full")
    func commandRows() throws {
        let (m, _) = try model("exec-test")
        #expect(m.commandRows.map(\.label) == [UIText.commandProgram, UIText.commandArguments, UIText.commandDirectory])
        #expect(m.commandRows[0].value == "/opt/homebrew/bin/psql")
        #expect(m.commandRows[1].value == "-h datenbank-test.beispiel.internal -U beispiel -c 'select count(*) from auftrag'")
        #expect(m.commandRows[2].value == "/Users/beispiel/develop/beispielprojekt")
        #expect(!m.commandScrolls)
    }

    @Test("T38 part (window): a long command scrolls and loses no character")
    func longCommand() throws {
        let (m, _) = try model("exec-long")
        #expect(m.commandScrolls)
        let request = try Fixtures.sampleRequest("exec-long")
        let shown = m.commandRows[1].value
        for arg in try #require(request.exec).args where !arg.contains("\n") && !arg.contains("'") {
            #expect(shown.contains(arg))
        }
        #expect(shown.contains(#"\n"#), "a line break is visible")
    }

    @Test("T38 part (window): each variable names the credential with the names of the backend")
    func variables() throws {
        let (m, _) = try model("exec-prod")
        #expect(m.variableRows.map(\.names) == [["DB_USER"], ["DB_PASSWORD"]])
        #expect(m.variableRows.map(\.credential.id) == ["c1", "c2"])
        #expect(m.variableRows[0].reference == "beispiel.app.k8s-prod › datenbank-prod › username")
    }

    @Test("Two names for one credential share one checkbox")
    func sharedCredential() throws {
        var tree = execTree()
        setPath(&tree, ["exec", "env"][...], [
            ["name": "PGPASSWORD", "credential_id": "c1"], ["name": "PGPASS_COPY", "credential_id": "c1"]])
        guard case .request(let r) = try decodeMessage(Fixtures.json(tree)) else { return }
        let m = ApprovalViewModel(request: r, authenticator: FakeAuthenticator(), startedAt: 0, clock: { 0 })
        #expect(m.variableRows.map(\.names) == [["PGPASSWORD", "PGPASS_COPY"]])
    }

    @Test("T38 part (window): the offered lifetimes come from the request, never longer than configured")
    func lifetimeChoices() throws {
        let (m, _) = try model("exec-test")
        #expect(m.lifetimeChoices == [1800, 900, 300])
        #expect(m.lifetime == 1800)
        #expect(m.offersLifetimeChoice)
        m.selectLifetime(3600)
        #expect(m.lifetime == 1800, "a longer lifetime is not possible")
        m.selectLifetime(300)
        #expect(m.lifetime == 300)
        #expect(m.bindingText == UIText.bindingProgram)
        #expect(m.lifetimeChoices == A38ApprovalCore.lifetimeChoices(proposal: 1800), "one rule for both windows")
    }

    @Test("T38 part (window): prod offers no lifetime and says \"only this command\"")
    func prodOnce() throws {
        let (m, _) = try model("exec-prod")
        #expect(m.lifetimeChoices.isEmpty)
        #expect(!m.offersLifetimeChoice)
        #expect(m.approvalText == UIText.approvalOnce)
    }

    @Test("D31: no variable is selected; confirm needs every variable")
    func confirmNeedsAll() throws {
        let (m, _) = try model("exec-prod")
        #expect(m.selected.isEmpty)
        arm(m)
        m.setSelected("c1", true)
        #expect(!m.canConfirm)
        #expect(m.confirmTitle == UIText.confirmSelectAll)
        m.setSelected("c2", true)
        #expect(m.canConfirm)
        #expect(m.confirmTitle == UIText.confirmCommand)
    }

    @Test("T38 part (window): prod approves with lifetime 0 (no ttl_seconds on the wire)")
    func prodDecision() async throws {
        let (m, _) = try model("exec-prod")
        var got: Decision?
        m.onFinish = { got = $0 }
        arm(m)
        m.setSelected("c1", true)
        m.setSelected("c2", true)
        await m.confirm()
        let d = try #require(got)
        #expect(d.outcome == .approved)
        #expect(d.ttlSeconds == nil)
        #expect(!encodeDecision(d).contains("ttl_seconds"))
        #expect(guardApproval(d, request: m.request).outcome == .approved)
    }

    @Test("T38 part (window): test approves with the selected lifetime")
    func testDecision() async throws {
        let (m, _) = try model("exec-test")
        var got: Decision?
        m.onFinish = { got = $0 }
        arm(m)
        m.setSelected("c1", true)
        m.selectLifetime(900)
        await m.confirm()
        #expect(got?.ttlSeconds == 900)
        #expect(got?.approvedCredentialIDs == ["c1"])
    }

    @Test("The Touch ID reason names the environment and verified facts only")
    func authReason() {
        let text = UIText.authReasonCommand(count: 1, environment: .prod, project: "beispielprojekt", language: .english)
        #expect(text.contains("prod"))
        #expect(text.contains("beispielprojekt"))
    }
}

@Suite("Exec request: last barrier")
struct ExecGuardTests {
    static func request(_ tree: [String: Any]) -> ApprovalRequest {
        guard case .request(let r) = try! decodeMessage(Fixtures.json(tree)) else { fatalError() }
        return r
    }

    static func approval(_ ids: [String] = ["c1"], ttl: Int64?) -> Decision {
        Decision(requestID: "req-0001", outcome: .approved, approvedCredentialIDs: ids, ttlSeconds: ttl,
                 auth: AuthResult(policy: "biometrics", ok: true), elapsedMS: 1)
    }

    @Test("T38 part (window): a grant needs an offered lifetime", arguments: [
        (Int64?(1800), true), (900, true), (300, true), (600, false), (3600, false), (0, false), (nil, false),
    ])
    func grant(ttl: Int64?, passes: Bool) {
        let checked = guardApproval(Self.approval(ttl: ttl), request: Self.request(execTree()), testSwitches: false)
        #expect((checked.outcome == .approved) == passes)
    }

    @Test("T38 part (window): \"only this command\" has no lifetime", arguments: [
        (Int64?(nil), true), (0, true), (60, false),
    ])
    func once(ttl: Int64?, passes: Bool) {
        let checked = guardApproval(Self.approval(ttl: ttl), request: Self.request(prodTree()), testSwitches: false)
        #expect((checked.outcome == .approved) == passes)
    }

    @Test("T36 part (window, D76): \"only this command\" in dev has no lifetime", arguments: [
        (Int64?(nil), true), (0, true), (60, false),
    ])
    func onceInDev(ttl: Int64?, passes: Bool) {
        let checked = guardApproval(Self.approval(ttl: ttl), request: Self.request(onceTree("dev")), testSwitches: false)
        #expect((checked.outcome == .approved) == passes)
    }

    @Test("An approval of a command covers every variable")
    func allVariables() {
        var tree = execTree()
        setPath(&tree, ["credentials"][...], [
            ["id": "c1", "vault": "beispiel.app.k8s-test", "title": "datenbank-test", "field": "password", "environment": "test"],
            ["id": "c2", "vault": "beispiel.app.k8s-test", "title": "cache-test", "field": "password", "environment": "test"]])
        setPath(&tree, ["exec", "env"][...], [
            ["name": "PGPASSWORD", "credential_id": "c1"], ["name": "CACHE_PASSWORD", "credential_id": "c2"]])
        let checked = guardApproval(Self.approval(["c1"], ttl: 1800), request: Self.request(tree), testSwitches: false)
        #expect(checked.outcome == .error)
    }
}

@Suite("Exec request: display of the command")
struct ShellWordTests {
    @Test("Each argument is one shell word, without loss", arguments: [
        ("psql", "psql"),
        ("--host=datenbank-test.beispiel.internal", "--host=datenbank-test.beispiel.internal"),
        ("select count(*) from auftrag", "'select count(*) from auftrag'"),
        ("it's", #"'it'\''s'"#),
        ("", "''"),
        ("a\nb", #"$'a\nb'"#),
        ("a\tb'c\\", #"$'a\tb\'c\\'"#),
    ])
    func word(arg: String, shown: String) {
        #expect(DisplayText.shellWord(arg) == shown)
    }
}

@Suite("Exec request: process", .serialized)
struct ExecProcessTests {
    @Test("T38 part (window): without a confirmation each command ends with timeout and exit 11",
          arguments: ["exec-dev", "exec-test", "exec-prod", "exec-long"])
    func timeout(sample: String) throws {
        let r = try AppProcess.run(["--no-window", "--auto-close", "0.3"], input: try Fixtures.sampleData(sample))
        #expect(r.code == 11, "\(r.stderr)")
        #expect(try r.decision()["outcome"] as? String == "timeout")
    }

    @Test("T38 part (window): the approval of a command carries the lifetime, or none in prod",
          arguments: [("exec-test", Int?(1800)), ("exec-prod", nil)])
    func approved(sample: String, ttl: Int?) throws {
        let r = try AppProcess.run(["--no-window", "--skip-auth", "--demo-select", "all", "--demo-arm-delay", "0", "--demo-confirm"],
                                   input: try Fixtures.sampleData(sample))
        #expect(r.code == 0, "\(r.stderr)")
        let d = try r.decision()
        #expect(d["outcome"] as? String == "approved")
        #expect(d["ttl_seconds"] as? Int == ttl)
    }

    @Test("T38 part (window): neither the decision nor a diagnostic repeats the command")
    func noCommandInOutput() throws {
        let r = try AppProcess.run(["--no-window", "--skip-auth", "--demo-select", "all", "--demo-arm-delay", "0", "--demo-confirm"],
                                   input: try Fixtures.sampleData("exec-test"))
        for value in ["psql", "datenbank-test", "PGPASSWORD", "auftrag", "beispielprojekt"] {
            #expect(!r.stdout.contains(value))
            #expect(!r.stderr.contains(value))
        }
    }

    @Test("D62: a plain request with a credential of test ends with 65 before a window exists")
    func plainOutsideDev() throws {
        var tree = Fixtures.requestTree()
        setPath(&tree, ["credentials"][...], [
            ["id": "c1", "vault": "beispiel.app.k8s-test", "title": "datenbank-test", "field": "password", "environment": "test"]])
        let r = try AppProcess.run([], input: Fixtures.json(tree))
        #expect(r.code == 65)
    }
}

@Suite("Test switch --demo-appearance")
struct AppearanceSwitchTests {
    @Test("A debug build reads light and dark, for the screenshots of the PR", arguments: [
        ("light", DemoAppearance.light), ("dark", .dark),
    ])
    func reads(value: String, want: DemoAppearance) throws {
        #expect(try parseOptions(["--demo-appearance", value], testSwitches: true).demoAppearance == want)
    }

    @Test("Another value is a usage error")
    func rejects() {
        #expect(throws: UsageError.self) { try parseOptions(["--demo-appearance", "blue"], testSwitches: true) }
    }
}
