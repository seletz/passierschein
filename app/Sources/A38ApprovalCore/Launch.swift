// The start of the app: options, input, validation. Each error ends the
// app before a window exists.
import Foundation

/// What the app does after the start.
public enum Launch: Sendable, Equatable {
    /// End at once: an optional decision line, a diagnostic, an exit code.
    case exit(stdout: String?, stderr: String, code: ExitCode)
    /// Show the popup for a valid request.
    case approval(ApprovalRequest, Options)
    /// Show a valid notice.
    case notice(Notice, Options)
}

/// The version of the app. The build of the bundle sets the same value in
/// Info.plist.
public let appVersion = "0.1.0"

/// Reads the options and the one message and checks them.
///
/// - Parameters:
///   - arguments: the arguments without the program name.
///   - readInput: reads the standard input; returns nil if the input is
///     larger than the limit.
public func prepare(arguments: [String], testSwitches: Bool = TestSwitches.compiledIn,
                    readInput: () -> Data?) -> Launch {
    let options: Options
    do {
        options = try parseOptions(arguments, testSwitches: testSwitches)
    } catch {
        let decision = Decision.failure(.internal, "usage: \(error.message)", elapsedMS: 0)
        return .exit(stdout: encodeDecision(decision), stderr: "\(error.message)\n\(usageText)", code: .usage)
    }
    if options.showHelp {
        return .exit(stdout: nil, stderr: usageText, code: .success)
    }
    if options.showVersion {
        return .exit(stdout: nil, stderr: "Passierschein \(appVersion)", code: .success)
    }

    var input: Data?
    #if A38_TEST_SWITCHES
    if let path = options.requestPath {
        guard let data = FileManager.default.contents(atPath: path) else {
            let decision = Decision.failure(.internal, "usage: the app cannot read the request file", elapsedMS: 0)
            return .exit(stdout: encodeDecision(decision), stderr: "the app cannot read the request file", code: .usage)
        }
        input = data.count > Limits.maxMessageBytes ? nil : data
    } else {
        input = readInput()
    }
    #else
    input = readInput()
    #endif
    guard let data = input else {
        let error = ProtocolError(code: .invalidRequest, message: "the message is larger than \(Limits.maxMessageBytes) bytes")
        return invalid(error)
    }
    do {
        switch try decodeMessage(data) {
        case .request(let request): return .approval(request, options)
        case .notice(let notice): return .notice(notice, options)
        }
    } catch {
        return invalid(error)
    }
}

private func invalid(_ error: ProtocolError) -> Launch {
    let decision = Decision.failure(error.code, error.message, requestID: error.requestID, elapsedMS: 0)
    let id = error.requestID.map { "request \($0): " } ?? ""
    return .exit(stdout: encodeDecision(decision),
                 stderr: "\(id)the message is not valid: \(error.message)", code: .invalidRequest)
}

/// Reads the standard input up to the limit. Returns nil if the input is
/// larger than the limit.
public func readStandardInput(limit: Int = Limits.maxMessageBytes) -> Data? {
    var data = Data()
    let handle = FileHandle.standardInput
    while data.count <= limit {
        guard let chunk = try? handle.read(upToCount: 64 * 1024), !chunk.isEmpty else { break }
        data.append(chunk)
    }
    return data.count > limit ? nil : data
}

/// Writes a diagnostic line to the standard error. It contains IDs and
/// codes only, never a text of the request.
public func diagnose(_ text: String) {
    FileHandle.standardError.write(Data("Passierschein: \(text)\n".utf8))
}

/// Writes the decision as the one line on the standard output and ends the
/// app with its exit code. The last barrier (guardApproval) runs here.
public func emit(_ decision: Decision, request: ApprovalRequest?) -> Never {
    let checked = guardApproval(decision, request: request)
    let code = ExitCode.of(checked)
    FileHandle.standardOutput.write(Data((encodeDecision(checked) + "\n").utf8))
    let id = checked.requestID ?? "-"
    var detail = ""
    if let error = checked.error { detail = ", error \(error.code.rawValue): \(error.message)" }
    diagnose("request \(id): outcome \(checked.outcome.rawValue), exit \(code.rawValue)\(detail)")
    exit(code.rawValue)
}

/// Ends the app after a launch error.
public func finishLaunch(stdout: String?, stderr: String, code: ExitCode) -> Never {
    if let stdout { FileHandle.standardOutput.write(Data((stdout + "\n").utf8)) }
    FileHandle.standardError.write(Data((stderr + "\n").utf8))
    exit(code.rawValue)
}

/// The monotonic clock of the app in seconds.
public func monotonicNow() -> TimeInterval { ProcessInfo.processInfo.systemUptime }

#if A38_TEST_SWITCHES
/// Test switch --no-window: runs the request without AppKit. The process
/// tests of T14 use it. A release build does not contain it.
@MainActor
public func runHeadless(request: ApprovalRequest, options: Options, startedAt: TimeInterval) -> Never {
    let authenticator = FakeAuthenticator(outcome: options.fakeAuth ?? .cancelled)
    let model = ApprovalViewModel(request: request, authenticator: authenticator, startedAt: startedAt,
                                  clock: monotonicNow, timeoutOverride: options.autoClose,
                                  armDelay: options.demoArmDelay ?? ApprovalViewModel.armDelay)
    model.onFinish = { emit($0, request: request) }
    let ids = options.demoSelect == ["all"] ? request.credentials.map(\.id) : options.demoSelect
    model.applyDemo(select: ids, lifetimeOpen: options.demoLifetimeOpen)
    var confirmed = false
    let timer = Timer(timeInterval: 0.05, repeats: true) { _ in
        MainActor.assumeIsolated {
            model.tick()
            if options.demoConfirm, !confirmed, model.canConfirm {
                confirmed = true
                Task { @MainActor in await model.confirm() }
            }
        }
    }
    RunLoop.main.add(timer, forMode: .common)
    while true { RunLoop.main.run(mode: .default, before: .distantFuture) }
}

/// Test switch --no-window for a notice: closes after --auto-close.
@MainActor
public func runHeadlessNotice(_ notice: Notice, options: Options) -> Never {
    let model = NoticeViewModel(notice: notice)
    model.onClose = {
        diagnose("notice \(notice.noticeID): closed")
        exit(ExitCode.success.rawValue)
    }
    Thread.sleep(forTimeInterval: options.autoClose ?? 0)
    model.close()
    exit(ExitCode.success.rawValue)
}
#endif
