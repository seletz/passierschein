// The layout of the approval popup, as accepted in the spike report
// (section "Aufbau des Popups"): banner, verified facts, claims of the
// agent, credentials, grant, footer. A command of the exec way (D65) has
// its own body: banner with the environment, verified facts, command,
// variables, claims of the agent, kind of approval, footer. The view shows the state of the
// ApprovalViewModel and calls its actions; it has no rules of its own.
// No control is focusable and no button is the default button (D31).
import A38ApprovalCore
import SwiftUI

struct PopupView: View {
    let model: ApprovalViewModel
    static let width: CGFloat = 540

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            banner
            verifiedSection
            if model.request.exec != nil {
                commandSection
                variableSection
                claimsSection
                execApprovalSection
            } else {
                claimsSection
                credentialSection
                grantSection
            }
            if let message = model.message {
                Text(message).font(.callout).foregroundStyle(.red)
            }
            Divider()
            footer
        }
        .padding(16)
        .frame(width: Self.width)
    }

    // MARK: Banner

    private var banner: some View {
        let style: (title: String, detail: String, icon: String, color: Color)
        switch model.popupCase {
        case .firstRequest:
            style = (UIText.bannerFirstTitle, UIText.bannerFirstDetail, "key.fill", .blue)
        case .deviation:
            style = (UIText.bannerDeviationTitle, UIText.bannerDeviationDetail, "exclamationmark.triangle.fill", .orange)
        case .command(let environment):
            style = (UIText.bannerCommandTitle, UIText.bannerCommandDetail(environment), "terminal.fill",
                     Self.bannerColor(environment))
        }
        return HStack(spacing: 10) {
            Image(systemName: style.icon).font(.title2)
            VStack(alignment: .leading, spacing: 2) {
                Text(style.title).font(.headline)
                Text(style.detail).font(.callout)
                    .fixedSize(horizontal: false, vertical: true)
            }
            Spacer()
            VStack(alignment: .trailing, spacing: 4) {
                Text(UIText.brand).font(.caption.monospaced()).opacity(0.8)
                if case .command(let environment) = model.popupCase, environment != .dev {
                    // D65: test and prod stand out.
                    Text(UIText.environmentBadge(environment))
                        .font(.title3.monospaced().bold())
                        .padding(.horizontal, 8).padding(.vertical, 2)
                        .foregroundStyle(style.color)
                        .background(.white, in: RoundedRectangle(cornerRadius: 4))
                }
            }
        }
        .foregroundStyle(.white)
        .padding(10)
        .frame(maxWidth: .infinity)
        .background(style.color, in: RoundedRectangle(cornerRadius: 8))
    }

    // MARK: Verified facts

    private var verifiedSection: some View {
        VStack(alignment: .leading, spacing: 6) {
            Label(UIText.verifiedHeading, systemImage: "checkmark.shield.fill")
                .font(.subheadline.bold())
                .foregroundStyle(.green)
            Grid(alignment: .leadingFirstTextBaseline, horizontalSpacing: 12, verticalSpacing: 3) {
                if let exec = model.request.exec {
                    GridRow {
                        Text(UIText.factEnvironment).foregroundStyle(.secondary)
                        environmentMark(exec.environment)
                            .frame(maxWidth: .infinity, alignment: .leading)
                    }
                }
                ForEach(model.verifiedRows) { row in
                    GridRow {
                        Text(row.label).foregroundStyle(.secondary)
                        Text(row.value).font(.body.monospaced())
                            .lineLimit(2).truncationMode(.middle)
                            .fixedSize(horizontal: false, vertical: true)
                            .frame(maxWidth: .infinity, alignment: .leading)
                    }
                }
            }
        }
    }

    // MARK: Command (exec way, D65)

    /// The colour of an environment: dev neutral, test orange, prod red.
    static func environmentColor(_ environment: ExecEnvironment) -> Color {
        switch environment {
        case .dev: return .secondary
        case .test: return .orange
        case .prod: return .red
        }
    }

    static func bannerColor(_ environment: ExecEnvironment) -> Color {
        environment == .dev ? .blue : environmentColor(environment)
    }

    private func environmentMark(_ environment: ExecEnvironment) -> some View {
        let highlighted = environment != .dev
        return Text(highlighted ? UIText.environmentBadge(environment) : environment.rawValue)
            .font(.body.monospaced().bold())
            .padding(.horizontal, highlighted ? 6 : 0)
            .foregroundStyle(highlighted ? Color.white : Color.primary)
            .background(highlighted ? Self.environmentColor(environment) : Color.clear,
                        in: RoundedRectangle(cornerRadius: 4))
    }

    private var commandSection: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(alignment: .firstTextBaseline) {
                Label(UIText.commandHeading, systemImage: "terminal")
                    .font(.subheadline.bold())
                Text(UIText.commandNote).font(.caption).foregroundStyle(.secondary)
            }
            if model.commandScrolls {
                // A long command scrolls inside a frame. Nothing is cut
                // (D65). Banner, variables and footer stay visible.
                ScrollView { commandGrid.padding(8).padding(.trailing, 8) }
                    .scrollIndicators(.visible)
                    .frame(height: 220)
                    .overlay(RoundedRectangle(cornerRadius: 6).strokeBorder(.separator))
                Text(UIText.commandScrolls(arguments: model.request.exec?.args.count ?? 0,
                                           characters: model.commandLength))
                    .font(.caption)
                    .foregroundStyle(.secondary)
            } else {
                commandGrid
            }
        }
    }

    private var commandGrid: some View {
        Grid(alignment: .leadingFirstTextBaseline, horizontalSpacing: 12, verticalSpacing: 4) {
            ForEach(model.commandRows) { row in
                GridRow {
                    Text(row.label).foregroundStyle(.secondary)
                    // No line limit and no hyphenation: long arguments
                    // wrap, nothing is cut or added.
                    CommandText(text: row.value)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
        }
    }

    // MARK: Variables (exec way, D65)

    private var variableSection: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                Text(UIText.variablesHeading).font(.subheadline.bold())
                Spacer()
                Text(model.selectedCountText)
                    .font(.callout.monospacedDigit())
                    .foregroundStyle(model.selected.isEmpty ? .secondary : .primary)
            }
            ForEach(model.variableRows) { row in
                Toggle(isOn: binding(for: row.id)) {
                    VStack(alignment: .leading, spacing: 1) {
                        HStack(spacing: 6) {
                            Text(row.names.joined(separator: ", ")).font(.body.monospaced().bold())
                            if let deviation = row.credential.deviation {
                                badge(deviation == .newItem ? UIText.deviationNewItem : UIText.deviationNewVault, .orange)
                            }
                        }
                        Text(row.reference).font(.callout.monospaced()).foregroundStyle(.secondary)
                            .fixedSize(horizontal: false, vertical: true)
                    }
                }
                .toggleStyle(.checkbox)
                .focusable(false)
                .disabled(!model.canEditSelection)
            }
        }
    }

    // MARK: Kind of approval (exec way, D63, D65)

    private var execApprovalSection: some View {
        VStack(alignment: .leading, spacing: 6) {
            if model.request.exec?.approval.kind == .once {
                HStack(spacing: 8) {
                    Text(UIText.approvalOnceLabel + ":").font(.callout)
                    Text(model.approvalText).font(.callout.bold())
                    Spacer()
                }
                Text(UIText.approvalOnceDetail)
                    .font(.callout)
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
            } else {
                lifetimeControls
                Text(model.bindingText)
                    .font(.callout)
                    .fixedSize(horizontal: false, vertical: true)
                Text(model.scopeText)
                    .font(.callout)
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
    }

    // MARK: Claims of the agent

    /// The colour of the claims: the heading and the frame. Green marks the
    /// verified facts, orange marks the claims that the daemon did not verify.
    private static let claimsColor = Color.orange

    private var claimsSection: some View {
        VStack(alignment: .leading, spacing: 4) {
            Label(UIText.claimsHeading, systemImage: "questionmark.bubble")
                .font(.subheadline.bold())
                .foregroundStyle(Self.claimsColor)
            Grid(alignment: .leadingFirstTextBaseline, horizontalSpacing: 8, verticalSpacing: 3) {
                ForEach(model.claimRows) { row in
                    GridRow {
                        Text(row.label).foregroundStyle(.secondary)
                        // The values have the full text colour: the human
                        // must be able to read them. The colour of the
                        // heading and the frame marks them as not verified.
                        // The session name is never cut (D25). Each other
                        // claim is cut to three lines.
                        Text(row.value)
                            .lineLimit(row.unabridged ? nil : 3)
                            .truncationMode(.tail)
                            .fixedSize(horizontal: false, vertical: true)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .foregroundStyle(.primary)
                            .opacity(row.missing ? 0.6 : 1)
                    }
                }
            }
            .font(.callout)
        }
        .padding(8)
        .frame(maxWidth: .infinity, alignment: .leading)
        .overlay(
            RoundedRectangle(cornerRadius: 6)
                .strokeBorder(style: StrokeStyle(lineWidth: 1, dash: [4, 3]))
                .foregroundStyle(Self.claimsColor.opacity(0.7))
        )
    }

    // MARK: Credentials

    private var credentialSection: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                Text(UIText.credentialsHeading).font(.subheadline.bold())
                Spacer()
                Text(model.selectedCountText)
                    .font(.callout.monospacedDigit())
                    .foregroundStyle(model.selected.isEmpty ? .secondary : .primary)
            }
            if model.listScrolls {
                // A long bundle scrolls. Banner, facts, counter and footer
                // stay visible.
                // The scroll bar and the frame stay visible, so that the
                // human sees that the list goes on.
                ScrollViewReader { proxy in
                    ScrollView { credentialList.padding(8).padding(.trailing, 8) }
                        .scrollIndicators(.visible)
                        .onAppear { demoScroll(proxy) }
                }
                    .frame(height: 180)
                    .overlay(RoundedRectangle(cornerRadius: 6).strokeBorder(.separator))
                Text(UIText.listScrolls(credentials: model.request.credentials.count, vaults: model.groups.count))
                    .font(.caption)
                    .foregroundStyle(.secondary)
            } else {
                credentialList
            }
        }
    }

    private func demoScroll(_ proxy: ScrollViewProxy) {
        #if A38_TEST_SWITCHES
        // Test switch --demo-scroll-end, for a screenshot.
        if model.demoScrollToEnd, let last = model.groups.last {
            proxy.scrollTo(last.id, anchor: .bottom)
        }
        #endif
    }

    private var credentialList: some View {
        VStack(alignment: .leading, spacing: 8) {
            ForEach(model.groups) { group in
                vaultGroup(group).id(group.id)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private func vaultGroup(_ group: VaultGroup) -> some View {
        VStack(alignment: .leading, spacing: 3) {
            HStack(spacing: 6) {
                Image(systemName: "lock.rectangle.stack")
                Text(DisplayText.clean(group.vault)).font(.callout.monospaced().bold())
                Text("\(group.items.count)").font(.caption).foregroundStyle(.secondary)
                Spacer()
                if group.offersSelectAll {
                    Button(UIText.selectAll(group.items.count)) {
                        model.selectAll(vault: group.vault)
                    }
                    .buttonStyle(.link)
                    .focusable(false)
                    .disabled(!model.canEditSelection)
                }
            }
            ForEach(group.items) { item in
                Toggle(isOn: binding(for: item.id)) {
                    HStack(spacing: 6) {
                        Text(DisplayText.clean(item.title)).font(.body.monospaced())
                        Text(UIText.fieldLabel(DisplayText.clean(item.field)))
                            .font(.callout.monospaced())
                            .foregroundStyle(.secondary)
                        if let deviation = item.deviation {
                            badge(deviation == .newItem ? UIText.deviationNewItem : UIText.deviationNewVault, .orange)
                        }
                    }
                }
                .toggleStyle(.checkbox)
                .focusable(false)
                .disabled(!model.canEditSelection)
                .padding(.leading, 20)
            }
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
        Binding(get: { model.isSelected(id) }, set: { model.setSelected(id, $0) })
    }

    // MARK: Grant and lifetime

    private var grantSection: some View {
        VStack(alignment: .leading, spacing: 6) {
            lifetimeControls
            Text(model.scopeText)
                .font(.callout)
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
        }
    }

    /// The fixed set of lifetimes, always visible (one way in both
    /// windows). The proposal is the first and longest value and is
    /// selected at the start; no value is longer (D32, D63).
    private var lifetimeControls: some View {
        HStack(spacing: 8) {
            Text(UIText.lifetimeLabel + ":").font(.callout)
            Picker(UIText.lifetimeLabel, selection: lifetimeBinding) {
                ForEach(model.lifetimeChoices, id: \.self) { seconds in
                    Text(UIText.duration(seconds)).tag(seconds)
                }
            }
            .pickerStyle(.segmented)
            .labelsHidden()
            .focusable(false)
            .fixedSize()
            .disabled(!model.canEditSelection)
            Spacer()
        }
    }

    private var lifetimeBinding: Binding<Int64> {
        Binding(get: { model.lifetime }, set: { model.selectLifetime($0) })
    }

    // MARK: Footer

    private var footer: some View {
        HStack {
            // No button is the default button. Return has no function.
            Button(UIText.deny) { model.deny() }
                .focusable(false)
            Text(model.closesInText)
                .font(.caption.monospacedDigit())
                .foregroundStyle(.secondary)
                .lineLimit(2)
                .fixedSize(horizontal: false, vertical: true)
            Spacer()
            Button {
                Task { await model.confirm() }
            } label: {
                Label(model.confirmTitle, systemImage: "touchid")
            }
            .disabled(!model.canConfirm)
            .focusable(false)
        }
    }
}
