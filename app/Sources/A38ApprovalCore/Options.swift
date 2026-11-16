// The command line of the app.
//
// The daemon starts the app without arguments. The test switches exist
// only in a debug build (T14): the compile-time condition A38_TEST_SWITCHES
// is set for the debug configuration only (Package.swift). A release build
// rejects each test switch with exit code 64.
import Foundation

public enum TestSwitches {
    /// True in a debug build.
    #if A38_TEST_SWITCHES
    public static let compiledIn = true
    #else
    public static let compiledIn = false
    #endif

    /// The names of the test switches. A release build names them in the
    /// error message only.
    public static let names: Set<String> = [
        "--request", "--skip-auth", "--fake-auth", "--auto-close", "--no-window",
        "--demo-select", "--demo-scroll-end", "--demo-arm-delay", "--demo-confirm", "--debug-window",
        "--demo-appearance",
    ]
}

/// The appearance of the window for a screenshot (test switch
/// --demo-appearance).
public enum DemoAppearance: String, Equatable, Sendable {
    case light
    case dark
}

public struct Options: Equatable, Sendable {
    public var showHelp = false
    public var showVersion = false

    // Test switches. They keep their default value in a release build.

    /// Read the message from a file, not from the standard input.
    public var requestPath: String?
    /// Use the fake authenticator with this result (policy "skipped").
    public var fakeAuth: AuthOutcome?
    /// Close after this time: a request with the outcome timeout, a notice
    /// without a decision.
    public var autoClose: TimeInterval?
    /// Run without a window. The process tests use it.
    public var noWindow = false
    /// Select these entries at the start ("all" selects each entry). For
    /// screenshots only.
    public var demoSelect: [String] = []
    /// Scroll a long list to its end at the start.
    public var demoScrollEnd = false
    /// Use another delay before the confirm action becomes active.
    public var demoArmDelay: TimeInterval?
    /// Click the confirm action once it is active (with --no-window).
    public var demoConfirm = false
    /// Write the window attributes to the standard error.
    public var debugWindow = false
    /// Show the window in this appearance, not in the appearance of the
    /// system. For screenshots only.
    public var demoAppearance: DemoAppearance?

    public init() {}
}

public struct UsageError: Error, Equatable, Sendable {
    public var message: String
}

public let usageText = usageBase + usageTestSwitches

let usageBase = """
Usage: Passierschein < message.json

The daemon a38 serve starts the app without arguments. The app reads one
message (an approval request or a notice) from the standard input. For a
request it writes one decision as one line of JSON to the standard output.

  --help       Show this text.
  --version    Show the version.

"""

#if A38_TEST_SWITCHES
let usageTestSwitches = """
Test switches (debug build only):
  --request <file>          Read the message from a file.
  --skip-auth               Use the fake authenticator: success, policy "skipped".
  --fake-auth <result>      success, cancel, unavailable or fail.
  --auto-close <seconds>    Close after this time.
  --no-window               Run without a window.
  --demo-select <ids|all>   Select these entries at the start (comma list).
  --demo-scroll-end         Scroll a long list to its end at the start.
  --demo-arm-delay <s>      Use another delay before the confirm action.
  --demo-confirm            Click the confirm action once it is active.
  --debug-window            Write the window attributes to the standard error.
  --demo-appearance <a>     Show the window light or dark.
"""
#else
let usageTestSwitches = ""
#endif

public func parseOptions(_ arguments: [String], testSwitches: Bool = TestSwitches.compiledIn) throws(UsageError) -> Options {
    var options = Options()
    var args = arguments[...]
    func value(_ flag: String) throws(UsageError) -> String {
        guard let next = args.popFirst() else {
            throw UsageError(message: "the option \(flag) needs a value")
        }
        return next
    }
    func seconds(_ flag: String) throws(UsageError) -> TimeInterval {
        guard let s = TimeInterval(try value(flag)), s.isFinite, s >= 0 else {
            throw UsageError(message: "the value of \(flag) must be a number of seconds")
        }
        return s
    }
    while let flag = args.popFirst() {
        if flag == "--help" || flag == "-h" {
            options.showHelp = true
            continue
        }
        if flag == "--version" {
            options.showVersion = true
            continue
        }
        guard testSwitches else {
            if TestSwitches.names.contains(flag) {
                throw UsageError(message: "the option \(flag) is a test switch; this build does not have test switches")
            }
            throw UsageError(message: "the option \(flag) is not known")
        }
        #if A38_TEST_SWITCHES
        try parseTestSwitch(flag, into: &options, value: value, seconds: seconds)
        #else
        throw UsageError(message: "the option \(flag) is not known")
        #endif
    }
    return options
}

#if A38_TEST_SWITCHES
/// Reads one test switch. A release build does not contain this function.
private func parseTestSwitch(_ flag: String, into options: inout Options,
                             value: (String) throws(UsageError) -> String,
                             seconds: (String) throws(UsageError) -> TimeInterval) throws(UsageError) {
    switch flag {
    case "--request":
        options.requestPath = try value(flag)
    case "--skip-auth":
        options.fakeAuth = .success
    case "--fake-auth":
        switch try value(flag) {
        case "success": options.fakeAuth = .success
        case "cancel": options.fakeAuth = .cancelled
        case "unavailable": options.fakeAuth = .unavailable("the fake authenticator reports no biometry")
        case "fail": options.fakeAuth = .failed("the fake authenticator reports a failure")
        default: throw UsageError(message: "the value of --fake-auth is not known")
        }
    case "--auto-close":
        let s = try seconds(flag)
        guard s > 0 else { throw UsageError(message: "the value of --auto-close must be more than zero") }
        options.autoClose = s
    case "--no-window":
        options.noWindow = true
    case "--demo-select":
        options.demoSelect = try value(flag).split(separator: ",").map(String.init)
    case "--demo-scroll-end":
        options.demoScrollEnd = true
    case "--demo-arm-delay":
        options.demoArmDelay = try seconds(flag)
    case "--demo-confirm":
        options.demoConfirm = true
    case "--debug-window":
        options.debugWindow = true
    case "--demo-appearance":
        guard let appearance = DemoAppearance(rawValue: try value(flag)) else {
            throw UsageError(message: "the value of --demo-appearance must be light or dark")
        }
        options.demoAppearance = appearance
    default:
        throw UsageError(message: "the option \(flag) is not known")
    }
}
#endif
