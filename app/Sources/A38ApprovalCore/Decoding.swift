// Strict decoding of the one message on the standard input.
//
// The app shows a popup only for a valid message. Each finding ends the
// app with an error decision (exit code 65) before a window exists. The
// rules follow the Go types of internal/approval: each field that Go always
// writes is required, each unknown field is an error. An error message
// names the field path, never a value of the request.
import Foundation

/// The limits of a message. They protect the app from a request that
/// nobody can read and from a very large input.
public enum Limits {
    /// The largest message on the standard input, in bytes.
    public static let maxMessageBytes = 256 * 1024
    /// The most entries of one request. The daemon limits a bundle to ten
    /// entries by default (D29); the value is configurable, so the app
    /// accepts more, but not without limit.
    public static let maxCredentials = 50
    /// The range of timeout_seconds.
    public static let timeoutSeconds: ClosedRange<Int64> = 1...3600
    /// The longest ID (request, session, credential, grant, notice).
    public static let maxIDLength = 128
    /// The longest text of one field.
    public static let maxTextBytes = 8 * 1024
    /// The longest path of a program or a working directory, in bytes
    /// (Go: protocol.MaxPathLength).
    public static let maxPathBytes = 1024
    /// The most arguments of a command (Go: protocol.MaxExecArgs).
    public static let maxExecArgs = 256
    /// The longest arguments of a command together, in bytes (Go:
    /// protocol.MaxExecArgsSize).
    public static let maxExecArgsBytes = 16 * 1024
    /// The most variables of a command (Go: protocol.MaxReferences).
    public static let maxExecVariables = 100
    /// The longest lifetime of a grant in seconds (D66: 24 h).
    public static let maxGrantSeconds: Int64 = 24 * 3600
}

/// A request that the app does not accept.
public struct ProtocolError: Error, Equatable, Sendable {
    public var code: ErrorCode
    public var message: String
    /// The request ID, if the app could read it.
    public var requestID: String?

    public init(code: ErrorCode, message: String, requestID: String? = nil) {
        self.code = code
        self.message = message
        self.requestID = requestID
    }

    static func invalid(_ message: String) -> ProtocolError {
        ProtocolError(code: .invalidRequest, message: message)
    }
}

/// Decodes the one message of the standard input.
public func decodeMessage(_ data: Data) throws(ProtocolError) -> Message {
    guard data.count <= Limits.maxMessageBytes else {
        throw .invalid("the message is larger than \(Limits.maxMessageBytes) bytes")
    }
    let value: Any
    do {
        value = try JSONSerialization.jsonObject(with: data, options: [])
    } catch {
        throw .invalid("the input is not one valid JSON object")
    }
    let root = try Reader(value, path: "message")
    // Read the version first: a message of another version can have other
    // fields.
    let version = try root.int("version")
    guard version == Int64(protocolVersion) else {
        throw ProtocolError(code: .unsupportedVersion,
                            message: "the app supports protocol version \(protocolVersion) only, got \(version)")
    }
    let type = try root.string("type")
    switch type {
    case MessageType.request:
        // Read the request ID early, so that an error decision can name it.
        let requestID = try? root.id("request_id")
        do {
            return .request(try decodeRequest(root))
        } catch {
            var e = error
            e.requestID = requestID
            throw e
        }
    case MessageType.notice:
        return .notice(try decodeNotice(root))
    default:
        throw .invalid("type: the message type is not known")
    }
}

