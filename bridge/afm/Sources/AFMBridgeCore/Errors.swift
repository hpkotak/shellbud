import Foundation

public enum BridgeError: Error, CustomStringConvertible, Sendable {
    case emptyInput
    case decodingFailed(String)
    case encodingFailed(String)
    case modelUnavailable(String)
    case inferenceError(String)

    public var description: String {
        switch self {
        case .emptyInput:
            return "no input received on stdin"
        case .decodingFailed(let msg):
            return "failed to decode request: \(msg)"
        case .encodingFailed(let msg):
            return "failed to encode response: \(msg)"
        case .modelUnavailable(let reason):
            return "model unavailable: \(reason)"
        case .inferenceError(let msg):
            return "inference error: \(msg)"
        }
    }
}
