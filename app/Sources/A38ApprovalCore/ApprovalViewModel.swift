// The state and the rules of the approval popup (D31, D7, D4).
//
// The view only shows this state and calls the actions. All rules against
// an accidental approval live here, so that the tests check them without a
// window:
// - no entry is selected at the start;
// - the confirm action is active only after the delay, with a selection,
//   and not while Touch ID runs;
// - only a passed authentication gives an approval;
// - the timeout gives the outcome timeout and stops an open Touch ID dialog.
import Foundation
import Observation

/// The kind of popup. The app derives it from the data: the object exec
/// marks a command of the exec way (D65), existing_grant marks a
/// deviation. A plain request outside dev never reaches the app (D62).
public enum PopupCase: Sendable, Equatable {
    case firstRequest
    case deviation
    /// A command of the exec way in the strictest environment of its
    /// credentials.
    case command(ExecEnvironment)
}

/// One credential of a command with the names of its variables.
public struct VariableRow: Sendable, Equatable, Identifiable {
    public var credential: Credential
    /// The names of the variables that get the value, in the order of the
    /// request.
    public var names: [String]
    /// "vault › item › field" with the names of the backend.
    public var reference: String
    public var id: String { credential.id }
}

/// One line of the verified facts.
public struct FactRow: Sendable, Equatable, Identifiable {
    public var label: String
    public var value: String
    public var id: String { label }
}

/// One line of the claims of the agent.
public struct ClaimRow: Sendable, Equatable, Identifiable {
    public var label: String
    public var value: String
    /// True if the agent did not give the claim.
    public var missing: Bool
    /// The view never cuts this line (the session name, D25). The view
    /// cuts each other claim to three lines.
    public var unabridged: Bool
    public var id: String { label }
}

/// The entries of one vault, in the order of the request.
public struct VaultGroup: Sendable, Equatable, Identifiable {
    public var vault: String
    public var items: [Credential]
    /// "Select all n" exists only for a dev vault with more than three
    /// entries. There is no control for all vaults together.
    public var offersSelectAll: Bool
    public var id: String { vault }
}

@MainActor
@Observable
public final class ApprovalViewModel {
    /// The delay before the confirm action becomes active (D31).
    public static let armDelay: TimeInterval = 1.5
    /// More entries than this make the list scroll.
    public static let scrollThreshold = 6
    /// A vault needs more entries than this for "Select all n".
    public static let selectAllThreshold = 3
    /// A command with more characters than this scrolls (D65: no cut).
    public static let commandScrollThreshold = 480

    public let request: ApprovalRequest
    /// The time from the start to the outcome timeout.
    public let timeout: TimeInterval
    public let armDelay: TimeInterval
    public let lifetimeChoices: [Int64]

    @ObservationIgnored private let authenticator: any Authenticator
    @ObservationIgnored private let clock: () -> TimeInterval
    @ObservationIgnored private let startedAt: TimeInterval
    @ObservationIgnored private let timeZone: TimeZone
    @ObservationIgnored public var onFinish: (Decision) -> Void = { _ in }

    public private(set) var selected: Set<String> = []
    public private(set) var lifetime: Int64
    public private(set) var lifetimeChoiceOpen = false
    public private(set) var armed = false
    public private(set) var secondsLeft: Int
    public private(set) var authInProgress = false
    public private(set) var message: String?
    public private(set) var decision: Decision?

