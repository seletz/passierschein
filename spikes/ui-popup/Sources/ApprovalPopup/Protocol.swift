// JSON protocol between the daemon and the approval app.
// The daemon writes one request to stdin. The app writes one decision to stdout.
// The protocol contains no secret values.
import Foundation

let protocolVersion = 1

/// Exit codes of the approval app.
/// Only exit code 0 together with outcome "approved" is an approval.
enum ExitCode: Int32 {
    case approved = 0
    case denied = 10
    case timeout = 11
    case authError = 12
    case usage = 64
    case invalidRequest = 65
    case internalError = 70
}

struct ApprovalRequest: Codable {
    var version: Int
    var type: String
    var requestId: String
    /// Time in seconds until the app closes with outcome "timeout".
    var timeoutSeconds: Int?
    var verified: VerifiedFacts
    var claims: AgentClaims
    var credentials: [CredentialRef]
    var grant: GrantProposal
    /// Present only when the request deviates from an active grant.
    var existingGrant: ExistingGrant?
}

/// Facts that the daemon verified. The agent cannot change them.
struct VerifiedFacts: Codable {
    /// "local" or "vm".
    var origin: String
    var project: String
    var peerPid: Int?
    var process: String?
    var cwd: String?
    var vmName: String?
    var sshSession: String?
}

/// Text from the agent. The daemon did not verify it.
struct AgentClaims: Codable {
    var agent: String?
    var task: String?
    var reason: String?
}

/// Reference to one credential. It contains the title only, never the value.
struct CredentialRef: Codable, Identifiable {
    var id: String
    var vault: String
    var title: String
    /// "dev" is the normal case. All other values are outside dev.
    var environment: String
    /// Deviation code, for example "new_item", "new_vault", "outside_scope".
    var deviation: String?

    var isDev: Bool { environment == "dev" }
}

struct GrantScope: Codable {
    var project: String
    var vaults: [String]
}

struct GrantProposal: Codable {
    var ttlSeconds: Int
    var scope: GrantScope
}

struct ExistingGrant: Codable {
    var grantId: String
    var expiresAt: String
    var scope: GrantScope
    var itemCount: Int
}

struct ApprovalDecision: Codable {
    var version = protocolVersion
    var type = "approval_decision"
    var requestId: String?
    /// "approved", "denied", "timeout" or "error".
    var outcome: String
    var approvedCredentialIds: [String] = []
    var ttlSeconds: Int?
    var auth: AuthResult?
    var error: DecisionError?
    var elapsedMs: Int
}

struct AuthResult: Codable {
    /// "biometrics", "biometrics_or_companion", "owner" or "skipped".
    var policy: String
    var ok: Bool
}

struct DecisionError: Codable {
    /// "invalid_request", "unsupported_version", "auth_unavailable", "auth_failed", "internal".
    var code: String
    var message: String
}

enum RequestError: Error {
    case invalid(String)
    case unsupportedVersion(Int)
}

func decodeRequest(_ data: Data) throws -> ApprovalRequest {
    let decoder = JSONDecoder()
    decoder.keyDecodingStrategy = .convertFromSnakeCase
    // Read the version first. A request of a different version can have different fields.
    struct Envelope: Codable { var version: Int }
    if let envelope = try? decoder.decode(Envelope.self, from: data), envelope.version != protocolVersion {
        throw RequestError.unsupportedVersion(envelope.version)
    }
    let request: ApprovalRequest
    do {
        request = try decoder.decode(ApprovalRequest.self, from: data)
    } catch {
        throw RequestError.invalid("The request is not valid JSON for this schema: \(error)")
    }
    guard request.version == protocolVersion else {
        throw RequestError.unsupportedVersion(request.version)
    }
    guard request.type == "approval_request" else {
        throw RequestError.invalid("The field 'type' must be 'approval_request'.")
    }
    guard !request.credentials.isEmpty else {
        throw RequestError.invalid("The request contains no credentials.")
    }
    guard Set(request.credentials.map(\.id)).count == request.credentials.count else {
        throw RequestError.invalid("The credential IDs are not unique.")
    }
    return request
}

func encodeDecision(_ decision: ApprovalDecision) -> String {
    let encoder = JSONEncoder()
    encoder.keyEncodingStrategy = .convertToSnakeCase
    encoder.outputFormatting = [.sortedKeys, .withoutEscapingSlashes]
    guard let data = try? encoder.encode(decision), let text = String(data: data, encoding: .utf8) else {
        return #"{"version":1,"type":"approval_decision","outcome":"error","error":{"code":"internal","message":"The app cannot encode the decision."},"elapsed_ms":0}"#
    }
    return text
}
