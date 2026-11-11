// swift-tools-version: 6.0
// The approval app of a38 (F11). The daemon starts it for one approval
// request or one notice. See docs/reference/90-freigabe-app.md.
import PackageDescription

// The test switches (--skip-auth, --auto-close, ...) exist only in a debug
// build (T14). A release build does not compile them and rejects them as
// unknown options.
let testSwitches: [SwiftSetting] = [
    .define("A38_TEST_SWITCHES", .when(configuration: .debug)),
]

let package = Package(
    name: "A38Approval",
    platforms: [.macOS(.v15)],
    products: [
        .executable(name: "a38-approve", targets: ["a38-approve"]),
    ],
    targets: [
        // Protocol, validation, view models, decisions. No AppKit, no
        // SwiftUI, no LocalAuthentication: the tests run headless.
        .target(
            name: "A38ApprovalCore",
            swiftSettings: testSwitches
        ),
        // Window, views and the Touch ID authenticator.
        .target(
            name: "A38ApprovalUI",
            dependencies: ["A38ApprovalCore"],
            swiftSettings: testSwitches
        ),
        .executableTarget(
            name: "a38-approve",
            dependencies: ["A38ApprovalCore", "A38ApprovalUI"],
            swiftSettings: testSwitches
        ),
        .testTarget(
            name: "A38ApprovalCoreTests",
            // The process tests start the built executable.
            dependencies: ["A38ApprovalCore", "a38-approve"],
            swiftSettings: testSwitches
        ),
    ]
)