    /// - Parameters:
    ///   - startedAt: the start of the app on the clock; elapsed_ms counts
    ///     from there.
    ///   - timeoutOverride: a shorter timeout (test switch --auto-close).
    ///   - clock: a monotonic clock in seconds.
    public init(request: ApprovalRequest, authenticator: any Authenticator,
                startedAt: TimeInterval, clock: @escaping () -> TimeInterval,
                timeoutOverride: TimeInterval? = nil, armDelay: TimeInterval = ApprovalViewModel.armDelay,
                timeZone: TimeZone = .current) {
        self.request = request
        self.authenticator = authenticator
        self.startedAt = startedAt
        self.clock = clock
        self.timeZone = timeZone
        let requested = TimeInterval(request.timeoutSeconds)
        self.timeout = min(requested, timeoutOverride ?? requested)
        self.armDelay = armDelay
        // A command offers the lifetimes of the daemon; "only this command"
        // offers none (D63). A plain request keeps the fixed set of D32.
        if let exec = request.exec {
            self.lifetimeChoices = exec.approval.lifetimeChoices
        } else {
            self.lifetimeChoices = A38ApprovalCore.lifetimeChoices(proposal: request.grant.ttlSeconds)
        }
        self.lifetime = request.grant.ttlSeconds
        self.secondsLeft = Int(timeout.rounded(.up))
    }

    // MARK: Derived state

    public var popupCase: PopupCase {
        if let exec = request.exec { return .command(exec.environment) }
        return request.existingGrant == nil ? .firstRequest : .deviation
    }

    /// True for a command in test or prod: the window highlights the
    /// environment (D65).
    public var environmentHighlighted: Bool {
        guard let exec = request.exec else { return false }
        return exec.environment != .dev
    }

    /// Program, arguments and working directory, in full (D65). Empty for
    /// a plain request.
    public var commandRows: [FactRow] {
        guard let exec = request.exec else { return [] }
        return [
            FactRow(label: UIText.commandProgram, value: DisplayText.clean(exec.program)),
            FactRow(label: UIText.commandArguments,
                    value: exec.args.isEmpty ? UIText.commandNoArguments : DisplayText.shellWords(exec.args)),
            FactRow(label: UIText.commandDirectory, value: DisplayText.clean(exec.cwd)),
        ]
    }

    /// The number of characters of the command.
    public var commandLength: Int { commandRows.reduce(0) { $0 + $1.value.count } }

    /// A long command scrolls; the view never cuts it.
    public var commandScrolls: Bool { commandLength > Self.commandScrollThreshold }

    /// The credentials of a command, each with its variables, in the order
    /// of the variables. Empty for a plain request.
    public var variableRows: [VariableRow] {
        guard let exec = request.exec else { return [] }
        var order: [String] = []
        var names: [String: [String]] = [:]
        for v in exec.env {
            if names[v.credentialID] == nil { order.append(v.credentialID) }
            names[v.credentialID, default: []].append(v.name)
        }
        return order.compactMap { id in
            guard let c = request.credentials.first(where: { $0.id == id }) else { return nil }
            let reference = [c.vault, c.title, c.field].map(DisplayText.clean).joined(separator: " › ")
            return VariableRow(credential: c, names: names[id] ?? [], reference: reference)
        }
    }

    /// True if the human can select a shorter lifetime.
    public var offersLifetimeChoice: Bool { lifetimeChoices.count > 1 }

    /// The text of the binding of a grant for a command (D63).
    public var bindingText: String {
        switch request.exec?.approval.binding {
        case .none?: return UIText.bindingNone
        case .program?: return UIText.bindingProgram
        case .command?: return UIText.bindingCommand
        case nil: return ""
        }
    }

    /// The kind of approval of a command: the lifetime of the grant, or
    /// "only this command" (D65).
    public var approvalText: String {
        if request.exec?.approval.kind == .once { return UIText.approvalOnce }
        return lifetimeText
    }

    /// True for a command: every variable must be selected.
    private var needsAll: Bool { request.exec != nil }

    private var allSelected: Bool { selected.count == request.credentials.count }

