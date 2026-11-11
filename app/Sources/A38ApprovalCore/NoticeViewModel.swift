// The state of the notice popup (D36, D48, D28, G20). A notice informs
// only: its one action is Close.
import Foundation

@MainActor
public final class NoticeViewModel {
    public let notice: Notice
    private let timeZone: TimeZone
    public var onClose: () -> Void = {}
    public private(set) var closed = false

    public init(notice: Notice, timeZone: TimeZone = .current) {
        self.notice = notice
        self.timeZone = timeZone
    }

    public var title: String {
        switch notice.kind {
        case .daemonFault: return UIText.noticeDaemonFaultTitle
        case .projectConfigInvalid: return UIText.noticeProjectTitle
        }
    }

    /// What the fault means for the human.
    public var detail: String {
        switch notice.kind {
        case .daemonFault: return UIText.noticeDaemonFaultDetail
        case .projectConfigInvalid: return UIText.noticeProjectDetail(DisplayText.clean(notice.project ?? ""))
        }
    }

    /// The finding of the daemon, as the daemon wrote it.
    public var message: String { DisplayText.clean(notice.message) }

    public var rows: [FactRow] {
        var rows: [FactRow] = []
        if let project = notice.project {
            rows.append(FactRow(label: UIText.noticeProject, value: DisplayText.clean(project)))
        }
        rows.append(FactRow(label: UIText.noticeCode, value: notice.code))
        rows.append(FactRow(label: UIText.noticeTime, value: DisplayText.timeWithSeconds(notice.occurredAt, timeZone: timeZone)))
        rows.append(FactRow(label: UIText.noticeID, value: notice.noticeID))
        return rows
    }

    public var nextSteps: [String] {
        switch notice.kind {
        case .daemonFault: return UIText.noticeDaemonFaultSteps
        case .projectConfigInvalid: return UIText.noticeProjectSteps
        }
    }

    /// The human closes the notice. It is the only action.
    public func close() {
        guard !closed else { return }
        closed = true
        onClose()
    }
}
