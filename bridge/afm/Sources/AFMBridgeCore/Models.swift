import Foundation

// BridgeRequest mirrors the Go afmRequest struct in internal/provider/afm.go.
// Snake_case JSON keys match what the Go side encodes.
public struct BridgeRequest: Decodable, Sendable {
    public let model: String
    public let messages: [BridgeMessage]
    public let expectJSON: Bool

    enum CodingKeys: String, CodingKey {
        case model
        case messages
        case expectJSON = "expect_json"
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        model = try c.decode(String.self, forKey: .model)
        messages = try c.decode([BridgeMessage].self, forKey: .messages)
        // expect_json is optional in the Go struct (omitempty), default false.
        expectJSON = try c.decodeIfPresent(Bool.self, forKey: .expectJSON) ?? false
    }
}

public struct BridgeMessage: Decodable, Sendable {
    public let role: String
    public let content: String
}

// BridgeResponse mirrors what internal/provider/afm.go decodes from stdout.
public struct BridgeResponse: Encodable, Sendable {
    public let content: String
    public let finishReason: String?
    public let usage: BridgeUsage?
    public var contextTrimmed: Bool?

    enum CodingKeys: String, CodingKey {
        case content
        case finishReason = "finish_reason"
        case usage
        case contextTrimmed = "context_trimmed"
    }

    public init(content: String, finishReason: String? = nil, usage: BridgeUsage? = nil, contextTrimmed: Bool? = nil) {
        self.content = content
        self.finishReason = finishReason
        self.usage = usage
        self.contextTrimmed = contextTrimmed
    }
}

public struct BridgeUsage: Encodable, Sendable {
    public let inputTokens: Int
    public let outputTokens: Int
    public let totalTokens: Int

    enum CodingKeys: String, CodingKey {
        case inputTokens = "input_tokens"
        case outputTokens = "output_tokens"
        case totalTokens = "total_tokens"
    }

    public init(inputTokens: Int, outputTokens: Int) {
        self.inputTokens = inputTokens
        self.outputTokens = outputTokens
        self.totalTokens = inputTokens + outputTokens
    }
}

// AvailabilityResponse is written to stdout for --check-availability.
public struct AvailabilityResponse: Encodable, Sendable {
    public let available: Bool
    public let reason: String?

    public init(available: Bool, reason: String? = nil) {
        self.available = available
        self.reason = reason
    }
}
