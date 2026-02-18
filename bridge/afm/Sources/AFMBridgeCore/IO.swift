import Foundation

public enum IO {
    /// Reads all bytes from stdin (blocks until EOF).
    public static func readStdin() -> Data {
        return FileHandle.standardInput.readDataToEndOfFile()
    }

    /// Encodes `value` as JSON and writes it to stdout followed by a newline.
    public static func writeJSON<T: Encodable>(_ value: T) throws {
        let encoder = JSONEncoder()
        // Produce compact output to keep the bridge contract simple.
        encoder.outputFormatting = []
        let data = try encoder.encode(value)
        FileHandle.standardOutput.write(data)
        FileHandle.standardOutput.write(Data([0x0A])) // '\n'
    }

    /// Writes `message` to stderr and exits with the given code.
    public static func exitWithError(_ message: String, code: Int32 = 1) -> Never {
        let line = message + "\n"
        if let data = line.data(using: .utf8) {
            FileHandle.standardError.write(data)
        }
        exit(code)
    }
}
