import FluidAudio
import Foundation
import Hummingbird

@main
struct App {
    static func main() async throws {
        let port = Int(ProcessInfo.processInfo.environment["SCRIB_DIAR_PORT"] ?? "8003") ?? 8003

        let modelDir = URL(fileURLWithPath: NSHomeDirectory())
            .appendingPathComponent(".cache/fluid-audio/models")

        try FileManager.default.createDirectory(at: modelDir, withIntermediateDirectories: true)

        let manager = OfflineDiarizerManager(config: .default)
        try await manager.prepareModels(directory: modelDir, configuration: .init())

        let state = AppState(manager: manager)

        let router = Router()
        router.get("/health") { _, _ in
            return Response(
                status: .ok,
                headers: [.contentType: "application/json"],
                body: .init(byteBuffer: ByteBuffer(string: #"{"status":"ok","model_loaded":true}"#))
            )
        }
        router.post("/v1/diarize") { request, context in
            try await DiarizeHandler.handle(request: request, context: context, state: state)
        }

        let app = Application(router: router, configuration: .init(address: .hostname("0.0.0.0", port: port)))
        print("scrib-diar listening on port \(port)")
        try await app.runService()
    }
}

final class AppState: Sendable {
    let manager: OfflineDiarizerManager

    init(manager: OfflineDiarizerManager) {
        self.manager = manager
    }
}
