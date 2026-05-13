// swift-tools-version: 5.9

import PackageDescription

let package = Package(
    name: "scrib-diar",
    platforms: [.macOS(.v14)],
    dependencies: [
        .package(url: "https://github.com/hummingbird-project/hummingbird.git", from: "2.5.0"),
        .package(url: "https://github.com/FluidInference/FluidAudio.git", from: "0.12.4"),
    ],
    targets: [
        .executableTarget(
            name: "scrib-diar",
            dependencies: [
                .product(name: "Hummingbird", package: "hummingbird"),
                .product(name: "FluidAudio", package: "FluidAudio"),
            ],
            path: "Sources"
        ),
    ]
)
