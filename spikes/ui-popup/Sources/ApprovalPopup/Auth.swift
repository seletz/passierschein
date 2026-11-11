// Touch ID confirmation with LocalAuthentication.
import Foundation
import LocalAuthentication

enum AuthPolicy: String {
    case biometrics
    case biometricsOrCompanion = "biometrics_or_companion"
    case owner

    var laPolicy: LAPolicy {
        switch self {
        case .biometrics: return .deviceOwnerAuthenticationWithBiometrics
        case .biometricsOrCompanion: return .deviceOwnerAuthenticationWithBiometricsOrCompanion
        case .owner: return .deviceOwnerAuthentication
        }
    }
}

enum AuthOutcome {
    case success
    /// The user or the system stopped the dialog. The popup stays open.
    case cancelled(String)
    /// The policy is not available. The app cannot continue.
    case unavailable(String)
    case failed(String)
}

/// Shows the system dialog. The system adds the text after: "<app name>" is trying to ...
func authenticate(policy: AuthPolicy, reason: String, completion: @escaping (AuthOutcome) -> Void) {
    let context = LAContext()
    // An empty title hides the "Use Password" button.
    context.localizedFallbackTitle = ""
    context.localizedCancelTitle = "Back"
    // Do not accept an earlier Touch ID match, for example from the device unlock.
    context.touchIDAuthenticationAllowableReuseDuration = 0

    var probeError: NSError?
    guard context.canEvaluatePolicy(policy.laPolicy, error: &probeError) else {
        completion(.unavailable(probeError?.localizedDescription ?? "The policy is not available."))
        return
    }
    context.evaluatePolicy(policy.laPolicy, localizedReason: reason) { success, error in
        DispatchQueue.main.async {
            if success {
                completion(.success)
                return
            }
            let message = error?.localizedDescription ?? "Unknown error"
            switch (error as? LAError)?.code {
            case .userCancel, .systemCancel, .appCancel, .userFallback:
                completion(.cancelled(message))
            case .biometryNotAvailable, .biometryNotEnrolled, .biometryLockout, .passcodeNotSet,
                 .companionNotAvailable:
                completion(.unavailable(message))
            default:
                completion(.failed(message))
            }
        }
    }
}

/// Reads the availability of a policy. This function shows no dialog.
func authAvailability(policy: AuthPolicy) -> [String: Any] {
    let context = LAContext()
    var error: NSError?
    let ok = context.canEvaluatePolicy(policy.laPolicy, error: &error)
    return [
        "policy": policy.rawValue,
        "available": ok,
        "biometry_type": context.biometryType.rawValue,
        "error": error?.localizedDescription ?? "",
    ]
}
