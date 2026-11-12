// The real confirmation with Touch ID (D31, spike report "Touch ID").
//
// Policy: .deviceOwnerAuthenticationWithBiometrics only, no fall back to
// the password. No test calls this type; the real dialog is a manual test
// with Stefan (G4).
import A38ApprovalCore
import Foundation
import LocalAuthentication

@MainActor
public final class TouchIDAuthenticator: Authenticator {
    public let policyName = biometricsPolicy
    private var context: LAContext?

    public init() {}

    public func authenticate(reason: String) async -> AuthOutcome {
        let context = LAContext()
        // An empty title hides the button "Use Password".
        context.localizedFallbackTitle = ""
        context.localizedCancelTitle = UIText.authCancelTitle()
        // Do not accept an earlier match, for example from the unlock.
        context.touchIDAuthenticationAllowableReuseDuration = 0
        self.context = context
        defer { self.context = nil }

        // The availability can change: check it directly before the dialog.
        var probeError: NSError?
        guard context.canEvaluatePolicy(.deviceOwnerAuthenticationWithBiometrics, error: &probeError) else {
            return .unavailable(probeError.map(Self.describe) ?? "Touch ID is not available")
        }
        let result: (Bool, NSError?) = await withCheckedContinuation { continuation in
            context.evaluatePolicy(.deviceOwnerAuthenticationWithBiometrics, localizedReason: reason) { ok, error in
                continuation.resume(returning: (ok, error as NSError?))
            }
        }
        if result.0 { return .success }
        guard let error = result.1 else { return .failed("Touch ID failed without an error") }
        switch LAError.Code(rawValue: error.code) {
        case .userCancel?, .systemCancel?, .appCancel?, .userFallback?:
            return .cancelled
        case .biometryNotAvailable?, .biometryNotEnrolled?, .biometryLockout?, .passcodeNotSet?:
            return .unavailable(Self.describe(error))
        default:
            return .failed(Self.describe(error))
        }
    }

    /// Stops an open dialog (timeout or denial).
    public func cancel() {
        context?.invalidate()
    }

    /// A short text for the log of the daemon: the LAError code only.
    private static func describe(_ error: NSError) -> String {
        "LocalAuthentication error \(error.code)"
    }
}
