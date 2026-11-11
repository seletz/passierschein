// swift-tools-version: 6.0
// Throwaway spike for issue #4. This is not production code.
import PackageDescription

let package = Package(
    name: "ApprovalPopup",
    platforms: [.macOS(.v15)],
    targets: [
        .executableTarget(
            name: "approval-popup",
            path: "Sources/ApprovalPopup",
            swiftSettings: [.swiftLanguageMode(.v5)]
        )
    ]
)
