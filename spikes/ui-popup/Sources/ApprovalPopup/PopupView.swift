// Layout of the popup.
// Top: facts that the daemon verified. Below: claims of the agent, marked as not verified.
import SwiftUI

struct PopupView: View {
    @ObservedObject var model: PopupModel

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            banner
            verifiedSection
            claimsSection
            credentialSection
            if model.hasOutsideDev { acknowledgement }
            grantLine
            if let message = model.message {
                Text(message).font(.callout).foregroundStyle(.red)
            }
            Divider()
            footer
        }
        .padding(16)
        .frame(width: 540)
    }

    // MARK: Banner

    private var bannerStyle: (title: String, detail: String, icon: String, color: Color) {
        switch model.popupCase {
        case .initial:
            return ("Credential request", "First request of this session.", "key.fill", .blue)
        case .deviation:
            return ("Deviation from active grant",
                    "This request is not in the scope of the active grant.",
                    "exclamationmark.triangle.fill", .orange)
        case .outsideDev:
            return ("Request OUTSIDE dev",
                    "One or more credentials are not in a dev vault.",
                    "exclamationmark.octagon.fill", .red)
        }
    }

    private var banner: some View {
        let style = bannerStyle
        return HStack(spacing: 10) {
            Image(systemName: style.icon).font(.title2)
            VStack(alignment: .leading, spacing: 2) {
                Text(style.title).font(.headline)
                Text(style.detail).font(.callout)
            }
            Spacer()
            Text("a38").font(.caption.monospaced()).opacity(0.8)
        }
        .foregroundStyle(.white)
        .padding(10)
        .frame(maxWidth: .infinity)
        .background(style.color, in: RoundedRectangle(cornerRadius: 8))
    }

    // MARK: Verified facts

    private var verifiedRows: [(String, String)] {
        let facts = model.request.verified
        var rows: [(String, String)] = [("Project", facts.project)]
        if facts.origin == "vm" {
            rows.append(("Origin", "VM"))
            if let name = facts.vmName { rows.append(("VM name", name)) }
            if let session = facts.sshSession { rows.append(("SSH session", session)) }
        } else {
            rows.append(("Origin", "Local process"))
            if let process = facts.process {
                let pid = facts.peerPid.map { " (PID \($0))" } ?? ""
                rows.append(("Process", process + pid))
            }
            if let cwd = facts.cwd { rows.append(("Directory", cwd)) }
        }
        if let grant = model.request.existingGrant {
            rows.append(("Active grant",
                         "\(grant.itemCount) items, vaults \(grant.scope.vaults.joined(separator: ", ")), until \(grant.expiresAt)"))
        }
        return rows
    }

    private var verifiedSection: some View {
        VStack(alignment: .leading, spacing: 6) {
            Label("Verified by the daemon", systemImage: "checkmark.shield.fill")
                .font(.subheadline.bold())
                .foregroundStyle(.green)
            Grid(alignment: .leadingFirstTextBaseline, horizontalSpacing: 12, verticalSpacing: 3) {
                ForEach(verifiedRows, id: \.0) { row in
                    GridRow {
                        Text(row.0).foregroundStyle(.secondary)
                        Text(row.1).font(.body.monospaced()).textSelection(.enabled)
                            .lineLimit(2).truncationMode(.middle)
                    }
                }
            }
        }
    }

    // MARK: Agent claims

    private var claimsSection: some View {
        let claims = model.request.claims
        return VStack(alignment: .leading, spacing: 4) {
            Label("Claims of the agent. NOT verified.", systemImage: "questionmark.bubble")
                .font(.subheadline.bold())
                .foregroundStyle(.secondary)
            Group {
                Text("Agent: ") + Text(claims.agent ?? "no data")
                Text("Task: ") + Text(claims.task ?? "no data")
                Text("Reason: ") + Text(claims.reason ?? "no data")
            }
            .font(.callout.italic())
            .foregroundStyle(.secondary)
            .lineLimit(3)
        }
        .padding(8)
        .frame(maxWidth: .infinity, alignment: .leading)
        .overlay(
            RoundedRectangle(cornerRadius: 6)
                .strokeBorder(style: StrokeStyle(lineWidth: 1, dash: [4, 3]))
                .foregroundStyle(.secondary)
        )
    }

    // MARK: Credentials

    private var credentialSection: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                Text("Credentials").font(.subheadline.bold())
                Spacer()
                Text("\(model.selected.count) of \(model.request.credentials.count) selected")
                    .font(.callout.monospacedDigit())
                    .foregroundStyle(model.selected.isEmpty ? .secondary : .primary)
            }
            let list = VStack(alignment: .leading, spacing: 8) {
                ForEach(model.groups, id: \.vault) { group in
                    vaultGroup(vault: group.vault, items: group.items)
                }
            }
            if model.request.credentials.count > 6 {
                // A long bundle scrolls. The header shows the count at all times.
                ScrollView { list.padding(.trailing, 12) }.frame(height: 190)
            } else {
                list
            }
        }
    }

    private func vaultGroup(vault: String, items: [CredentialRef]) -> some View {
        let allDev = items.allSatisfy(\.isDev)
        return VStack(alignment: .leading, spacing: 3) {
            HStack(spacing: 6) {
                Image(systemName: "lock.rectangle.stack")
                Text(vault).font(.callout.monospaced().bold())
                Text("\(items.count)").font(.caption).foregroundStyle(.secondary)
                if !allDev { badge("OUTSIDE DEV", .red) }
                Spacer()
                // A bulk control exists only for dev vaults with more than three items.
                if allDev && items.count > 3 {
                    Button("Select all \(items.count)") {
                        model.selected.formUnion(items.map(\.id))
                    }
                    .buttonStyle(.link)
                    .focusable(false)
                }
            }
            ForEach(items) { item in
                Toggle(isOn: binding(for: item.id)) {
                    HStack(spacing: 6) {
                        Text(item.title).font(.body.monospaced())
                        if let deviation = item.deviation {
                            badge(deviationLabel(deviation), item.isDev ? .orange : .red)
                        }
                    }
                }
                .toggleStyle(.checkbox)
                .focusable(false)
                .padding(.leading, 20)
            }
        }
    }

    private func deviationLabel(_ code: String) -> String {
        switch code {
        case "new_item": return "NEW ITEM"
        case "new_vault": return "NEW VAULT"
        case "outside_scope": return "OUTSIDE SCOPE"
        default: return code.uppercased()
        }
    }

    private func badge(_ text: String, _ color: Color) -> some View {
        Text(text)
            .font(.caption2.bold())
            .padding(.horizontal, 5).padding(.vertical, 1)
            .foregroundStyle(.white)
            .background(color, in: Capsule())
    }

    private func binding(for id: String) -> Binding<Bool> {
        Binding(
            get: { model.selected.contains(id) },
            set: { isOn in
                if isOn { model.selected.insert(id) } else { model.selected.remove(id) }
            }
        )
    }

    // MARK: Acknowledgement, grant and footer

    private var acknowledgement: some View {
        Toggle(isOn: $model.acknowledgedOutsideDev) {
            Text("I approve access outside dev.").bold().foregroundStyle(.red)
        }
        .toggleStyle(.checkbox)
        .focusable(false)
    }

    private var grantLine: some View {
        let grant = model.request.grant
        return Text("The grant is valid for \(formatDuration(grant.ttlSeconds)). Scope: project \(grant.scope.project), vaults \(grant.scope.vaults.joined(separator: ", ")).")
            .font(.callout)
            .foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)
    }

    private var confirmTitle: String {
        if model.authInProgress { return "Touch ID…" }
        if !model.armed { return "Read the request…" }
        if model.selected.isEmpty { return "Select credentials" }
        return "Approve \(model.selected.count) with Touch ID"
    }

    private var footer: some View {
        HStack {
            // No button is the default button. The Return key has no function.
            Button("Deny (Esc)") { model.onDeny() }
                .focusable(false)
            Text("Closes in \(model.secondsLeft) s. No decision means no approval.")
                .font(.caption.monospacedDigit())
                .foregroundStyle(.secondary)
            Spacer()
            Button {
                model.onApprove()
            } label: {
                Label(confirmTitle, systemImage: "touchid")
            }
            .disabled(!model.canConfirm)
            .focusable(false)
        }
    }
}