private func decodeRequest(_ root: Reader) throws(ProtocolError) -> ApprovalRequest {
    let requestID = try root.id("request_id")
    let timeout = try root.int("timeout_seconds")
    guard Limits.timeoutSeconds.contains(timeout) else {
        throw .invalid("timeout_seconds: the value is not in \(Limits.timeoutSeconds)")
    }
    let verified = try decodeFacts(try root.object("verified"))
    let claims = try decodeClaims(try root.object("claims"))
    let rawCredentials = try root.array("credentials")
    guard !rawCredentials.isEmpty else {
        throw .invalid("credentials: the request contains no credentials")
    }
    guard rawCredentials.count <= Limits.maxCredentials else {
        throw .invalid("credentials: the request contains more than \(Limits.maxCredentials) credentials")
    }
    var credentials: [Credential] = []
    var seen: Set<String> = []
    for (index, raw) in rawCredentials.enumerated() {
        let credential = try decodeCredential(try Reader(raw, path: "credentials[\(index)]"))
        guard seen.insert(credential.id).inserted else {
            throw .invalid("credentials[\(index)].id: the ID is not unique")
        }
        credentials.append(credential)
    }
    let grant = try decodeGrantProposal(try root.object("grant"))
    let existing = try root.optionalObject("existing_grant").map { reader throws(ProtocolError) in
        try decodeExistingGrant(reader)
    }
    let exec = try root.optionalObject("exec").map { reader throws(ProtocolError) in
        try decodeExec(reader, credentials: credentials, ttl: grant.ttlSeconds)
    }
    try root.finish()

    if exec == nil {
        // D62: a value outside dev never goes as an MCP answer. A plain
        // request outside dev never reaches the approval (fail closed).
        if let index = credentials.firstIndex(where: { $0.environment != "dev" }) {
            throw .invalid("credentials[\(index)].environment: a request outside dev needs the exec way")
        }
        guard grant.ttlSeconds > 0 else {
            throw .invalid("grant.ttl_seconds: the lifetime is not more than zero")
        }
    }

    // existing_grant marks a deviation. Then each entry names its kind of
    // deviation; without it no entry does (internal/approval, D8).
    for (index, credential) in credentials.enumerated() {
        if existing != nil && credential.deviation == nil {
            throw .invalid("credentials[\(index)].deviation: a deviation request needs the kind of deviation")
        }
        if existing == nil && credential.deviation != nil {
            throw .invalid("credentials[\(index)].deviation: the field needs existing_grant")
        }
    }
    return ApprovalRequest(requestID: requestID, timeoutSeconds: Int(timeout), verified: verified,
                           claims: claims, credentials: credentials, grant: grant, existingGrant: existing,
                           exec: exec)
}

private func decodeFacts(_ r: Reader) throws(ProtocolError) -> Facts {
    let sessionID = try r.id("session_id")
    let originText = try r.string("origin")
    guard let origin = Origin(rawValue: originText) else {
        throw .invalid("verified.origin: the origin is not local or vm")
    }
    let project = try r.text("project")
    let peerPID = try r.optionalInt("peer_pid")
    if let pid = peerPID, pid <= 0 {
        throw .invalid("verified.peer_pid: the value is not a process ID")
    }
    let peerUID = try r.int("peer_uid")
    guard peerUID >= 0 else {
        throw .invalid("verified.peer_uid: the value is not a user ID")
    }
    let facts = Facts(sessionID: sessionID, origin: origin, project: project,
                      peerPID: peerPID.map { Int($0) }, peerUID: Int(peerUID),
                      process: try r.optionalText("process"), cwd: try r.optionalText("cwd"))
    try r.finish()
    return facts
}

private func decodeClaims(_ r: Reader) throws(ProtocolError) -> Claims {
    let claims = Claims(sessionName: try r.optionalText("session_name"),
                        agent: try r.optionalText("agent"),
                        task: try r.optionalText("task"),
                        reason: try r.optionalText("reason"))
    try r.finish()
    return claims
}

private func decodeCredential(_ r: Reader) throws(ProtocolError) -> Credential {
    let id = try r.id("id")
    let vault = try r.text("vault")
    let title = try r.text("title")
    let field = try r.text("field")
    let environment = try r.string("environment")
    // D62: test and prod reach the approval only with the exec way. The
    // check is in decodeRequest, after the object exec.
    guard ExecEnvironment(rawValue: environment) != nil else {
        throw .invalid("\(r.path).environment: the environment is not dev, test or prod")
    }
    var deviation: Deviation?
    if let text = try r.optionalString("deviation"), !text.isEmpty {
        guard let value = Deviation(rawValue: text) else {
            throw .invalid("\(r.path).deviation: the kind of deviation is not known")
        }
        deviation = value
    }
    try r.finish()
    return Credential(id: id, vault: vault, title: title, field: field,
                      environment: environment, deviation: deviation)
}

