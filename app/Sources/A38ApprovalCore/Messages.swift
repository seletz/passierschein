// The messages between the daemon and the approval app (boundary B2).
//
// The types follow the Go package internal/approval: Request, Credential,
// GrantProposal, ExistingGrant, ExecCommand, Decision, AuthResult, Notice. The field
// names are snake_case. No message contains a secret value: a credential
// has an ID, the vault, the item title and the field label only.
import Foundation

/// The version of the schema of request, decision and notice.
public let protocolVersion = 1

/// The values of the field "type".
public enum MessageType {
    public static let request = "approval_request"
    public static let decision = "approval_decision"
    public static let notice = "notice"
}

/// How the peer reached the daemon.
public enum Origin: String, Sendable, Equatable {
    case local
    case vm
}

/// The facts that the daemon verified (object "verified").
public struct Facts: Sendable, Equatable {
    public var sessionID: String
    public var origin: Origin
    public var project: String
    public var peerPID: Int?
    public var peerUID: Int
    public var process: String?
    public var cwd: String?

    public init(sessionID: String, origin: Origin, project: String, peerPID: Int? = nil,
                peerUID: Int, process: String? = nil, cwd: String? = nil) {
        self.sessionID = sessionID
        self.origin = origin
        self.project = project
        self.peerPID = peerPID
        self.peerUID = peerUID
        self.process = process
        self.cwd = cwd
    }
}

/// The text of the agent (object "claims"). The daemon did not verify it.
public struct Claims: Sendable, Equatable {
    public var sessionName: String?
    public var agent: String?
    public var task: String?
    public var reason: String?

    public init(sessionName: String? = nil, agent: String? = nil, task: String? = nil, reason: String? = nil) {
        self.sessionName = sessionName
        self.agent = agent
        self.task = task
        self.reason = reason
    }
}

/// How an entry goes beyond an existing grant.
public enum Deviation: String, Sendable, Equatable {
    case newItem = "new_item"
    case newVault = "new_vault"
}

/// One entry of an approval request.
public struct Credential: Sendable, Equatable, Identifiable {
    public var id: String
    public var vault: String
    public var title: String
    public var field: String
    public var environment: String
    public var deviation: Deviation?

    public init(id: String, vault: String, title: String, field: String,
                environment: String = "dev", deviation: Deviation? = nil) {
        self.id = id
        self.vault = vault
        self.title = title
        self.field = field
        self.environment = environment
        self.deviation = deviation
    }
}

/// A project and vaults, for the display only.
public struct GrantScope: Sendable, Equatable {
    public var project: String
    public var vaults: [String]

    public init(project: String, vaults: [String]) {
        self.project = project
        self.vaults = vaults
    }
}

/// The grant that an approval creates.
public struct GrantProposal: Sendable, Equatable {
    /// The lifetime from the configuration. The human can select a shorter
    /// one, never a longer one (D32).
    public var ttlSeconds: Int64
    public var scope: GrantScope

    public init(ttlSeconds: Int64, scope: GrantScope) {
        self.ttlSeconds = ttlSeconds
        self.scope = scope
    }
}

/// The newest grant that the session already has. It marks a deviation.
public struct ExistingGrant: Sendable, Equatable {
    public var grantID: String
    public var expiresAt: Date
    public var scope: GrantScope
    public var itemCount: Int

    public init(grantID: String, expiresAt: Date, scope: GrantScope, itemCount: Int) {
        self.grantID = grantID
        self.expiresAt = expiresAt
        self.scope = scope
        self.itemCount = itemCount
    }
}

/// The environment of a vault (D47). Prod is the strictest (D62).
public enum ExecEnvironment: String, Sendable, Equatable, Comparable {
    case dev
    case test
    case prod

    var rank: Int {
        switch self {
        case .dev: return 1
        case .test: return 2
        case .prod: return 3
        }
    }

    public static func < (a: ExecEnvironment, b: ExecEnvironment) -> Bool { a.rank < b.rank }
}

/// The kind of approval for a command (D65, D63).
public enum ExecApprovalKind: String, Sendable, Equatable {
    /// A grant with a lifetime and a binding (dev, test).
    case grant
    /// Only this command, one time (prod). No choice of the lifetime.
    case once
}

/// For which commands a grant is valid (D63).
public enum GrantBinding: String, Sendable, Equatable {
    case none
    case program
    case command
}

/// One environment variable of a command. It points to one entry of the
/// request; the entry holds the names of the backend.
public struct ExecVariable: Sendable, Equatable {
    public var name: String
    public var credentialID: String

    public init(name: String, credentialID: String) {
        self.name = name
        self.credentialID = credentialID
    }
}

