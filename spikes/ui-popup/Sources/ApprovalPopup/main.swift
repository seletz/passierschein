// Throwaway spike for issue #4: approval popup.
// The app reads one request from stdin, shows the popup and writes one decision to stdout.
import AppKit
import SwiftUI

struct Options {
    var autoClose: TimeInterval?
    var noAuth = false
    var requestPath: String?
    var authPolicy = AuthPolicy.biometrics
    var panel = PanelOptions()
    var debugWindow = false
}

let usage = """
Usage: approval-popup [options] < request.json
  --request <file>       Read the request from a file, not from stdin.
  --auto-close <seconds> Close the popup after this time with outcome "timeout".
  --no-auth              Do not call Touch ID. Use this flag only for tests.
  --auth-policy <name>   biometrics (default), biometrics_or_companion, owner.
  --take-focus           Make the popup the key window when it opens.
  --closable             Add a close button. Use this flag to test the window manager.
  --debug-window         Write the window attributes to stderr.
"""

func finish(_ decision: ApprovalDecision, _ code: ExitCode) -> Never {
    print(encodeDecision(decision))
    fflush(stdout)
    exit(code.rawValue)
}

func fail(_ code: ExitCode, errorCode: String, message: String, requestId: String? = nil) -> Never {
    FileHandle.standardError.write(Data((message + "\n").utf8))
    finish(ApprovalDecision(requestId: requestId, outcome: "error",
                            error: DecisionError(code: errorCode, message: message), elapsedMs: 0), code)
}

func parseOptions() -> Options {
    var options = Options()
    var args = Array(CommandLine.arguments.dropFirst())
    func value(_ flag: String) -> String {
        guard !args.isEmpty else {
            FileHandle.standardError.write(Data("The option \(flag) needs a value.\n\(usage)\n".utf8))
            exit(ExitCode.usage.rawValue)
        }
        return args.removeFirst()
    }
    while !args.isEmpty {
        let flag = args.removeFirst()
        switch flag {
        case "--auto-close":
            guard let seconds = TimeInterval(value(flag)), seconds > 0 else {
                FileHandle.standardError.write(Data("The value of --auto-close must be a positive number.\n".utf8))
                exit(ExitCode.usage.rawValue)
            }
            options.autoClose = seconds
        case "--no-auth": options.noAuth = true
        case "--request": options.requestPath = value(flag)
        case "--auth-policy":
            guard let policy = AuthPolicy(rawValue: value(flag)) else {
                FileHandle.standardError.write(Data("The auth policy is not known.\n\(usage)\n".utf8))
                exit(ExitCode.usage.rawValue)
            }
            options.authPolicy = policy
        case "--take-focus": options.panel.takeFocus = true
        case "--closable": options.panel.closable = true
        case "--debug-window": options.debugWindow = true
        case "--help", "-h":
            print(usage)
            exit(0)
        default:
            FileHandle.standardError.write(Data("The option \(flag) is not known.\n\(usage)\n".utf8))
            exit(ExitCode.usage.rawValue)
        }
    }
    return options
}

let options = parseOptions()

// Read the full request before the app shows a window.
let input: Data
if let path = options.requestPath {
    guard let data = FileManager.default.contents(atPath: path) else {
        fail(.usage, errorCode: "invalid_request", message: "The app cannot read the file \(path).")
    }
    input = data
} else {
    input = FileHandle.standardInput.readDataToEndOfFile()
}

let request: ApprovalRequest
do {
    request = try decodeRequest(input)
} catch RequestError.unsupportedVersion(let version) {
    fail(.invalidRequest, errorCode: "unsupported_version",
         message: "The app does not support protocol version \(version).")
} catch RequestError.invalid(let message) {
    fail(.invalidRequest, errorCode: "invalid_request", message: message)
} catch {
    fail(.internalError, errorCode: "internal", message: "\(error)")
}

let timeout = options.autoClose ?? TimeInterval(request.timeoutSeconds ?? 120)
let model = PopupModel(request: request, timeout: timeout)

let app = NSApplication.shared
// ".accessory" gives the same behaviour as LSUIElement: no Dock icon and no menu bar.
app.setActivationPolicy(.accessory)

let panel = ApprovalPanel(options: options.panel)
let hosting = NSHostingView(rootView: PopupView(model: model))
panel.contentView = hosting
panel.setContentSize(hosting.fittingSize)
panel.place()

func decision(_ outcome: String) -> ApprovalDecision {
    ApprovalDecision(requestId: request.requestId, outcome: outcome, elapsedMs: model.elapsedMs)
}

func approve(policyName: String) -> Never {
    var result = decision("approved")
    // Keep the order of the request.
    result.approvedCredentialIds = request.credentials.map(\.id).filter(model.selected.contains)
    result.ttlSeconds = request.grant.ttlSeconds
    result.auth = AuthResult(policy: policyName, ok: true)
    finish(result, .approved)
}

model.onDeny = { finish(decision("denied"), .denied) }
panel.onCancel = { finish(decision("denied"), .denied) }

model.onApprove = {
    // The view disables the button. This check is a second barrier.
    guard model.canConfirm else { return }
    if options.noAuth {
        approve(policyName: "skipped")
    }
    model.authInProgress = true
    model.message = nil
    let count = model.selected.count
    let reason = "approve \(count) credential\(count == 1 ? "" : "s") for project \(request.verified.project)"
    authenticate(policy: options.authPolicy, reason: reason) { outcome in
        model.authInProgress = false
        switch outcome {
        case .success:
            approve(policyName: options.authPolicy.rawValue)
        case .cancelled:
            model.message = "Touch ID was stopped. Try again or deny the request."
        case .unavailable(let text):
            var result = decision("error")
            result.auth = AuthResult(policy: options.authPolicy.rawValue, ok: false)
            result.error = DecisionError(code: "auth_unavailable", message: text)
            finish(result, .authError)
        case .failed(let text):
            var result = decision("error")
            result.auth = AuthResult(policy: options.authPolicy.rawValue, ok: false)
            result.error = DecisionError(code: "auth_failed", message: text)
            finish(result, .authError)
        }
    }
}

let timer = Timer(timeInterval: 0.25, repeats: true) { _ in
    model.tick()
    // The timeout does not stop a Touch ID dialog that is open.
    if model.secondsLeft <= 0 && !model.authInProgress {
        finish(decision("timeout"), .timeout)
    }
}
RunLoop.main.add(timer, forMode: .common)

if options.panel.takeFocus {
    panel.makeKeyAndOrderFront(nil)
} else {
    // Show the panel, but do not take the keyboard focus from the user.
    panel.orderFrontRegardless()
}

if options.debugWindow {
    DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) {
        printWindowDebug(panel)
    }
}

app.run()
