// The run of the app: start, window, timer, decision.
import A38ApprovalCore
import AppKit
import SwiftUI

@MainActor
public enum AppMain {
    /// Keeps the window objects alive for the run of the app.
    private static var keep: [AnyObject] = []

    public static func run() -> Never {
        let startedAt = monotonicNow()
        // A closed standard output must not kill the app with a signal; the
        // write fails and the daemon treats the missing line as a denial.
        signal(SIGPIPE, SIG_IGN)
        let launch = prepare(arguments: Array(CommandLine.arguments.dropFirst()),
                             readInput: { readStandardInput() })
        switch launch {
        case let .exit(stdout, stderr, code):
            finishLaunch(stdout: stdout, stderr: stderr, code: code)
        case let .approval(request, options):
            #if A38_TEST_SWITCHES
            if options.noWindow { runHeadless(request: request, options: options, startedAt: startedAt) }
            #endif
            runApproval(request, options: options, startedAt: startedAt)
        case let .notice(notice, options):
            #if A38_TEST_SWITCHES
            if options.noWindow { runHeadlessNotice(notice, options: options) }
            #endif
            runNotice(notice, options: options)
        }
    }

    /// The authenticator of the run. A release build has Touch ID only.
    private static func authenticator(_ options: Options) -> any Authenticator {
        #if A38_TEST_SWITCHES
        if let outcome = options.fakeAuth {
            diagnose("test switch: fake authenticator, no Touch ID")
            return FakeAuthenticator(outcome: outcome)
        }
        #endif
        return TouchIDAuthenticator()
    }

    private static func startApp() -> NSApplication {
        let app = NSApplication.shared
        // ".accessory" is LSUIElement: no Dock icon, no menu bar (D6).
        app.setActivationPolicy(.accessory)
        return app
    }

    private static func show(_ panel: ApprovalPanel, options: Options) {
        panel.place()
        // Show the panel without the keyboard focus (D31, D45).
        panel.orderFrontRegardless()
        #if A38_TEST_SWITCHES
        if options.debugWindow {
            DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) {
                FileHandle.standardError.write(Data((panel.debugDescriptionLine() + "\n").utf8))
            }
        }
        #endif
    }

    private static func runApproval(_ request: ApprovalRequest, options: Options, startedAt: TimeInterval) -> Never {
        let app = startApp()
        var armDelay = ApprovalViewModel.armDelay
        #if A38_TEST_SWITCHES
        armDelay = options.demoArmDelay ?? armDelay
        #endif
        let model = ApprovalViewModel(request: request, authenticator: authenticator(options),
                                      startedAt: startedAt, clock: monotonicNow,
                                      timeoutOverride: options.autoClose, armDelay: armDelay)
        model.onFinish = { emit($0, request: request) }
        #if A38_TEST_SWITCHES
        let ids = options.demoSelect == ["all"] ? request.credentials.map(\.id) : options.demoSelect
        model.applyDemo(select: ids, lifetimeOpen: options.demoLifetimeOpen)
        model.demoScrollToEnd = options.demoScrollEnd
        #endif

        let panel = ApprovalPanel(width: PopupView.width)
        panel.title = UIText.windowTitle
        panel.onCancel = { model.deny() }
        let hosting = NSHostingView(rootView: PopupView(model: model))
        panel.contentView = hosting
        panel.setContentSize(hosting.fittingSize)

        let timer = Timer(timeInterval: 0.1, repeats: true) { _ in
            MainActor.assumeIsolated { model.tick() }
        }
        RunLoop.main.add(timer, forMode: .common)
        keep = [model, panel, hosting, timer]
        show(panel, options: options)
        app.run()
        emit(.failure(.internal, "the run loop ended without a decision", requestID: request.requestID, elapsedMS: 0),
             request: request)
    }

    private static func runNotice(_ notice: Notice, options: Options) -> Never {
        let app = startApp()
        let model = NoticeViewModel(notice: notice)
        model.onClose = {
            diagnose("notice \(notice.noticeID): closed")
            exit(ExitCode.success.rawValue)
        }
        let panel = ApprovalPanel(width: NoticeView.width)
        panel.title = UIText.windowTitle
        panel.onCancel = { model.close() }
        let hosting = NSHostingView(rootView: NoticeView(model: model))
        panel.contentView = hosting
        panel.setContentSize(hosting.fittingSize)
        keep = [model, panel, hosting]
        #if A38_TEST_SWITCHES
        if let seconds = options.autoClose {
            DispatchQueue.main.asyncAfter(deadline: .now() + seconds) { model.close() }
        }
        #endif
        show(panel, options: options)
        app.run()
        exit(ExitCode.success.rawValue)
    }
}