private func decodeScope(_ r: Reader) throws(ProtocolError) -> GrantScope {
    // The scope is for the display only (G26); the app shows the verified
    // project. An empty project is no reason to fail.
    let project = try r.string("project")
    // Go writes a nil slice as null. The scope is for the display only.
    let rawVaults = try r.array("vaults", allowNull: true)
    var vaults: [String] = []
    for (index, raw) in rawVaults.enumerated() {
        guard let name = raw as? String, !name.isEmpty, name.utf8.count <= Limits.maxTextBytes else {
            throw .invalid("\(r.path).vaults[\(index)]: the value is not a vault name")
        }
        vaults.append(name)
    }
    try r.finish()
    return GrantScope(project: project, vaults: vaults)
}

private func decodeGrantProposal(_ r: Reader) throws(ProtocolError) -> GrantProposal {
    // 0 is valid only for "only this command" (exec way, prod). The
    // request checks it with the object exec.
    let ttl = try r.int("ttl_seconds")
    guard ttl >= 0 else {
        throw .invalid("grant.ttl_seconds: the lifetime is negative")
    }
    let scope = try decodeScope(try r.object("scope"))
    try r.finish()
    return GrantProposal(ttlSeconds: ttl, scope: scope)
}

/// Decodes the object exec (D65). The rules follow the Go function
/// approval.Request.Validate.
private func decodeExec(_ r: Reader, credentials: [Credential], ttl: Int64) throws(ProtocolError) -> ExecCommand {
    let envText = try r.string("environment")
    guard let environment = ExecEnvironment(rawValue: envText) else {
        throw .invalid("exec.environment: the environment is not dev, test or prod")
    }
    let strictest = credentials.compactMap { ExecEnvironment(rawValue: $0.environment) }.max()
    guard environment == strictest else {
        throw .invalid("exec.environment: the environment is not the strictest environment of the credentials")
    }
    let program = try execPath(r, "program")
    let rawArgs = try r.array("args", allowNull: true)
    guard rawArgs.count <= Limits.maxExecArgs else {
        throw .invalid("exec.args: more than \(Limits.maxExecArgs) arguments")
    }
    var args: [String] = []
    var size = 0
    for (index, raw) in rawArgs.enumerated() {
        guard let arg = raw as? String else {
            throw .invalid("exec.args[\(index)]: the value is not a string")
        }
        size += arg.utf8.count
        guard size <= Limits.maxExecArgsBytes else {
            throw .invalid("exec.args: the arguments are longer than \(Limits.maxExecArgsBytes) bytes together")
        }
        // A line break and a tab are allowed (Go: protocol.checkArgs); the
        // view shows them as \\n and \\t.
        guard arg.unicodeScalars.allSatisfy({ $0 == "\n" || $0 == "\t" || !isHidden($0) }) else {
            throw .invalid("exec.args[\(index)]: the argument contains a control character")
        }
        args.append(arg)
    }
    let cwd = try execPath(r, "cwd")
    let rawEnv = try r.array("env")
    guard !rawEnv.isEmpty, rawEnv.count <= Limits.maxExecVariables else {
        throw .invalid("exec.env: the command needs 1 to \(Limits.maxExecVariables) variables")
    }
    let known = Set(credentials.map(\.id))
    var names: Set<String> = []
    var used: Set<String> = []
    var env: [ExecVariable] = []
    for (index, raw) in rawEnv.enumerated() {
        let v = try Reader(raw, path: "exec.env[\(index)]")
        let name = try v.string("name")
        guard isEnvName(name) else {
            throw .invalid("exec.env[\(index)].name: the name is not a valid or allowed variable name")
        }
        guard names.insert(name).inserted else {
            throw .invalid("exec.env[\(index)].name: the name appears twice")
        }
        let id = try v.id("credential_id")
        guard known.contains(id) else {
            throw .invalid("exec.env[\(index)].credential_id: the credential is not in the request")
        }
        used.insert(id)
        try v.finish()
        env.append(ExecVariable(name: name, credentialID: id))
    }
    if let index = credentials.firstIndex(where: { !used.contains($0.id) }) {
        throw .invalid("credentials[\(index)]: the credential has no variable")
    }
    let approval = try decodeExecApproval(try r.object("approval"), environment: environment, ttl: ttl)
    try r.finish()
    return ExecCommand(environment: environment, program: program, args: args, cwd: cwd, env: env, approval: approval)
}

