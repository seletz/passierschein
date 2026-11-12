// T14 part 1: the built app as a process, as the daemon starts it: no
// arguments (except test switches), a restricted environment, one message
// on the standard input. The runs with a valid request use --no-window and
// a fake authenticator: no test opens a window or calls Touch ID.
import Foundation
import Testing
@testable import A38ApprovalCore

struct RunResult {
    var code: Int32
    var stdout: String
    var stderr: String

    var lines: [String] { stdout.split(separator: "\n", omittingEmptySubsequences: false).map(String.init).filter { !$0.isEmpty } }

    func decision() throws -> [String: Any] {
        #expect(lines.count == 1, "exactly one line on the standard output: \(stdout)")
        let line = try #require(lines.first)
        let object = try JSONSerialization.jsonObject(with: Data(line.utf8))
        return try #require(object as? [String: Any])
    }
}

final class BundleMarker {}

enum AppProcess {
    /// The built executable next to the test bundle.
    static var executable: URL {
        // The test bundle lies in the build folder next to the executable.
        Bundle(for: BundleMarker.self).bundleURL.deletingLastPathComponent().appendingPathComponent("Passierschein")
    }

    static func run(_ args: [String], input: Data) throws -> RunResult {
        let process = Process()
        process.executableURL = executable
        process.arguments = args
        process.currentDirectoryURL = URL(fileURLWithPath: "/")
        // The environment that the daemon passes (contract of issue 40).
        let env = ProcessInfo.processInfo.environment
        process.environment = ["HOME", "USER", "LOGNAME", "TMPDIR", "LANG", "LC_ALL", "PATH", "__CF_USER_TEXT_ENCODING"]
            .reduce(into: [String: String]()) { $0[$1] = env[$1] }
        let stdin = Pipe(), stdout = Pipe(), stderr = Pipe()
        process.standardInput = stdin
        process.standardOutput = stdout
        process.standardError = stderr
        try process.run()
        stdin.fileHandleForWriting.write(input)
        try stdin.fileHandleForWriting.close()
        let out = stdout.fileHandleForReading.readDataToEndOfFile()
        let err = stderr.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        return RunResult(code: process.terminationStatus, stdout: String(decoding: out, as: UTF8.self),
                         stderr: String(decoding: err, as: UTF8.self))
    }
}

@Suite("Process (T14 part 1)", .serialized)
struct ProcessTests {
    @Test("T14 part 1: without a confirmation each case ends with timeout and exit 11",
          arguments: ["first-request", "bundle", "deviation"])
    func timeout(sample: String) throws {
        let r = try AppProcess.run(["--no-window", "--auto-close", "0.3"], input: try Fixtures.sampleData(sample))
        #expect(r.code == 11, "\(r.stderr)")
        let d = try r.decision()
        #expect(d["outcome"] as? String == "timeout")
        #expect(d["approved_credential_ids"] as? [String] == [])
        #expect(d["ttl_seconds"] == nil)
    }

    @Test("T14 part 1: invalid JSON ends with error and exit 65, before a window exists",
          arguments: ["", "{", "not json", "[1,2]"])
    func invalidJSON(text: String) throws {
        // No --no-window: the app must end before it creates a window.
        let r = try AppProcess.run([], input: Data(text.utf8))
        #expect(r.code == 65)
        let d = try r.decision()
        #expect(d["outcome"] as? String == "error")
        #expect((d["error"] as? [String: String])?["code"] == "invalid_request")
        #expect(d["request_id"] == nil)
    }

    @Test("A request of another version ends with unsupported_version and exit 65")
    func otherVersion() throws {
        var tree = Fixtures.requestTree()
        tree["version"] = 2
        let r = try AppProcess.run([], input: Fixtures.json(tree))
        #expect(r.code == 65)
        #expect((try r.decision()["error"] as? [String: String])?["code"] == "unsupported_version")
    }

    @Test("The diagnostics name IDs only, never a text of the request")
    func diagnostics() throws {
        var tree = Fixtures.requestTree()
        tree["grant"] = nil
        let r = try AppProcess.run([], input: Fixtures.json(tree))
        #expect(r.code == 65)
        #expect(r.stderr.contains("req-0001"))
        for value in ["beispielprojekt", "datenbank-dev", "beispiel-koordinator", "Run the tests"] {
            #expect(!r.stderr.contains(value))
            #expect(!r.stdout.contains(value))
        }
    }

    @Test("Only exit code 0 with approved is an approval: the fake gives policy skipped")
    func approvedWithFake() throws {
        let r = try AppProcess.run(["--no-window", "--skip-auth", "--demo-select", "c2", "--demo-arm-delay", "0", "--demo-confirm"],
                                   input: try Fixtures.sampleData("first-request"))
        #expect(r.code == 0, "\(r.stderr)")
        let d = try r.decision()
        #expect(d["outcome"] as? String == "approved")
        #expect(d["approved_credential_ids"] as? [String] == ["c2"])
        #expect(d["ttl_seconds"] as? Int == 14400)
        // The daemon never accepts the policy skipped as an approval.
        #expect(d["auth"] as? [String: AnyHashable] == ["policy": "skipped", "ok": true])
    }

    @Test("Touch ID not available ends with error and exit 12", arguments: [("unavailable", "auth_unavailable"), ("fail", "auth_failed")])
    func authError(mode: String, code: String) throws {
        let r = try AppProcess.run(["--no-window", "--fake-auth", mode, "--demo-select", "all", "--demo-arm-delay", "0", "--demo-confirm"],
                                   input: try Fixtures.sampleData("deviation"))
        #expect(r.code == 12)
        let d = try r.decision()
        #expect(d["outcome"] as? String == "error")
        #expect((d["error"] as? [String: String])?["code"] == code)
        #expect(d["approved_credential_ids"] as? [String] == [])
    }

    @Test("A notice writes nothing to the standard output and ends with 0",
          arguments: ["notice-daemon-fault", "notice-project-config-invalid"])
    func notice(sample: String) throws {
        let r = try AppProcess.run(["--no-window", "--auto-close", "0.1"], input: try Fixtures.sampleData(sample))
        #expect(r.code == 0, "\(r.stderr)")
        #expect(r.stdout.isEmpty)
    }

    @Test("An unknown option ends with 64")
    func usage() throws {
        let r = try AppProcess.run(["--approve-everything"], input: Data())
        #expect(r.code == 64)
        #expect(try r.decision()["outcome"] as? String == "error")
    }
}
