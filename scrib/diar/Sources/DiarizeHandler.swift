import FluidAudio
import Foundation
import Hummingbird

struct DiarizeResponse: Codable {
    struct Segment: Codable {
        let speaker: String
        let start: Double
        let end: Double
    }

    let speakers: Int
    let segments: [Segment]
}

enum DiarizeHandler {
    static func handle(request: Request, context: some RequestContext, state: AppState) async throws -> Response {
        let body = try await request.body.collect(upTo: 500 * 1024 * 1024)
        let data = Data(buffer: body)

        let contentType = request.headers[.contentType] ?? ""
        let samples: [Float]

        if contentType.contains("audio/wav") || AudioUtils.isWAV(data: data) {
            samples = try AudioUtils.convertWAV(data: data)
        } else {
            samples = data.withUnsafeBytes { ptr in
                Array(ptr.bindMemory(to: Float.self))
            }
        }

        let result = try await state.manager.process(audio: samples)

        var speakerSet: Set<String> = []
        var segments: [DiarizeResponse.Segment] = []

        for segment in result.segments {
            let label = segment.speaker
            speakerSet.insert(label)
            segments.append(.init(speaker: label, start: segment.start, end: segment.end))
        }

        let response = DiarizeResponse(speakers: speakerSet.count, segments: segments)
        let json = try JSONEncoder().encode(response)

        return Response(
            status: .ok,
            headers: [.contentType: "application/json"],
            body: .init(byteBuffer: ByteBuffer(data: json))
        )
    }
}