private func decodeExecApproval(_ r: Reader, environment: ExecEnvironment, ttl: Int64) throws(ProtocolError) -> ExecApproval {
    guard let kind = ExecApprovalKind(rawValue: try r.string("kind")) else {
        throw .invalid("exec.approval.kind: the kind is not grant or once")
    }
    guard let binding = GrantBinding(rawValue: try r.string("binding")) else {
        throw .invalid("exec.approval.binding: the binding is not none, program or command")
    }
    let rawChoices = try r.optionalArray("lifetime_choices_seconds")
    try r.finish()
    switch kind {
    case .once:
        guard environment == .prod else {
            throw .invalid("exec.approval.kind: once is for prod only")
        }
        guard binding == .command else {
            throw .invalid("exec.approval.binding: once needs the binding command")
        }
        guard ttl == 0, rawChoices?.isEmpty ?? true else {
            throw .invalid("exec.approval: once has no lifetime and no lifetime choices")
        }
        return ExecApproval(kind: .once, binding: .command, lifetimeChoices: [])
    case .grant:
        guard environment != .prod else {
            throw .invalid("exec.approval.kind: prod allows only the kind once")
        }
        guard !(environment == .test && binding == .none) else {
            throw .invalid("exec.approval.binding: test does not allow the binding none")
        }
        guard ttl > 0, ttl <= Limits.maxGrantSeconds else {
            throw .invalid("grant.ttl_seconds: a grant needs a lifetime of 1 to \(Limits.maxGrantSeconds) seconds")
        }
        var choices: [Int64] = []
        for raw in rawChoices ?? [] {
            guard let value = integer(raw) else {
                throw .invalid("exec.approval.lifetime_choices_seconds: a value is not an integer")
            }
            choices.append(value)
        }
        guard choices.first == ttl,
              zip(choices, choices.dropFirst()).allSatisfy({ $1 > 0 && $1 < $0 }) else {
            throw .invalid("exec.approval.lifetime_choices_seconds: the choices are not the lifetime and shorter lifetimes in descending order")
        }
        return ExecApproval(kind: .grant, binding: binding, lifetimeChoices: choices)
    }
}

/// Reads an absolute, clean path without a control character.
private func execPath(_ r: Reader, _ key: String) throws(ProtocolError) -> String {
    let path = try r.string(key)
    guard !path.isEmpty, path.utf8.count <= Limits.maxPathBytes,
          !path.unicodeScalars.contains(where: isHidden) else {
        throw .invalid("exec.\(key): the path is empty, too long or contains a control character")
    }
    guard path.hasPrefix("/"), isCleanPath(path) else {
        throw .invalid("exec.\(key): the path is not absolute and clean")
    }
    return path
}

/// True for a path as Go path.Clean writes it: no empty element, no "."
/// and no "..", no "/" at the end (except the root).
func isCleanPath(_ path: String) -> Bool {
    if path == "/" { return true }
    let parts = path.split(separator: "/", omittingEmptySubsequences: false).dropFirst()
    return parts.allSatisfy { !$0.isEmpty && $0 != "." && $0 != ".." }
}

/// The rule of protocol.CheckEnvName (D64): [A-Z_][A-Z0-9_]*, not one of
/// the forbidden names, no forbidden prefix.
func isEnvName(_ name: String) -> Bool {
    guard let first = name.unicodeScalars.first, first == "_" || ("A"..."Z").contains(first),
          name.unicodeScalars.allSatisfy({ $0 == "_" || ("A"..."Z").contains($0) || ("0"..."9").contains($0) }) else {
        return false
    }
    let forbidden: Set<String> = ["PATH", "HOME", "USER", "LOGNAME", "SHELL", "PWD", "TMPDIR", "IFS"]
    let prefixes = ["DYLD_", "LD_", "A38_"]
    return !forbidden.contains(name) && !prefixes.contains(where: name.hasPrefix)
}

