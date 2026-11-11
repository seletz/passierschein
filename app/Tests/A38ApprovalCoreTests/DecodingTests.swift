// Protocol decoding: valid samples and every kind of invalid input.
import Foundation
import Testing
@testable import A38ApprovalCore

@Suite("Decoding")
struct DecodingTests {
    @Test("The samples decode", arguments: ["first-request", "bundle", "deviation", "long-claims"])
    func samplesDecode(name: String) throws {
        let request = try Fixtures.sampleRequest(name)
        #expect(!request.credentials.isEmpty)
        #expect(request.credentials.count <= 10, "D29: a bundle has at most ten entries by default")
        #expect(request.verified.project == "beispielprojekt")
        #expect(request.credentials.allSatisfy { $0.environment == "dev" })
    }

    @Test("The notice samples decode")
    func noticeSamples() throws {
        let fault = try Fixtures.sampleNotice("notice-daemon-fault")
        #expect(fault.kind == .daemonFault)
        #expect(fault.project == nil)
        let config = try Fixtures.sampleNotice("notice-project-config-invalid")
        #expect(config.kind == .projectConfigInvalid)
        #expect(config.project == "beispielprojekt")
    }

    @Test("A full request keeps each field")
    func fullRequest() throws {
        guard case .request(let r) = try decodeMessage(Fixtures.json(Fixtures.requestTree())) else {
            Issue.record("not a request")
            return
        }
        #expect(r.requestID == "req-0001")
        #expect(r.timeoutSeconds == 120)
        #expect(r.verified == Facts(sessionID: "s-0001", origin: .local, project: "beispielprojekt", peerPID: 4242,
                                    peerUID: 501, process: "/usr/local/bin/agent-cli",
                                    cwd: "/Users/beispiel/develop/beispielprojekt"))
        #expect(r.claims.sessionName == "beispiel-koordinator")
        #expect(r.credentials.map(\.id) == ["c1", "c2"])
        #expect(r.credentials[0].field == "password")
        #expect(r.grant.ttlSeconds == 14400)
        #expect(r.existingGrant == nil)
    }

    @Test("The deviation sample names the existing grant")
    func deviation() throws {
        let r = try Fixtures.sampleRequest("deviation")
        let grant = try #require(r.existingGrant)
        #expect(grant.grantID == "grant-0007")
        #expect(grant.itemCount == 2)
        #expect(r.credentials.map(\.deviation) == [.newItem, .newVault])
        #expect(grant.expiresAt == Date(timeIntervalSince1970: 1_794_497_400))
    }

    @Test("A time with nine digits of fraction decodes")
    func goTime() throws {
        let date = try #require(parseRFC3339("2026-11-12T09:41:07.123456789+01:00"))
        #expect(abs(date.timeIntervalSince1970 - 1_794_472_867.123456789) < 0.001)
        #expect(parseRFC3339("yesterday") == nil)
        #expect(parseRFC3339("2026-11-12T09:41:07.Z") == nil)
    }

    /// One invalid input: a change of the valid tree and the expected code.
    struct Invalid: CustomTestStringConvertible, Sendable {
        let name: String
        let code: ErrorCode
        let change: @Sendable (inout [String: Any]) -> Void
        var testDescription: String { name }
    }

    static func set(_ path: [String], _ value: Any?) -> @Sendable (inout [String: Any]) -> Void {
        let box = UncheckedBox(value)
        return { tree in setPath(&tree, path[...], box.value) }
    }

    static let invalidRequests: [Invalid] = [
        Invalid(name: "version 2", code: .unsupportedVersion, change: set(["version"], 2)),
        Invalid(name: "version missing", code: .invalidRequest, change: set(["version"], nil)),
        Invalid(name: "version as text", code: .invalidRequest, change: set(["version"], "1")),
        Invalid(name: "type unknown", code: .invalidRequest, change: set(["type"], "approval_magic")),
        Invalid(name: "type missing", code: .invalidRequest, change: set(["type"], nil)),
        Invalid(name: "unknown top field", code: .invalidRequest, change: set(["auto_approve"], true)),
        Invalid(name: "request_id missing", code: .invalidRequest, change: set(["request_id"], nil)),
        Invalid(name: "request_id with space", code: .invalidRequest, change: set(["request_id"], "req 1")),
        Invalid(name: "request_id empty", code: .invalidRequest, change: set(["request_id"], "")),
        Invalid(name: "timeout missing", code: .invalidRequest, change: set(["timeout_seconds"], nil)),
        Invalid(name: "timeout zero", code: .invalidRequest, change: set(["timeout_seconds"], 0)),
        Invalid(name: "timeout fraction", code: .invalidRequest, change: set(["timeout_seconds"], 1.5)),
        Invalid(name: "timeout boolean", code: .invalidRequest, change: set(["timeout_seconds"], true)),
        Invalid(name: "verified missing", code: .invalidRequest, change: set(["verified"], nil)),
        Invalid(name: "verified null", code: .invalidRequest, change: set(["verified"], NSNull())),
        Invalid(name: "session_id missing", code: .invalidRequest, change: set(["verified", "session_id"], nil)),
        Invalid(name: "origin unknown", code: .invalidRequest, change: set(["verified", "origin"], "cloud")),
        Invalid(name: "project empty", code: .invalidRequest, change: set(["verified", "project"], "")),
        Invalid(name: "peer_uid missing", code: .invalidRequest, change: set(["verified", "peer_uid"], nil)),
        Invalid(name: "peer_pid zero", code: .invalidRequest, change: set(["verified", "peer_pid"], 0)),
        Invalid(name: "VM facts of the prototype", code: .invalidRequest, change: set(["verified", "vm_name"], "beispiel-vm-01")),
        Invalid(name: "claims missing", code: .invalidRequest, change: set(["claims"], nil)),
        Invalid(name: "unknown claim", code: .invalidRequest, change: set(["claims", "verified_by"], "daemon")),
        Invalid(name: "claim not a string", code: .invalidRequest, change: set(["claims", "task"], 7)),
        Invalid(name: "credentials missing", code: .invalidRequest, change: set(["credentials"], nil)),
        Invalid(name: "credentials empty", code: .invalidRequest, change: set(["credentials"], [Any]())),
        Invalid(name: "credentials null", code: .invalidRequest, change: set(["credentials"], NSNull())),
        Invalid(name: "duplicate credential IDs", code: .invalidRequest, change: { tree in
            var list = tree["credentials"] as! [[String: Any]]
            list[1]["id"] = "c1"
            tree["credentials"] = list
        }),
        Invalid(name: "too many credentials", code: .invalidRequest, change: { tree in
            tree["credentials"] = (1...(Limits.maxCredentials + 1)).map {
                ["id": "c\($0)", "vault": "beispiel.app.k8s-dev", "title": "datenbank-dev", "field": "f\($0)", "environment": "dev"]
            }
        }),
        Invalid(name: "credential field missing", code: .invalidRequest, change: setCredential("field", nil)),
        Invalid(name: "credential vault empty", code: .invalidRequest, change: setCredential("vault", "")),
        Invalid(name: "credential unknown field", code: .invalidRequest, change: setCredential("value", "x")),
        Invalid(name: "environment prod (D24)", code: .invalidRequest, change: setCredential("environment", "prod")),
        Invalid(name: "environment test (D24)", code: .invalidRequest, change: setCredential("environment", "test")),
        Invalid(name: "environment unknown", code: .invalidRequest, change: setCredential("environment", "staging")),
        Invalid(name: "deviation outside_scope (G26)", code: .invalidRequest, change: setCredential("deviation", "outside_scope")),
        Invalid(name: "deviation without existing grant", code: .invalidRequest, change: setCredential("deviation", "new_item")),
        Invalid(name: "existing grant without deviation", code: .invalidRequest, change: set(["existing_grant"], [
            "grant_id": "grant-0007", "expires_at": "2026-11-12T15:30:00Z", "item_count": 2,
            "scope": ["project": "beispielprojekt", "vaults": ["beispiel.app.k8s-dev"]] as [String: Any],
        ] as [String: Any])),
        Invalid(name: "existing grant bad time", code: .invalidRequest, change: { tree in
            setPath(&tree, ["existing_grant"][...], [
                "grant_id": "grant-0007", "expires_at": "tomorrow", "item_count": 2,
                "scope": ["project": "beispielprojekt", "vaults": ["beispiel.app.k8s-dev"]] as [String: Any],
            ] as [String: Any])
            var list = tree["credentials"] as! [[String: Any]]
            for i in list.indices { list[i]["deviation"] = "new_item" }
            tree["credentials"] = list
        }),
        Invalid(name: "grant missing", code: .invalidRequest, change: set(["grant"], nil)),
        Invalid(name: "ttl zero", code: .invalidRequest, change: set(["grant", "ttl_seconds"], 0)),
        Invalid(name: "ttl negative", code: .invalidRequest, change: set(["grant", "ttl_seconds"], -60)),
        Invalid(name: "scope unknown field", code: .invalidRequest, change: set(["grant", "scope", "all_vaults"], true)),
        Invalid(name: "scope vaults not a list", code: .invalidRequest, change: set(["grant", "scope", "vaults"], "beispiel.app.k8s-dev")),
    ]

    static func setCredential(_ key: String, _ value: Any?) -> @Sendable (inout [String: Any]) -> Void {
        let box = UncheckedBox(value)
        return { tree in
            var list = tree["credentials"] as! [[String: Any]]
            list[0][key] = box.value
            tree["credentials"] = list
        }
    }

    @Test("An invalid request is rejected with its code", arguments: invalidRequests)
    func invalidRequest(_ c: Invalid) {
        var tree = Fixtures.requestTree()
        c.change(&tree)
        #expect(throws: ProtocolError.self) { try decodeMessage(Fixtures.json(tree)) }
        do {
            _ = try decodeMessage(Fixtures.json(tree))
        } catch {
            #expect(error.code == c.code, "\(error.message)")
            // The error message names fields, never a value of the request.
            for value in ["beispielprojekt", "datenbank-dev", "beispiel-koordinator", "Run the tests", "/usr/local/bin"] {
                #expect(!error.message.contains(value), "\(error.message)")
            }
        }
    }

    @Test("The error names the request ID if it is readable")
    func requestIDInError() {
        var tree = Fixtures.requestTree()
        tree["grant"] = nil
        do {
            _ = try decodeMessage(Fixtures.json(tree))
            Issue.record("no error")
        } catch {
            #expect(error.requestID == "req-0001")
        }
    }

    @Test("Input that is not one JSON object is rejected", arguments: [
        "", "not json", "[]", "42", "\"approval_request\"", "{\"version\":1}{\"version\":1}", "{\"version\":1,",
    ])
    func notAnObject(text: String) {
        #expect(throws: ProtocolError.self) { try decodeMessage(Data(text.utf8)) }
    }

    @Test("A message larger than the limit is rejected")
    func tooLarge() {
        var tree = Fixtures.requestTree()
        tree["padding"] = String(repeating: "x", count: Limits.maxMessageBytes)
        do {
            _ = try decodeMessage(Fixtures.json(tree))
            Issue.record("no error")
        } catch {
            #expect(error.code == .invalidRequest)
            #expect(error.message.contains("larger"))
        }
    }

    @Test("A null list of vaults in the scope is an empty list")
    func nullVaults() throws {
        var tree = Fixtures.requestTree()
        setPath(&tree, ["grant", "scope", "vaults"][...], NSNull())
        guard case .request(let r) = try decodeMessage(Fixtures.json(tree)) else {
            Issue.record("not a request")
            return
        }
        #expect(r.grant.scope.vaults.isEmpty)
    }

    @Test("Empty optional claims are missing claims")
    func emptyClaims() throws {
        var tree = Fixtures.requestTree()
        tree["claims"] = [String: Any]()
        guard case .request(let r) = try decodeMessage(Fixtures.json(tree)) else {
            Issue.record("not a request")
            return
        }
        #expect(r.claims == Claims())
    }

    static let invalidNotices: [Invalid] = [
        Invalid(name: "kind unknown", code: .invalidRequest, change: set(["kind"], "disk_full")),
        Invalid(name: "code missing", code: .invalidRequest, change: set(["code"], nil)),
        Invalid(name: "code not a code", code: .invalidRequest, change: set(["code"], "Audit Failed!")),
        Invalid(name: "message empty", code: .invalidRequest, change: set(["message"], "")),
        Invalid(name: "occurred_at missing", code: .invalidRequest, change: set(["occurred_at"], nil)),
        Invalid(name: "notice_id missing", code: .invalidRequest, change: set(["notice_id"], nil)),
        Invalid(name: "unknown field", code: .invalidRequest, change: set(["actions"], ["restart"])),
        Invalid(name: "daemon fault with project", code: .invalidRequest, change: set(["project"], "beispielprojekt")),
        Invalid(name: "project notice without project", code: .invalidRequest, change: set(["kind"], "project_config_invalid")),
        Invalid(name: "version 2", code: .unsupportedVersion, change: set(["version"], 2)),
    ]

    @Test("An invalid notice is rejected", arguments: invalidNotices)
    func invalidNotice(_ c: Invalid) {
        var tree = Fixtures.noticeTree()
        c.change(&tree)
        do {
            _ = try decodeMessage(Fixtures.json(tree))
            Issue.record("no error")
        } catch {
            #expect(error.code == c.code, "\(error.message)")
        }
    }
}

/// Moves a value of type Any into a Sendable closure. The tests use it on
/// one thread only.
struct UncheckedBox: @unchecked Sendable {
    let value: Any?
    init(_ value: Any?) { self.value = value }
}

/// Sets or removes (nil) the value at a path of nested objects.
func setPath(_ tree: inout [String: Any], _ path: ArraySlice<String>, _ value: Any?) {
    guard let key = path.first else { return }
    if path.count == 1 {
        tree[key] = value
        return
    }
    var child = tree[key] as? [String: Any] ?? [:]
    setPath(&child, path.dropFirst(), value)
    tree[key] = child
}
