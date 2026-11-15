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

    // MARK: Banner of a command (exec way, D65)

    public static let bannerCommandTitle = "Run a command with credentials"
    public static func bannerCommandDetail(_ environment: ExecEnvironment) -> String {
        switch environment {
        case .dev: return "Environment dev."
        case .test: return "Environment TEST. The command gets credentials of test."
        case .prod: return "Environment PROD. You approve only this one command."
        }
    }
    /// The mark of the environment in the banner and in the facts.
    public static func environmentBadge(_ environment: ExecEnvironment) -> String {
        environment.rawValue.uppercased()
    }

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
    public static let factEnvironment = "Environment"

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

    // MARK: Command and variables (exec way, D65)

    public static let commandHeading = "Command"
    public static let commandNote = "a38 mcp starts exactly this program with these arguments."
    public static let commandProgram = "Program"
    public static let commandArguments = "Arguments"
    public static let commandDirectory = "Directory"
    public static let commandNoArguments = "(none)"
    /// The hint below a command that scrolls.
    public static func commandScrolls(arguments: Int, characters: Int) -> String {
        "Scroll to read the whole command: \(arguments) argument\(arguments == 1 ? "" : "s"), \(characters) characters."
    }
    public static let variablesHeading = "Variables"
    public static func variablesCount(_ selected: Int, of total: Int) -> String {
        "\(selected) of \(total) selected"
    }

    // MARK: Kind of approval of a command (D63, D65)

    public static let approvalOnce = "Only this command, one time"
    public static let approvalOnceLabel = "Approval"
    public static let approvalOnceDetail = "No lifetime and no grant. The next command needs a new approval."
    public static let bindingNone = "Applies to: these credentials, with any command and as MCP answer."
    public static let bindingProgram = "Applies to: this program with any arguments."
    public static let bindingCommand = "Applies to: only this exact command (program, arguments, directory)."

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
    public static let confirmSelectAll = "Select every variable"
    public static let confirmCommand = "Run with Touch ID"

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
    /// The reason for a command (D65). It names verified facts only: the
    /// number of credentials, the environment and the project.
    public static func authReasonCommand(count: Int, environment: ExecEnvironment, project: String,
                                         language: SystemLanguage = .current) -> String {
        switch language {
        case .german:
            return "Kommando mit \(count) \(count == 1 ? "Zugangsdatum" : "Zugangsdaten") aus \(environment.rawValue) für das Projekt \(project) freigeben"
        case .english:
            return "run a command with \(count) credential\(count == 1 ? "" : "s") of \(environment.rawValue) for project \(project)"
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