/// True for a character that must not appear in a text that a human reads
/// (Go: protocol.IsControl): a control character, a format character, a
/// line or paragraph separator, or U+FFFD.
func isHidden(_ c: Unicode.Scalar) -> Bool {
    switch c.properties.generalCategory {
    case .control, .format, .lineSeparator, .paragraphSeparator:
        return true
    default:
        return c == "\u{FFFD}"
    }
}

private func decodeExistingGrant(_ r: Reader) throws(ProtocolError) -> ExistingGrant {
    let grantID = try r.id("grant_id")
    let expiresAt = try r.time("expires_at")
    let scope = try decodeScope(try r.object("scope"))
    let count = try r.int("item_count")
    guard count >= 0 else {
        throw .invalid("existing_grant.item_count: the value is negative")
    }
    try r.finish()
    return ExistingGrant(grantID: grantID, expiresAt: expiresAt, scope: scope, itemCount: Int(count))
}

private func decodeNotice(_ root: Reader) throws(ProtocolError) -> Notice {
    let noticeID = try root.id("notice_id")
    let kindText = try root.string("kind")
    guard let kind = NoticeKind(rawValue: kindText) else {
        throw .invalid("kind: the kind of notice is not known")
    }
    let code = try root.string("code")
    guard !code.isEmpty, code.count <= 64,
          code.unicodeScalars.allSatisfy({ ("a"..."z").contains($0) || $0 == "_" }) else {
        throw .invalid("code: the value is not a code")
    }
    let project = try root.optionalText("project")
    switch kind {
    case .projectConfigInvalid where project == nil:
        throw .invalid("project: a notice about a project needs the project")
    case .daemonFault where project != nil:
        throw .invalid("project: a notice about a daemon fault names no project")
    default:
        break
    }
    let message = try root.text("message")
    let occurredAt = try root.time("occurred_at")
    try root.finish()
    return Notice(noticeID: noticeID, kind: kind, code: code, project: project,
                  message: message, occurredAt: occurredAt)
}

/// Reads one JSON object and remembers the keys that the decoder used.
/// finish() rejects each other key.
final class Reader {
    let path: String
    private let dict: [String: Any]
    private var used: Set<String> = []

    init(_ value: Any, path: String) throws(ProtocolError) {
        guard let dict = value as? [String: Any] else {
            throw .invalid("\(path): the value is not an object")
        }
        self.dict = dict
        self.path = path
    }

    private func name(_ key: String) -> String {
        path == "message" ? key : "\(path).\(key)"
    }

    private func value(_ key: String) -> Any? {
        used.insert(key)
        return dict[key]
    }

    private func required(_ key: String) throws(ProtocolError) -> Any {
        guard let v = value(key) else {
            throw .invalid("\(name(key)): the field is missing")
        }
        if v is NSNull {
            throw .invalid("\(name(key)): the field is null")
        }
        return v
    }

    func string(_ key: String) throws(ProtocolError) -> String {
        guard let s = try required(key) as? String else {
            throw .invalid("\(name(key)): the value is not a string")
        }
        guard s.utf8.count <= Limits.maxTextBytes else {
            throw .invalid("\(name(key)): the text is too long")
        }
        return s
    }

    func optionalString(_ key: String) throws(ProtocolError) -> String? {
        guard let v = value(key) else { return nil }
        guard let s = v as? String else {
            throw .invalid("\(name(key)): the value is not a string")
        }
        guard s.utf8.count <= Limits.maxTextBytes else {
            throw .invalid("\(name(key)): the text is too long")
        }
        return s
    }

    /// A required, non-empty text.
    func text(_ key: String) throws(ProtocolError) -> String {
        let s = try string(key)
        guard !s.isEmpty else {
            throw .invalid("\(name(key)): the text is empty")
        }
        return s
    }

    /// An optional text. Go leaves out an empty text (omitempty).
    func optionalText(_ key: String) throws(ProtocolError) -> String? {
        guard let s = try optionalString(key), !s.isEmpty else { return nil }
        return s
    }

