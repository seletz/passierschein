// The layout of the approval popup, as accepted in the spike report
// (section "Aufbau des Popups"): banner, verified facts, claims of the
// agent, credentials, grant, footer. The view shows the state of the
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
            claimsSection
            credentialSection
            grantSection
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
        }
        return HStack(spacing: 10) {
            Image(systemName: style.icon).font(.title2)
            VStack(alignment: .leading, spacing: 2) {
                Text(style.title).font(.headline)
                Text(style.detail).font(.callout)
            }
            Spacer()
            Text(UIText.brand).font(.caption.monospaced()).opacity(0.8)
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

    // MARK: Claims of the agent

    private var claimsSection: some View {
        VStack(alignment: .leading, spacing: 4) {
            Label(UIText.claimsHeading, systemImage: "questionmark.bubble")
                .font(.subheadline.bold())
                .foregroundStyle(.secondary)
            Grid(alignment: .leadingFirstTextBaseline, horizontalSpacing: 8, verticalSpacing: 3) {
                ForEach(model.claimRows) { row in
                    GridRow {
                        Text(row.label + ":")
                        // The session name is never cut (D25). Each other
                        // claim is cut to three lines.
                        Text(row.value)
                            .lineLimit(row.unabridged ? nil : 3)
                            .truncationMode(.tail)
                            .fixedSize(horizontal: false, vertical: true)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .opacity(row.missing ? 0.7 : 1)
                    }
                }
            }
            .font(.callout.italic())
            .foregroundStyle(.secondary)
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
            HStack(spacing: 8) {
                Text(UIText.lifetimeLabel + ":").font(.callout)
                Text(model.lifetimeText).font(.callout.bold())
                if model.lifetimeChoices.count > 1 && !model.lifetimeChoiceOpen {
                    Button(UIText.lifetimeShorter) { model.openLifetimeChoice() }
                        .buttonStyle(.link)
                        .focusable(false)
                        .disabled(!model.canEditSelection)
                }
                Spacer()
            }
            if model.lifetimeChoiceOpen {
                HStack(spacing: 8) {
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
                    Button(UIText.lifetimeKeep) { model.closeLifetimeChoice() }
                        .buttonStyle(.link)
                        .focusable(false)
                    Spacer()
                }
            }
            Text(model.scopeText)
                .font(.callout)
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
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
