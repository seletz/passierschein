// All texts that the human sees: popup, notice and the Touch ID dialog.
//
// The texts are English, as in the accepted prototype. The language of the
// product is open (G4); a translation changes this file only. Diagnostics
// on the standard error and the error messages of a decision are not texts
// for the human and stay in the code.
import Foundation

public enum UIText {
    // MARK: Window

    public static let windowTitle = "Passierschein"
    /// The name of the app at the right of the banner.
    public static let brand = "Passierschein"

    // MARK: Banner

    public static let bannerFirstTitle = "Credential request"
    public static let bannerFirstDetail = "First request of this session."
    public static let bannerDeviationTitle = "Deviation from active grant"
    public static let bannerDeviationDetail = "This request goes beyond the active grant of this session."

    // MARK: Verified facts

    public static let verifiedHeading = "Verified by the daemon"
    public static let factProject = "Project"
    public static let factSession = "Session ID"
    public static let factOrigin = "Origin"
    public static let originLocal = "Local process"
    public static let originVM = "VM"
    public static let factProcess = "Process"
    public static let factDirectory = "Directory"
    public static let factActiveGrant = "Active grant"

    public static func processLine(path: String?, pid: Int?, uid: Int) -> String {
        var details: [String] = []
        if let pid { details.append("PID \(pid)") }
        details.append("UID \(uid)")
        let joined = details.joined(separator: ", ")
        guard let path else { return joined }
        return "\(path) (\(joined))"
    }

    public static func activeGrantLine(grantID: String, itemCount: Int, vaults: [String], until: String) -> String {
        let items = itemCount == 1 ? "1 credential" : "\(itemCount) credentials"
        let where_ = vaults.isEmpty ? "" : " in \(vaults.joined(separator: ", "))"
        return "\(grantID): \(items)\(where_), valid until \(until)"
    }

    // MARK: Claims

    public static let claimsHeading = "Claims of the agent. NOT verified."
    public static let claimSession = "Session"
    public static let claimAgent = "Agent"
    public static let claimTask = "Task"
    public static let claimReason = "Reason"
    /// The mark for a claim that the agent did not give. The app never
    /// invents a value.
    public static let claimMissing = "(not given)"

    // MARK: Credentials

    public static let credentialsHeading = "Credentials"
    public static func selectedCount(_ selected: Int, of total: Int) -> String {
        "\(selected) of \(total) selected"
    }
    public static func selectAll(_ count: Int) -> String { "Select all \(count)" }
    public static func fieldLabel(_ field: String) -> String { "field \(field)" }
    /// The hint below a list that scrolls.
    public static func listScrolls(credentials: Int, vaults: Int) -> String {
        "Scroll the list: \(credentials) credentials in \(vaults) vault\(vaults == 1 ? "" : "s")."
    }
    public static let deviationNewItem = "NEW ITEM"
    public static let deviationNewVault = "NEW VAULT"

    // MARK: Grant and lifetime

    public static let lifetimeLabel = "Valid for"
    public static let lifetimeProposed = "proposed"
    public static let lifetimeShorter = "Shorter…"
    public static let lifetimeKeep = "Done"
    public static func lifetimeChoice(_ text: String, proposed: Bool) -> String {
        proposed ? "\(text) (\(lifetimeProposed))" : text
    }
    /// The text about the scope of the grant (D32, G26): a grant covers
    /// exactly the selected entries, never a whole vault.
    public static func grantScope(project: String) -> String {
        "The grant covers only the selected credentials, for this session and project \(project)."
    }

    // MARK: Footer

    public static let deny = "Deny (Esc)"
    public static func closesIn(_ seconds: Int) -> String {
        "Closes in \(seconds) s. No decision means no approval."
    }
    public static let confirmWaiting = "Read the request…"
    public static let confirmSelect = "Select credentials"
    public static let confirmInProgress = "Touch ID…"
    public static func confirm(_ count: Int) -> String { "Approve \(count) with Touch ID" }

    // MARK: Touch ID

    /// The system shows: "<app name>" is trying to <reason>. The reason has
    /// verified facts only, never a claim of the agent.
    ///
    /// macOS builds the dialog from its own frame in the language of the
    /// system and from this text. The text and the cancel title follow the
    /// language of the system, so that the dialog has one language. The
    /// popup itself stays English.
    public static func authReason(count: Int, project: String,
                                  language: SystemLanguage = .current) -> String {
        switch language {
        case .german:
            return "\(count) \(count == 1 ? "Zugangsdatum" : "Zugangsdaten") für das Projekt \(project) freigeben"
        case .english:
            return "approve \(count) credential\(count == 1 ? "" : "s") for project \(project)"
        }
    }
    public static func authCancelTitle(language: SystemLanguage = .current) -> String {
        switch language {
        case .german: return "Zurück"
        case .english: return "Back"
        }
    }
    public static let authStopped = "Touch ID was stopped. Try again or deny the request."

    // MARK: Notice

    public static let noticeDaemonFaultTitle = "Passierschein: daemon fault"
    public static let noticeDaemonFaultDetail = "a38 delivers no credentials to any session until you lift the lock."
    public static let noticeProjectTitle = "Passierschein: invalid project configuration"
    public static func noticeProjectDetail(_ project: String) -> String {
        "a38 rejects all requests of project \(project). Other projects keep working."
    }
    public static let noticeWhatHappened = "Message of the daemon"
    public static let noticeCode = "Code"
    public static let noticeProject = "Project"
    public static let noticeTime = "Time"
    public static let noticeID = "Notice"
    public static let noticeNextSteps = "Next steps"
    public static let noticeDaemonFaultSteps = [
        "Run a38 doctor to find the cause.",
        "Remove the cause.",
        "Lift the lock with a38 fault clear, or restart a38 serve.",
    ]
    public static let noticeProjectSteps = [
        "Run a38 doctor to see the finding.",
        "Correct the project in the configuration file.",
        "Restart a38 serve.",
    ]
    public static let noticeInfoOnly = "This notice informs only. It changes nothing."
    public static let noticeClose = "Close"

    // MARK: Durations

    /// A lifetime as text: "4 h", "1 h 30 min", "15 min", "45 s".
    public static func duration(_ seconds: Int64) -> String {
        if seconds < 60 { return "\(seconds) s" }
        let hours = seconds / 3600
        let minutes = (seconds % 3600) / 60
        let rest = seconds % 60
        var parts: [String] = []
        if hours > 0 { parts.append("\(hours) h") }
        if minutes > 0 { parts.append("\(minutes) min") }
        if rest > 0 { parts.append("\(rest) s") }
        return parts.joined(separator: " ")
    }
}

/// The language of the system dialog of Touch ID. Only the texts that the
/// app gives to that dialog depend on it.
public enum SystemLanguage: Sendable, Equatable {
    case german
    case english

    /// German if the first preferred language of the user is German, else
    /// English.
    public static var current: SystemLanguage {
        from(preferredLanguages: Locale.preferredLanguages)
    }

    public static func from(preferredLanguages: [String]) -> SystemLanguage {
        guard let first = preferredLanguages.first?.lowercased() else { return .english }
        return first == "de" || first.hasPrefix("de-") || first.hasPrefix("de_") ? .german : .english
    }
}