    public var verifiedRows: [FactRow] {
        let facts = request.verified
        var rows = [
            FactRow(label: UIText.factProject, value: DisplayText.clean(facts.project)),
            FactRow(label: UIText.factSession, value: facts.sessionID),
        ]
        switch facts.origin {
        case .local: rows.append(FactRow(label: UIText.factOrigin, value: UIText.originLocal))
        case .vm: rows.append(FactRow(label: UIText.factOrigin, value: UIText.originVM))
        }
        rows.append(FactRow(label: UIText.factProcess,
                            value: UIText.processLine(path: facts.process.map(DisplayText.clean),
                                                      pid: facts.peerPID, uid: facts.peerUID)))
        if let cwd = facts.cwd {
            rows.append(FactRow(label: UIText.factDirectory, value: DisplayText.clean(cwd)))
        }
        if let grant = request.existingGrant {
            rows.append(FactRow(label: UIText.factActiveGrant,
                                value: UIText.activeGrantLine(grantID: grant.grantID, itemCount: grant.itemCount,
                                                              vaults: grant.scope.vaults.map(DisplayText.clean),
                                                              until: DisplayText.time(grant.expiresAt, timeZone: timeZone))))
        }
        return rows
    }

    public var claimRows: [ClaimRow] {
        let claims = request.claims
        func row(_ label: String, _ value: String?, unabridged: Bool = false) -> ClaimRow {
            guard let value else {
                return ClaimRow(label: label, value: UIText.claimMissing, missing: true, unabridged: unabridged)
            }
            return ClaimRow(label: label, value: DisplayText.clean(value), missing: false, unabridged: unabridged)
        }
        return [
            row(UIText.claimSession, claims.sessionName, unabridged: true),
            row(UIText.claimAgent, claims.agent),
            row(UIText.claimTask, claims.task),
            row(UIText.claimReason, claims.reason),
        ]
    }

    public var groups: [VaultGroup] {
        var order: [String] = []
        var byVault: [String: [Credential]] = [:]
        for credential in request.credentials {
            if byVault[credential.vault] == nil { order.append(credential.vault) }
            byVault[credential.vault, default: []].append(credential)
        }
        return order.map { vault in
            let items = byVault[vault] ?? []
            let allDev = items.allSatisfy { $0.environment == "dev" }
            return VaultGroup(vault: vault, items: items,
                              offersSelectAll: allDev && items.count > Self.selectAllThreshold)
        }
    }

    public var listScrolls: Bool { request.credentials.count > Self.scrollThreshold }

    public var selectedCountText: String {
        UIText.selectedCount(selected.count, of: request.credentials.count)
    }

    public var canConfirm: Bool {
        decision == nil && armed && !authInProgress && !selected.isEmpty && (!needsAll || allSelected)
    }

    /// The selection can change until Touch ID starts.
    public var canEditSelection: Bool { decision == nil && !authInProgress }

    public var confirmTitle: String {
        if authInProgress { return UIText.confirmInProgress }
        if !armed { return UIText.confirmWaiting }
        if needsAll {
            return allSelected ? UIText.confirmCommand : UIText.confirmSelectAll
        }
        if selected.isEmpty { return UIText.confirmSelect }
        return UIText.confirm(selected.count)
    }

    public var lifetimeText: String {
        UIText.lifetimeChoice(UIText.duration(lifetime), proposed: lifetime == request.grant.ttlSeconds)
    }

    public var scopeText: String {
        UIText.grantScope(project: DisplayText.clean(request.verified.project))
    }

    public var closesInText: String { UIText.closesIn(secondsLeft) }

    /// The selected IDs in the order of the request.
    public var selectedIDs: [String] {
        request.credentials.map(\.id).filter(selected.contains)
    }

    // MARK: Actions

    public func setSelected(_ id: String, _ isOn: Bool) {
        guard canEditSelection, request.credentials.contains(where: { $0.id == id }) else { return }
        if isOn { selected.insert(id) } else { selected.remove(id) }
    }

    public func isSelected(_ id: String) -> Bool { selected.contains(id) }