/// The kind of approval of a command.
public struct ExecApproval: Sendable, Equatable {
    public var kind: ExecApprovalKind
    public var binding: GrantBinding
    /// The lifetimes that the human can select, in descending order. The
    /// first value is the configured lifetime. Empty for "once".
    public var lifetimeChoices: [Int64]

    public init(kind: ExecApprovalKind, binding: GrantBinding, lifetimeChoices: [Int64]) {
        self.kind = kind
        self.binding = binding
        self.lifetimeChoices = lifetimeChoices
    }
}

/// The command of a request of the exec way (object "exec", D65).
public struct ExecCommand: Sendable, Equatable {
    /// The strictest environment of the credentials.
    public var environment: ExecEnvironment
    public var program: String
    public var args: [String]
    public var cwd: String
    public var env: [ExecVariable]
    public var approval: ExecApproval

    public init(environment: ExecEnvironment, program: String, args: [String], cwd: String,
                env: [ExecVariable], approval: ExecApproval) {
        self.environment = environment
        self.program = program
        self.args = args
        self.cwd = cwd
        self.env = env
        self.approval = approval
    }
}

/// One approval request.
public struct ApprovalRequest: Sendable, Equatable {
    public var requestID: String
    public var timeoutSeconds: Int
    public var verified: Facts
    public var claims: Claims
    public var credentials: [Credential]
    public var grant: GrantProposal
    public var existingGrant: ExistingGrant?
    /// The command of a request of the exec way. Nil for a plain request.
    public var exec: ExecCommand?

    public init(requestID: String, timeoutSeconds: Int, verified: Facts, claims: Claims,
                credentials: [Credential], grant: GrantProposal, existingGrant: ExistingGrant? = nil,
                exec: ExecCommand? = nil) {
        self.requestID = requestID
        self.timeoutSeconds = timeoutSeconds
        self.verified = verified
        self.claims = claims
        self.credentials = credentials
        self.grant = grant
        self.existingGrant = existingGrant
        self.exec = exec
    }
}

/// What a notice is about.
public enum NoticeKind: String, Sendable, Equatable {
    /// The daemon found a fault in itself and locked delivery (D48, D36).
    case daemonFault = "daemon_fault"
    /// The configuration of one project is invalid (D28).
    case projectConfigInvalid = "project_config_invalid"
}

/// A notice tells the human about a fault. It has no actions.
public struct Notice: Sendable, Equatable {
    public var noticeID: String
    public var kind: NoticeKind
    public var code: String
    public var project: String?
    public var message: String
    public var occurredAt: Date

    public init(noticeID: String, kind: NoticeKind, code: String, project: String? = nil,
                message: String, occurredAt: Date) {
        self.noticeID = noticeID
        self.kind = kind
        self.code = code
        self.project = project
        self.message = message
        self.occurredAt = occurredAt
    }
}

/// One message on the standard input of the app.
public enum Message: Sendable, Equatable {
    case request(ApprovalRequest)
    case notice(Notice)
}

/// The result of an approval request.
public enum Outcome: String, Sendable, Equatable {
    case approved
    case denied
    case timeout
    case error
}

/// How the human confirmed an approval.
public struct AuthResult: Sendable, Equatable {
    /// "biometrics" for Touch ID, "skipped" for the fake of a debug build.
    public var policy: String
    public var ok: Bool

    public init(policy: String, ok: Bool) {
        self.policy = policy
        self.ok = ok
    }
}

/// The codes of the field error.code.
public enum ErrorCode: String, Sendable, Equatable {
    case invalidRequest = "invalid_request"
    case unsupportedVersion = "unsupported_version"
    case authUnavailable = "auth_unavailable"
    case authFailed = "auth_failed"
    case `internal` = "internal"
}

/// The error of the app.
public struct DecisionError: Sendable, Equatable {
    public var code: ErrorCode
    /// Text for the log of the daemon. It never contains a value of the
    /// request, only field names and IDs.
    public var message: String

    public init(code: ErrorCode, message: String) {
        self.code = code
        self.message = message
    }
}

/// The answer of the app to a request.
public struct Decision: Sendable, Equatable {
    public var requestID: String?
    public var outcome: Outcome
    public var approvedCredentialIDs: [String]
    public var ttlSeconds: Int64?
    public var auth: AuthResult?
    public var error: DecisionError?
    public var elapsedMS: Int64

    public init(requestID: String?, outcome: Outcome, approvedCredentialIDs: [String] = [],
                ttlSeconds: Int64? = nil, auth: AuthResult? = nil, error: DecisionError? = nil,
                elapsedMS: Int64) {
        self.requestID = requestID
        self.outcome = outcome
        self.approvedCredentialIDs = approvedCredentialIDs
        self.ttlSeconds = ttlSeconds
        self.auth = auth
        self.error = error
        self.elapsedMS = elapsedMS
    }
}
