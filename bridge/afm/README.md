# afm-bridge

Swift CLI that bridges Go to [Apple Foundation Models](https://developer.apple.com/documentation/foundationmodels) (macOS 26+, Apple Silicon).

The Go process (`sb`) cannot link Swift frameworks directly. Instead it spawns `afm-bridge`
as a subprocess and communicates over stdin/stdout JSON.

## Requirements

- macOS 26+ on Apple Silicon
- Xcode 26 (FoundationModels SDK)
- Apple Intelligence enabled in System Settings

## Package Structure

```
bridge/afm/
├── Sources/
│   ├── AFMBridgeCore/    # Testable library — models, IO, errors (no FoundationModels dep)
│   └── afm-bridge/       # Executable — wires AFMBridgeCore to FoundationModels.framework
└── Tests/
    └── AFMBridgeCoreTests/
```

`AFMBridgeCore` has no FoundationModels dependency so it can be tested on any macOS version.
The `afm-bridge` executable adds the framework dependency and is only buildable on macOS 26+.

## Build & Test

```bash
# From the repo root:
make build-bridge      # release build → bridge/afm/.build/release/afm-bridge
make install-bridge    # build + copy to ~/.shellbud/bin/

# From bridge/afm/ directly:
swift build -c release --arch arm64
swift test             # runs AFMBridgeCoreTests (no macOS 26 required)
```

## Modes

### Normal mode (stdin → stdout)

Go writes one JSON object to stdin; the bridge writes one JSON object to stdout.

**Request** (Go → bridge):
```json
{"model":"","messages":[{"role":"system","content":"..."},{"role":"user","content":"..."}],"expect_json":false}
```

**Response** (bridge → Go):
```json
{"content":"...","finish_reason":"stop","usage":{"input_tokens":10,"output_tokens":20,"total_tokens":30}}
```

If conversation history exceeds the model's context window (~4096 tokens), the bridge retries
with a fresh session (system prompt + latest user message only) and adds `"context_trimmed":true`
to the response. The Go side surfaces this as a warning to the user.

### Availability probe

```bash
afm-bridge --check-availability
```

Outputs:
```json
{"available":true}
# or
{"available":false,"reason":"device_not_eligible"}
```

Reason values: `device_not_eligible`, `apple_intelligence_not_enabled`, `model_not_ready`.

`sb setup` uses this probe to decide whether to offer AFM as a provider option.
