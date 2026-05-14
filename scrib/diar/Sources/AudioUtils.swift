import FluidAudio
import Foundation

enum AudioUtils {
    static func isWAV(data: Data) -> Bool {
        guard data.count >= 12 else { return false }
        let riff = String(data: data[0..<4], encoding: .ascii)
        let wave = String(data: data[8..<12], encoding: .ascii)
        return riff == "RIFF" && wave == "WAVE"
    }

    static func convertWAV(data: Data) throws -> [Float] {
        let tempDir = FileManager.default.temporaryDirectory
        let tempFile = tempDir.appendingPathComponent(UUID().uuidString + ".wav")
        try data.write(to: tempFile)
        defer { try? FileManager.default.removeItem(at: tempFile) }
        return try AudioConverter.resampleAudioFile(path: tempFile.path)
    }
}
