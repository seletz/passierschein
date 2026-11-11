// Shared test data. Neutral names only.
import Foundation
@testable import A38ApprovalCore

enum Fixtures {
    /// The folder app/Samples.
    static let samplesDir = URL(fileURLWithPath: #filePath)
        .deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        .appendingPathComponent("Samples")

    static func sampleData(_ name: String) throws -> Data {
        try Data(contentsOf: samplesDir.appendingPathComponent(name + ".json"))
    }

    static func sampleRequest(_ name: String) throws -> ApprovalRequest {
        guard case .request(let request) = try decodeMessage(try sampleData(name)) else {
            throw FixtureError.notARequest
        }
        return request
    }

    static func sampleNotice(_ name: String) throws -> Notice {
        guard case .notice(let notice) = try decodeMessage(try sampleData(name)) else {
            throw FixtureError.notANotice
        }
        return notice
    }

    enum FixtureError: Error { case notARequest, notANotice }

    /// A valid request as a JSON tree; tests change it.
    static func requestTree() -> [String: Any] {
        [
            "version": 1,
            "type": "approval_request",
            "request_id": "req-0001",
            "timeout_seconds": 120,
            "verified": [
                "session_id": "s-0001", "origin": "local", "project": "beispielprojekt",
                "peer_pid": 4242, "peer_uid": 501, "process": "/usr/local/bin/agent-cli",
                "cwd": "/Users/beispiel/develop/beispielprojekt",
            ] as [String: Any],
            "claims": [
                "session_name": "beispiel-koordinator", "agent": "coding-agent",
                "task": "Run the tests", "reason": "The tests need the login.",
            ],
            "credentials": [
                ["id": "c1", "vault": "beispiel.app.k8s-dev", "title": "datenbank-dev", "field": "password", "environment": "dev"],
                ["id": "c2", "vault": "beispiel.app.k8s-dev", "title": "objektspeicher-dev", "field": "access-key", "environment": "dev"],
            ],
            "grant": [
                "ttl_seconds": 14400,
                "scope": ["project": "beispielprojekt", "vaults": ["beispiel.app.k8s-dev"]] as [String: Any],
            ] as [String: Any],
        ]
    }

    static func noticeTree() -> [String: Any] {
        [
            "version": 1, "type": "notice", "notice_id": "notice-0001", "kind": "daemon_fault",
            "code": "audit_failed", "message": "The daemon cannot write the audit log. Run a38 doctor.",
            "occurred_at": "2026-11-12T09:41:07Z",
        ]
    }

    static func json(_ tree: Any) -> Data {
        try! JSONSerialization.data(withJSONObject: tree, options: [.sortedKeys])
    }

    static func request(credentials: [Credential], existing: ExistingGrant? = nil,
                        claims: Claims = Claims(sessionName: "beispiel-koordinator", agent: "coding-agent",
                                                task: "Run the tests", reason: "The tests need the login."),
                        ttl: Int64 = 14400, timeout: Int = 120) -> ApprovalRequest {
        ApprovalRequest(
            requestID: "req-0001", timeoutSeconds: timeout,
            verified: Facts(sessionID: "s-0001", origin: .local, project: "beispielprojekt", peerPID: 4242,
                            peerUID: 501, process: "/usr/local/bin/agent-cli",
                            cwd: "/Users/beispiel/develop/beispielprojekt"),
            claims: claims, credentials: credentials,
            grant: GrantProposal(ttlSeconds: ttl, scope: GrantScope(project: "beispielprojekt",
                                                                    vaults: Array(Set(credentials.map(\.vault))).sorted())),
            existingGrant: existing)
    }

    static func credentials(_ count: Int, vault: String = "beispiel.app.k8s-dev", from: Int = 1) -> [Credential] {
        (from..<(from + count)).map { Credential(id: "c\($0)", vault: vault, title: "datenbank-dev-\($0)", field: "password") }
    }
}

/// A clock that the test moves.
@MainActor
final class TestClock {
    var now: TimeInterval = 1000
    func advance(_ seconds: TimeInterval) { now += seconds }
}