    /// An ID: 1 to 128 characters from letters, digits, ".", "_", ":", "-".
    /// The app writes IDs to the standard error, so they must be plain.
    func id(_ key: String) throws(ProtocolError) -> String {
        let s = try string(key)
        guard !s.isEmpty, s.count <= Limits.maxIDLength, s.unicodeScalars.allSatisfy(isIDCharacter) else {
            throw .invalid("\(name(key)): the value is not a valid ID")
        }
        return s
    }

    func int(_ key: String) throws(ProtocolError) -> Int64 {
        guard let n = integer(try required(key)) else {
            throw .invalid("\(name(key)): the value is not an integer")
        }
        return n
    }

    func optionalInt(_ key: String) throws(ProtocolError) -> Int64? {
        guard let v = value(key) else { return nil }
        guard let n = integer(v) else {
            throw .invalid("\(name(key)): the value is not an integer")
        }
        return n
    }

    /// An optional list. A missing field and null are nil.
    func optionalArray(_ key: String) throws(ProtocolError) -> [Any]? {
        guard let v = value(key), !(v is NSNull) else { return nil }
        guard let a = v as? [Any] else {
            throw .invalid("\(name(key)): the value is not a list")
        }
        return a
    }

    func object(_ key: String) throws(ProtocolError) -> Reader {
        try Reader(try required(key), path: name(key))
    }

    func optionalObject(_ key: String) throws(ProtocolError) -> Reader? {
        guard let v = value(key) else { return nil }
        return try Reader(v, path: name(key))
    }

    func array(_ key: String, allowNull: Bool = false) throws(ProtocolError) -> [Any] {
        guard let v = value(key) else {
            throw .invalid("\(name(key)): the field is missing")
        }
        if v is NSNull, allowNull { return [] }
        guard let a = v as? [Any] else {
            throw .invalid("\(name(key)): the value is not a list")
        }
        return a
    }

    /// A time in RFC 3339, as Go writes a time.Time.
    func time(_ key: String) throws(ProtocolError) -> Date {
        guard let date = parseRFC3339(try string(key)) else {
            throw .invalid("\(name(key)): the value is not an RFC 3339 time")
        }
        return date
    }

    /// Rejects each key that the decoder did not read.
    func finish() throws(ProtocolError) {
        let unknown = Set(dict.keys).subtracting(used).sorted()
        if let first = unknown.first {
            throw .invalid("\(name(first)): the field is not known")
        }
    }
}

private func isIDCharacter(_ c: Unicode.Scalar) -> Bool {
    ("a"..."z").contains(c) || ("A"..."Z").contains(c) || ("0"..."9").contains(c)
        || c == "." || c == "_" || c == ":" || c == "-"
}

/// Returns the integer value of a JSON number. A boolean, a fraction or a
/// number out of the safe range is not an integer.
private func integer(_ value: Any) -> Int64? {
    guard let n = value as? NSNumber, CFGetTypeID(n) != CFBooleanGetTypeID() else { return nil }
    let d = n.doubleValue
    guard d.rounded() == d, abs(d) <= 9_007_199_254_740_991 else { return nil }
    return n.int64Value
}

/// Parses an RFC 3339 time with an optional fraction of any length.
func parseRFC3339(_ text: String) -> Date? {
    // ISO8601DateFormatter reads at most three digits of a fraction; Go
    // writes up to nine. Cut the fraction and add it back.
    var base = text
    var fraction = 0.0
    if let dot = text.firstIndex(of: ".") {
        var end = text.index(after: dot)
        while end < text.endIndex, text[end].isASCII, text[end].isNumber {
            end = text.index(after: end)
        }
        let digits = text[text.index(after: dot)..<end]
        guard !digits.isEmpty, let value = Double("0." + digits) else { return nil }
        fraction = value
        base = String(text[..<dot]) + String(text[end...])
    }
    let formatter = ISO8601DateFormatter()
    formatter.formatOptions = [.withInternetDateTime]
    guard let date = formatter.date(from: base) else { return nil }
    return date.addingTimeInterval(fraction)
}
