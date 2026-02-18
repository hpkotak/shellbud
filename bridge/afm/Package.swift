// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "afm-bridge",
    platforms: [.macOS("26.0")],
    targets: [
        // AFMBridgeCore: pure Swift, no FoundationModels — testable on any macOS.
        .target(
            name: "AFMBridgeCore",
            path: "Sources/AFMBridgeCore"
        ),
        // afm-bridge: the executable, imports FoundationModels (macOS 26+ only).
        .executableTarget(
            name: "afm-bridge",
            dependencies: ["AFMBridgeCore"],
            path: "Sources/afm-bridge"
        ),
        .testTarget(
            name: "AFMBridgeCoreTests",
            dependencies: ["AFMBridgeCore"],
            path: "Tests/AFMBridgeCoreTests"
        ),
    ]
)
