// The confirmation of an approval. The real implementation uses Touch ID
// (A38ApprovalUI.TouchIDAuthenticator). The fake exists for tests and for
// the test switches of a debug build.

/// The result of one authentication.
public enum AuthOutcome: Sendable, Equatable {
    case success
    /// The human or the app stopped the dialog. The popup stays open.
    case cancelled
    /// The policy is not available, for example Touch ID with a closed lid.
    case unavailable(String)
    /// The authentication failed.
    case failed(String)
}

/// Confirms an approval with the human.
@MainActor
public protocol Authenticator: AnyObject {
    /// The policy name for the field auth.policy of the decision.
    var policyName: String { get }
    /// Shows the dialog and returns its result. reason contains verified
    /// facts only.
    func authenticate(reason: String) async -> AuthOutcome
    /// Stops an open dialog, for example at the timeout. The open call of
    /// authenticate then returns.
    func cancel()
}

#if A38_TEST_SWITCHES
/// An authenticator without a dialog. It reports the policy "skipped".
/// Only a debug build contains it (T14); guardApproval also blocks the
/// policy "skipped" in a release build.
@MainActor
public final class FakeAuthenticator: Authenticator {
    public let policyName = skippedPolicy
    /// The result of each call.
    public var outcome: AuthOutcome
    /// If true, authenticate waits until resume(with:) or cancel().
    public var hold: Bool
    public private(set) var calls: [String] = []
    public private(set) var cancelCount = 0
    private var pending: CheckedContinuation<AuthOutcome, Never>?

    public init(outcome: AuthOutcome = .success, hold: Bool = false) {
        self.outcome = outcome
        self.hold = hold
    }

    public func authenticate(reason: String) async -> AuthOutcome {
        calls.append(reason)
        guard hold else { return outcome }
        return await withCheckedContinuation { continuation in
            pending = continuation
        }
    }

    /// Ends a held call with a result.
    public func resume(with outcome: AuthOutcome) {
        pending?.resume(returning: outcome)
        pending = nil
    }

    public var isWaiting: Bool { pending != nil }

    public func cancel() {
        cancelCount += 1
        resume(with: .cancelled)
    }
}
#endif