    /// Selects all entries of a vault. It does nothing for a vault without
    /// "Select all n".
    public func selectAll(vault: String) {
        guard canEditSelection, let group = groups.first(where: { $0.vault == vault }),
              group.offersSelectAll else { return }
        selected.formUnion(group.items.map(\.id))
    }

    public func openLifetimeChoice() {
        guard canEditSelection, offersLifetimeChoice else { return }
        lifetimeChoiceOpen = true
    }

    public func closeLifetimeChoice() { lifetimeChoiceOpen = false }

    /// Selects a lifetime. Only a value of the fixed set is possible; it is
    /// never longer than the proposal.
    public func selectLifetime(_ seconds: Int64) {
        guard canEditSelection, lifetimeChoices.contains(seconds) else { return }
        lifetime = seconds
    }

    /// Updates the delay and the remaining time. At the end of the time the
    /// popup closes with the outcome timeout, also during Touch ID.
    public func tick() {
        guard decision == nil else { return }
        let elapsed = clock() - startedAt
        if !armed && elapsed >= armDelay { armed = true }
        let left = timeout - elapsed
        secondsLeft = max(0, Int(left.rounded(.up)))
        if left <= 0 {
            if authInProgress { authenticator.cancel() }
            finish(Decision(requestID: request.requestID, outcome: .timeout, elapsedMS: elapsedMS()))
        }
    }

    /// The human denies: Deny button or Esc. Deny needs no delay.
    public func deny() {
        guard decision == nil else { return }
        if authInProgress { authenticator.cancel() }
        finish(Decision(requestID: request.requestID, outcome: .denied, elapsedMS: elapsedMS()))
    }

    /// The human confirms with a click. Only a passed authentication gives
    /// an approval. The selection and the lifetime are those at the click.
    public func confirm() async {
        tick()
        guard canConfirm else { return }
        let ids = selectedIDs
        // "Only this command" has no lifetime (D63: lifetime 0).
        let ttl: Int64? = request.exec?.approval.kind == .once ? nil : lifetime
        authInProgress = true
        lifetimeChoiceOpen = false
        message = nil
        let project = DisplayText.clean(request.verified.project)
        let reason = request.exec.map {
            UIText.authReasonCommand(count: ids.count, environment: $0.environment, project: project)
        } ?? UIText.authReason(count: ids.count, project: project)
        let outcome = await authenticator.authenticate(reason: reason)
        authInProgress = false
        // The timeout or a denial can end the popup during the dialog.
        guard decision == nil else { return }
        let policy = authenticator.policyName
        switch outcome {
        case .success:
            finish(Decision(requestID: request.requestID, outcome: .approved, approvedCredentialIDs: ids,
                            ttlSeconds: ttl, auth: AuthResult(policy: policy, ok: true), elapsedMS: elapsedMS()))
        case .cancelled:
            message = UIText.authStopped
        case .unavailable(let text):
            finish(.failure(.authUnavailable, text, requestID: request.requestID,
                            auth: AuthResult(policy: policy, ok: false), elapsedMS: elapsedMS()))
        case .failed(let text):
            finish(.failure(.authFailed, text, requestID: request.requestID,
                            auth: AuthResult(policy: policy, ok: false), elapsedMS: elapsedMS()))
        }
    }

    #if A38_TEST_SWITCHES
    /// Test switch --demo-scroll-end: the view scrolls the list to the end.
    public var demoScrollToEnd = false

    /// Test switch of a debug build: puts the popup in a state for a
    /// screenshot. A release build does not contain this function.
    public func applyDemo(select ids: [String], lifetimeOpen: Bool) {
        for id in ids { setSelected(id, true) }
        if lifetimeOpen { openLifetimeChoice() }
    }
    #endif

    private func elapsedMS() -> Int64 {
        Int64(((clock() - startedAt) * 1000).rounded())
    }

    private func finish(_ result: Decision) {
        guard decision == nil else { return }
        decision = result
        onFinish(result)
    }
}
