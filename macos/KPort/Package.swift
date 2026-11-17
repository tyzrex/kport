// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "KPort",
    platforms: [.macOS(.v13)],
    products: [.executable(name: "KPort", targets: ["KPortApp"])],
    targets: [.executableTarget(name: "KPortApp")]
)
