// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "FieldworkKit",
    platforms: [.iOS(.v17), .macOS(.v14)],
    products: [
        .library(name: "FieldworkKit", targets: ["FieldworkKit"])
    ],
    targets: [
        .target(name: "FieldworkKit"),
        .testTarget(name: "FieldworkKitTests", dependencies: ["FieldworkKit"]),
    ]
)
