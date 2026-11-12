// The layout of a notice (D36, D48, D28, G20). It informs only: it names
// what happened and the next steps, and its one action is Close.
import A38ApprovalCore
import SwiftUI

struct NoticeView: View {
    let model: NoticeViewModel
    static let width: CGFloat = 540

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            banner
            Text(model.detail).font(.body.bold())
            VStack(alignment: .leading, spacing: 4) {
                Text(UIText.noticeWhatHappened).font(.subheadline.bold()).foregroundStyle(.secondary)
                Text(model.message)
                    .font(.callout)
                    .fixedSize(horizontal: false, vertical: true)
            }
            Grid(alignment: .leadingFirstTextBaseline, horizontalSpacing: 12, verticalSpacing: 3) {
                ForEach(model.rows) { row in
                    GridRow {
                        Text(row.label).foregroundStyle(.secondary)
                        Text(row.value).font(.callout.monospaced())
                    }
                }
            }
            .font(.callout)
            VStack(alignment: .leading, spacing: 4) {
                Text(UIText.noticeNextSteps).font(.subheadline.bold())
                ForEach(Array(model.nextSteps.enumerated()), id: \.offset) { index, step in
                    Text("\(index + 1). \(step)").font(.callout)
                }
            }
            Divider()
            HStack {
                Text(UIText.noticeInfoOnly).font(.caption).foregroundStyle(.secondary)
                Spacer()
                // Not the default button: Return does nothing (D31).
                Button(UIText.noticeClose) { model.close() }
                    .focusable(false)
            }
        }
        .padding(16)
        .frame(width: Self.width)
    }

    private var banner: some View {
        // A daemon fault locks all sessions: red. An invalid project locks
        // one project: orange.
        let fault = model.notice.kind == .daemonFault
        return HStack(spacing: 10) {
            Image(systemName: fault ? "exclamationmark.octagon.fill" : "exclamationmark.triangle.fill").font(.title2)
            Text(model.title).font(.headline)
            Spacer()
            Text(UIText.brand).font(.caption.monospaced()).opacity(0.8)
        }
        .foregroundStyle(.white)
        .padding(10)
        .frame(maxWidth: .infinity)
        .background(fault ? Color.red : Color.orange, in: RoundedRectangle(cornerRadius: 8))
    }
}
