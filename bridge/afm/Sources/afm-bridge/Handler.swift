import Foundation
import FoundationModels
import AFMBridgeCore

// ShellBudResponse is the @Generable type used for structured JSON generation.
// Its fields match the Go prompt contract: {"text":"...","commands":["..."]}.
@Generable
struct ShellBudResponse {
    @Guide(description: "Concise user-facing guidance — what the user should know")
    var text: String

    @Guide(description: "Zero or more executable shell commands. Empty array if no command is needed.")
    var commands: [String]
}

@available(macOS 26.0, *)
struct Handler {
    // MARK: - Availability check

    func checkAvailability() {
        let availability = SystemLanguageModel.default.availability
        let response: AvailabilityResponse

        switch availability {
        case .available:
            response = AvailabilityResponse(available: true)
        case .unavailable(let reason):
            let reasonStr: String
            switch reason {
            case .deviceNotEligible:          reasonStr = "device_not_eligible"
            case .appleIntelligenceNotEnabled: reasonStr = "apple_intelligence_not_enabled"
            case .modelNotReady:              reasonStr = "model_not_ready"
            @unknown default:                 reasonStr = "unknown"
            }
            response = AvailabilityResponse(available: false, reason: reasonStr)
        }

        if let data = try? JSONEncoder().encode(response),
           let str = String(data: data, encoding: .utf8) {
            print(str)
        } else {
            print(#"{"available":false,"reason":"encoding_error"}"#)
        }
    }

    // MARK: - Inference entry point

    func handleRequest() async {
        do {
            let inputData = IO.readStdin()
            guard !inputData.isEmpty else {
                throw BridgeError.emptyInput
            }

            let request: BridgeRequest
            do {
                request = try JSONDecoder().decode(BridgeRequest.self, from: inputData)
            } catch {
                throw BridgeError.decodingFailed(error.localizedDescription)
            }

            let response = try await infer(request: request)
            try IO.writeJSON(response)
        } catch {
            IO.exitWithError(error.localizedDescription)
        }
    }

    // MARK: - Core inference

    private func infer(request: BridgeRequest) async throws -> BridgeResponse {
        let availability = SystemLanguageModel.default.availability
        guard case .available = availability else {
            throw BridgeError.modelUnavailable(availabilityReason(availability))
        }

        // Split system message from conversation history.
        let (systemPrompt, history) = extractContext(from: request.messages)

        guard let lastMessage = history.last else {
            throw BridgeError.emptyInput
        }
        guard lastMessage.role == "user" else {
            throw BridgeError.decodingFailed("last message must be from user, got \"\(lastMessage.role)\"")
        }
        let prompt = lastMessage.content

        // Build a Transcript from prior turns so the session has full conversation state.
        let session = buildSession(systemPrompt: systemPrompt, history: history)

        // Eagerly load model resources to reduce first-call latency.
        session.prewarm()

        do {
            if request.expectJSON {
                return try await inferStructured(session: session, prompt: prompt)
            } else {
                return try await inferPlain(session: session, prompt: prompt)
            }
        } catch LanguageModelSession.GenerationError.exceededContextWindowSize {
            // The 4096 token limit was exceeded. Retry with just instructions + last message.
            let freshSession = LanguageModelSession(instructions: systemPrompt)
            var response: BridgeResponse
            if request.expectJSON {
                response = try await inferStructured(session: freshSession, prompt: prompt)
            } else {
                response = try await inferPlain(session: freshSession, prompt: prompt)
            }
            response.contextTrimmed = true
            return response
        }
    }

    // MARK: - Structured generation (expect_json=true)

    private func inferStructured(session: LanguageModelSession, prompt: String) async throws -> BridgeResponse {
        do {
            let result = try await session.respond(to: prompt, generating: ShellBudResponse.self)
            // respond(to:generating:) returns Response<ShellBudResponse>; .content unwraps it.
            let jsonContent = try encodeAsJSONString(result.content)
            return BridgeResponse(content: jsonContent, finishReason: "stop")
        } catch {
            // Fallback: if structured generation fails, wrap plain response in minimal JSON.
            // This keeps the Go fail-closed parser happy: it will parse the text field and
            // return no commands (safe default).
            let fallback = try await inferPlain(session: session, prompt: prompt)
            let fallbackShellBud = ShellBudResponse(text: fallback.content, commands: [])
            let jsonContent = try encodeAsJSONString(fallbackShellBud)
            return BridgeResponse(content: jsonContent, finishReason: "stop")
        }
    }

    // MARK: - Plain text generation (expect_json=false)

    private func inferPlain(session: LanguageModelSession, prompt: String) async throws -> BridgeResponse {
        do {
            let response = try await session.respond(to: prompt)
            return BridgeResponse(content: response.content, finishReason: "stop")
        } catch {
            throw BridgeError.inferenceError(error.localizedDescription)
        }
    }

    // MARK: - Helpers

    /// Extracts the system role message (instructions) and remaining messages.
    private func extractContext(from messages: [BridgeMessage]) -> (system: String, history: [BridgeMessage]) {
        var systemPrompt = ""
        var history: [BridgeMessage] = []
        for msg in messages {
            if msg.role == "system" && systemPrompt.isEmpty {
                systemPrompt = msg.content
            } else {
                history.append(msg)
            }
        }
        return (systemPrompt, history)
    }

    /// Builds a LanguageModelSession with a Transcript reconstructed from prior turns.
    /// The final user message is NOT included — it gets passed to session.respond() separately.
    private func buildSession(systemPrompt: String, history: [BridgeMessage]) -> LanguageModelSession {
        var entries: [Transcript.Entry] = []

        if !systemPrompt.isEmpty {
            let segments = [Transcript.Segment.text(Transcript.TextSegment(content: systemPrompt))]
            entries.append(.instructions(Transcript.Instructions(segments: segments, toolDefinitions: [])))
        }

        // Add all turns except the last (which will be passed to respond()).
        for msg in history.dropLast() {
            let segments = [Transcript.Segment.text(Transcript.TextSegment(content: msg.content))]
            if msg.role == "user" {
                entries.append(.prompt(Transcript.Prompt(segments: segments)))
            } else if msg.role == "assistant" {
                entries.append(.response(Transcript.Response(assetIDs: [], segments: segments)))
            }
        }

        let transcript = Transcript(entries: entries)
        return LanguageModelSession(transcript: transcript)
    }

    private func availabilityReason(_ availability: SystemLanguageModel.Availability) -> String {
        switch availability {
        case .available: return "available"
        case .unavailable(let reason):
            switch reason {
            case .deviceNotEligible:           return "device_not_eligible"
            case .appleIntelligenceNotEnabled:  return "apple_intelligence_not_enabled"
            case .modelNotReady:               return "model_not_ready"
            @unknown default:                  return "unknown"
            }
        }
    }

    /// Encodes a ShellBudResponse to the compact JSON string the Go side parses.
    private func encodeAsJSONString(_ response: ShellBudResponse) throws -> String {
        struct Wire: Encodable {
            let text: String
            let commands: [String]
        }
        let wire = Wire(text: response.text, commands: response.commands)
        let data = try JSONEncoder().encode(wire)
        guard let str = String(data: data, encoding: .utf8) else {
            throw BridgeError.encodingFailed("UTF-8 encoding failed")
        }
        return str
    }
}
