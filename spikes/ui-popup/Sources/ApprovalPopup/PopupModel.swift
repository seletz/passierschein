// State of the popup.
import Foundation
import SwiftUI

enum PopupCase {
    case initial
    case deviation
    case outsideDev
}

final class PopupModel: ObservableObject {
    let request: ApprovalRequest
    let timeout: TimeInterval
    let started = Date()

    @Published var selected: Set<String> = []
    @Published var acknowledgedOutsideDev = false
    @Published var secondsLeft: Int
    @Published var armed = false
    @Published var authInProgress = false
    @Published var message: String?

    var onApprove: () -> Void = {}
    var onDeny: () -> Void = {}

    init(request: ApprovalRequest, timeout: TimeInterval) {
        self.request = request
        self.timeout = timeout
        self.secondsLeft = Int(timeout.rounded(.up))
    }

    var hasOutsideDev: Bool { request.credentials.contains { !$0.isDev } }

    var popupCase: PopupCase {
        if hasOutsideDev { return .outsideDev }
        if request.existingGrant != nil { return .deviation }
        return .initial
    }

    /// Delay before the confirm control becomes active.
    var armDelay: TimeInterval { hasOutsideDev ? 3.0 : 1.5 }

    var canConfirm: Bool {
        armed && !authInProgress && !selected.isEmpty && (!hasOutsideDev || acknowledgedOutsideDev)
    }

    /// Credentials in groups by vault. The order of the request stays the same.
    var groups: [(vault: String, items: [CredentialRef])] {
        var order: [String] = []
        var byVault: [String: [CredentialRef]] = [:]
        for credential in request.credentials {
            if byVault[credential.vault] == nil { order.append(credential.vault) }
            byVault[credential.vault, default: []].append(credential)
        }
        return order.map { ($0, byVault[$0] ?? []) }
    }

    var elapsedMs: Int { Int(Date().timeIntervalSince(started) * 1000) }

    func tick() {
        let elapsed = Date().timeIntervalSince(started)
        if !armed && elapsed >= armDelay { armed = true }
        secondsLeft = max(0, Int((timeout - elapsed).rounded(.up)))
    }
}

func formatDuration(_ seconds: Int) -> String {
    if seconds % 3600 == 0 { return "\(seconds / 3600) h" }
    if seconds >= 60 { return "\(seconds / 60) min" }
    return "\(seconds) s"
}
