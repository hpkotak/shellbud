import Testing
import Foundation
@testable import AFMBridgeCore

// MARK: - BridgeRequest decoding

@Suite("BridgeRequest decoding")
struct BridgeRequestTests {
    @Test("decodes all fields")
    func decodesAllFields() throws {
        let json = #"{"model":"default","messages":[{"role":"user","content":"list files"}],"expect_json":true}"#
        let data = Data(json.utf8)
        let req = try JSONDecoder().decode(BridgeRequest.self, from: data)

        #expect(req.model == "default")
        #expect(req.messages.count == 1)
        #expect(req.messages[0].role == "user")
        #expect(req.messages[0].content == "list files")
        #expect(req.expectJSON == true)
    }

    @Test("expect_json defaults to false when omitted")
    func expectJSONDefaultsFalse() throws {
        let json = #"{"model":"default","messages":[]}"#
        let req = try JSONDecoder().decode(BridgeRequest.self, from: Data(json.utf8))
        #expect(req.expectJSON == false)
    }

    @Test("decodes multiple messages")
    func decodesMultipleMessages() throws {
        let json = """
        {
          "model": "default",
          "messages": [
            {"role": "system", "content": "You are helpful."},
            {"role": "user", "content": "Hello"},
            {"role": "assistant", "content": "Hi!"},
            {"role": "user", "content": "What time is it?"}
          ],
          "expect_json": false
        }
        """
        let req = try JSONDecoder().decode(BridgeRequest.self, from: Data(json.utf8))
        #expect(req.messages.count == 4)
        #expect(req.messages[0].role == "system")
        #expect(req.messages[3].content == "What time is it?")
    }

    @Test("fails with missing required fields")
    func failsWithMissingModel() {
        let json = #"{"messages":[]}"#
        #expect(throws: (any Error).self) {
            _ = try JSONDecoder().decode(BridgeRequest.self, from: Data(json.utf8))
        }
    }
}

// MARK: - BridgeResponse encoding

@Suite("BridgeResponse encoding")
struct BridgeResponseTests {
    @Test("encodes with snake_case keys")
    func encodesSnakeCaseKeys() throws {
        let response = BridgeResponse(
            content: #"{"text":"list files","commands":["ls -la"]}"#,
            finishReason: "stop",
            usage: BridgeUsage(inputTokens: 10, outputTokens: 20)
        )
        let data = try JSONEncoder().encode(response)
        let dict = try JSONSerialization.jsonObject(with: data) as! [String: Any]

        #expect(dict["content"] as? String != nil)
        #expect(dict["finish_reason"] as? String == "stop")
        let usage = dict["usage"] as? [String: Any]
        #expect(usage?["input_tokens"] as? Int == 10)
        #expect(usage?["output_tokens"] as? Int == 20)
        #expect(usage?["total_tokens"] as? Int == 30)
        // Verify camelCase keys are NOT present
        #expect(dict["finishReason"] == nil)
        #expect((usage)?["inputTokens"] == nil)
    }

    @Test("encodes without optional fields when nil")
    func encodesWithoutNilFields() throws {
        let response = BridgeResponse(content: "hello")
        let data = try JSONEncoder().encode(response)
        let dict = try JSONSerialization.jsonObject(with: data) as! [String: Any]

        #expect(dict["content"] as? String == "hello")
        #expect(dict["finish_reason"] == nil)
        #expect(dict["usage"] == nil)
    }
}

// MARK: - AvailabilityResponse encoding

@Suite("AvailabilityResponse encoding")
struct AvailabilityResponseTests {
    @Test("available true, no reason")
    func encodesAvailableTrue() throws {
        let resp = AvailabilityResponse(available: true)
        let data = try JSONEncoder().encode(resp)
        let dict = try JSONSerialization.jsonObject(with: data) as! [String: Any]
        #expect(dict["available"] as? Bool == true)
        #expect(dict["reason"] == nil)
    }

    @Test("available false with reason")
    func encodesAvailableFalse() throws {
        let resp = AvailabilityResponse(available: false, reason: "device_not_eligible")
        let data = try JSONEncoder().encode(resp)
        let dict = try JSONSerialization.jsonObject(with: data) as! [String: Any]
        #expect(dict["available"] as? Bool == false)
        #expect(dict["reason"] as? String == "device_not_eligible")
    }
}

// MARK: - Round-trip

@Suite("JSON round-trip")
struct RoundTripTests {
    @Test("request decode → response encode preserves content")
    func roundTrip() throws {
        let requestJSON = #"{"model":"default","messages":[{"role":"user","content":"ls"}],"expect_json":true}"#
        let req = try JSONDecoder().decode(BridgeRequest.self, from: Data(requestJSON.utf8))

        // Simulate the bridge producing a structured response.
        let content = #"{"text":"List files","commands":["ls -la"]}"#
        let response = BridgeResponse(content: content, finishReason: "stop")

        let responseData = try JSONEncoder().encode(response)
        let dict = try JSONSerialization.jsonObject(with: responseData) as! [String: Any]

        // The Go side reads "content" and parses the inner JSON.
        let decoded = dict["content"] as? String
        #expect(decoded == content)
        _ = req  // consumed
    }
}
