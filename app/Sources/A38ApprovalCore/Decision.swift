// The decision line on the standard output and the exit codes.
import Foundation

/// The exit codes of the app. Only exit code 0 together with the outcome
/// "approved" is an approval. A closed notice also ends with 0; it writes
/// no decision.
public enum ExitCode: Int32, Sendable {
    case approved = 0
    case denied = 10
    case timeout = 11
    case authError = 12
    case usage = 64
    case invalidRequest = 65
    case internalError = 70

    /// Exit code 0 without a decision: --help, --version and a closed
    /// notice. The daemon reads an approval only together with a decision
    /// line with the outcome approved.
    public static let success = ExitCode.approved

    /// The exit code of a decision.
    public static func of(_ decision: Decision) -> ExitCode {
        switch decision.outcome {
        case .approved: return .approved
        case .denied: return .denied
        case .timeout: return .timeout
        case .error:
            switch decision.error?.code {
            case .invalidRequest?, .unsupportedVersion?: return .invalidRequest
            case .authUnavailable?, .authFailed?: return .authError
            case .internal?, nil: return .internalError
            }
        }
    }
}

/// The policy name of a confirmation with Touch ID only (D31).
public let biometricsPolicy = "biometrics"
/// The policy name of the fake authenticator of a debug build.
public let skippedPolicy = "skipped"

extension Decision {
    /// An error decision. requestID is nil if the app could not read the
    /// request.
    public static func failure(_ code: ErrorCode, _ message: String, requestID: String? = nil,
                               auth: AuthResult? = nil, elapsedMS: Int64) -> Decision {
        Decision(requestID: requestID, outcome: .error, auth: auth,
                 error: DecisionError(code: code, message: message), elapsedMS: elapsedMS)
    }
}

/// Checks the last barrier before the app writes a decision: an approval
/// needs a passed authentication, at least one entry, IDs of the request
/// and a lifetime that is not longer than the proposal. A release build
/// also needs the policy of Touch ID. Each other approval becomes an
/// internal error, never an approval.
public func guardApproval(_ decision: Decision, request: ApprovalRequest?,
                          testSwitches: Bool = TestSwitches.compiledIn) -> Decision {
    guard decision.outcome == .approved else { return decision }
    func reject(_ why: String) -> Decision {
        .failure(.internal, "the app blocked an approval: \(why)", requestID: decision.requestID,
                 elapsedMS: decision.elapsedMS)
    }
    guard let request else { return reject("no request") }
    guard decision.requestID == request.requestID else { return reject("the request ID does not match") }
    guard let auth = decision.auth, auth.ok else { return reject("no passed authentication") }
    if !testSwitches && auth.policy != biometricsPolicy {
        return reject("the policy is not biometrics")
    }
    guard !decision.approvedCredentialIDs.isEmpty else { return reject("no entry selected") }
    let known = Set(request.credentials.map(\.id))
    guard Set(decision.approvedCredentialIDs).count == decision.approvedCredentialIDs.count,
          decision.approvedCredentialIDs.allSatisfy(known.contains) else {
        return reject("an entry is not in the request")
    }
    guard let ttl = decision.ttlSeconds, ttl > 0, ttl <= request.grant.ttlSeconds else {
        return reject("the lifetime is not valid")
    }
    return decision
}

/// Encodes the decision as one line of JSON without a line break.
public func encodeDecision(_ decision: Decision) -> String {
    var object: [String: Any] = [
        "version": protocolVersion,
        "type": MessageType.decision,
        "outcome": decision.outcome.rawValue,
        // Empty for each outcome that is not approved.
        "approved_credential_ids": decision.outcome == .approved ? decision.approvedCredentialIDs : [],
        "elapsed_ms": decision.elapsedMS,
    ]
    if let id = decision.requestID { object["request_id"] = id }
    if decision.outcome == .approved, let ttl = decision.ttlSeconds { object["ttl_seconds"] = ttl }
    if let auth = decision.auth { object["auth"] = ["policy": auth.policy, "ok": auth.ok] }
    if let error = decision.error {
        object["error"] = ["code": error.code.rawValue, "message": error.message]
    }
    guard let data = try? JSONSerialization.data(withJSONObject: object, options: [.sortedKeys, .withoutEscapingSlashes]),
          let line = String(data: data, encoding: .utf8) else {
        return #"{"approved_credential_ids":[],"elapsed_ms":0,"error":{"code":"internal","message":"the app cannot encode the decision"},"outcome":"error","type":"approval_decision","version":1}"#
    }
    return line
}
