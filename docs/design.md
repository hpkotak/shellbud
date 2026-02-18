# ShellBud Design Document

## Problem

Shell commands are hard to remember, and most existing AI shell tools are either one-shot translators (crowded space) or generic chatbot wrappers with no awareness of your actual environment. You end up typing a query, getting a command, and still having to mentally verify it's right for your OS, directory, and context.

## Solution

`sb` (ShellBud) is a context-aware shell assistant with two modes:

**One-shot** — quick questions from your normal shell:
```
$ sb what git branch am I on

You're on the main branch.
```

**Chat** — interactive sessions for complex tasks:
```
$ sb chat
sb> find the largest files in this project

  > find . -type f -exec du -h {} + | sort -rh | head -20
  [r]un / [e]xplain / [s]kip: r

(output displayed)

sb> what was the biggest one?
```

The key differentiator is **deep environment awareness**: ShellBud knows your cwd, git state, directory contents, OS, shell, and architecture. Commands are tailored to your actual context, not generic.

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                     CLI Layer (cmd/)                      │
│  root.go: one-shot query → chat → parse → confirm → exec │
│  chat.go: delegates to repl.Run()                        │
│  setup.go / config.go: onboarding + settings             │
└───────┬───────┬──────────┬──────────┬──────────┬─────────┘
        │       │          │          │          │
   ┌────▼──┐ ┌──▼───┐ ┌───▼────┐ ┌───▼─────┐ ┌──▼──────┐
   │Provider│ │ShellEnv│ │ REPL   │ │ Safety  │ │ Config  │
   │        │ │       │ │        │ │         │ │         │
   │Chat API│ │Gather │ │Run/    │ │ Regex   │ │ YAML    │
   │Message │ │Format │ │Explain/│ │patterns │ │load/save│
   │[]      │ │       │ │Skip    │ │         │ │         │
   │Ollama  │ └───────┘ └───┬────┘ └─────────┘ └─────────┘
   │AFM     │               │
   └───┬────┘               │
       │              ┌─────▼────┐
  ┌────▼────┐         │ Executor │
  │ Prompt  │         │          │
  │         │         │Confirm   │
  │ChatSys  │         │Run       │
  │PromptFn │         │RunCapture│
  │ParseChat│         └──────────┘
  │Response │
  └─────────┘
```

## Key Design Decisions

### 1. Provider Abstraction

The `Provider` interface decouples the CLI from any specific LLM backend:

```go
type Message struct {
    Role    string // "system", "user", "assistant"
    Content string
}

type ChatRequest struct {
    Messages   []Message
    Model      string
    ExpectJSON bool
}

type ChatResponse struct {
    Text         string
    Raw          string
    Structured   bool
    FinishReason string
    Usage        Usage
    Warning      string // provider-level notices (e.g., context was trimmed)
}

type Provider interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
    Name() string
    Capabilities() Capabilities
    Available(ctx context.Context) error
}
```

`Message` is defined in the `provider` package so callers stay decoupled from backend-specific SDK types. Each provider implementation performs its own conversion.

**Why this matters:** Adding a new LLM backend means implementing the typed provider contract and wiring one constructor in the provider factory. The CLI code stays unchanged.

### 2. Provider Backends

Current backends:
- `ollama` via Ollama `Chat` API
- `openai` via Chat Completions API
- `afm` via a Swift bridge executable for Apple Foundation Models (macOS 26+, Apple Silicon)

One-shot mode is a single-turn chat for all providers. Chat mode reuses the same provider interface with conversation history.

#### AFM Bridge

FoundationModels.framework is Swift-only and macOS 26+. Rather than linking Swift into the Go binary, `sb` launches a standalone Swift executable (`afm-bridge`) and communicates via stdin/stdout JSON. The Go process writes a request to the bridge's stdin and reads a response from stdout.

The bridge supports an availability probe: `afm-bridge --check-availability` returns `{"available": true}` or `{"available": false, "reason": "device_not_eligible"}`. Setup uses this to decide whether to offer AFM.

The on-device model has a ~4096 token context window. When conversation history exceeds this, the bridge retries with a fresh session (system prompt + latest user message only) and sets `context_trimmed: true` in the response. The Go side surfaces this as `Warning` on `ChatResponse`, displayed separately from the response text.

The system message is rebuilt every turn with fresh environment context (cwd and git state change as commands execute), while conversation history is kept separate and capped at 50 messages.

### 3. Environment Context (the differentiator)

The `shellenv` package gathers a best-effort snapshot before each LLM call:

| Field | Source | Timeout |
|-------|--------|---------|
| CWD | `os.Getwd()` | instant |
| Directory listing | `ls -la` (first 50 lines) | 2s |
| Git branch | `git rev-parse --abbrev-ref HEAD` | 2s |
| Git dirty | `git status --porcelain` | 2s |
| Recent commits | `git log --oneline -5` | 2s |
| OS / Arch / Shell | `runtime.GOOS` / `runtime.GOARCH` / `$SHELL` | instant |
| Env vars | Allowlisted: EDITOR, VISUAL, LANG, TERM, HOME, USER | instant |

Individual failures are swallowed — not in a git repo? `GitBranch` is just empty. The snapshot is always best-effort, never an error.

### 4. Safety: Regex, Not LLM

Destructive command detection uses compiled regex patterns, not LLM classification.

**Why:** Safety checks must be deterministic, fast, and independent of the LLM. A regex match on `rm`, `sudo`, `dd` etc. is predictable and testable. Trusting the LLM to classify its own output would be circular.

- One-shot mode: safe commands show "Run this? [Y/n]" (default yes), destructive show "Are you sure? [y/N]" (default no)
- Chat mode: all commands show "[r]un / [e]xplain / [s]kip", destructive commands require an additional "Are you sure? [y/N]" confirmation after choosing run

See [docs/decisions.md](decisions.md) for the documented decision to stay with regex over shell AST parsing (`mvdan.cc/sh`).

### 5. Structured Response Parsing (Fail Closed)

The LLM is instructed to return only JSON with this schema:

```json
{"text":"...","commands":["..."]}
```

Providers also return normalized metadata (`finish_reason`, usage, structured-output validity) in `ChatResponse`. Parsing safety still remains prompt-parser driven and fail-closed.

`ParseChatResponse()` uses `json.Unmarshal` to validate this contract, then normalizes command strings (trim + drop empties).

Execution safety rule:
- **Only commands from valid structured JSON are executable.**
- If output is malformed or unstructured, ShellBud still displays it, but does not offer execution prompts (fail closed).

Responses can be:
- **Valid JSON, no commands** — display `text`, no action prompt.
- **Valid JSON, commands present** — display `text`, then each command gets run/explain/skip.
- **Invalid JSON** — raw response displayed, no action prompt.

### 6. Output Capture (chat mode)

`RunCapture()` uses `io.MultiWriter` to simultaneously display output to the terminal and buffer it. The captured output (truncated at 8KB) is added to conversation history as a user message so the LLM can reference it in follow-up turns.

### 7. Config: YAML, Not Viper

Three config fields don't need a framework. Raw `gopkg.in/yaml.v3` is simpler and has fewer dependencies.

### 8. Setup Flow

First-run setup handles the entire onboarding:

1. **Platform check:** on macOS, look for `afm-bridge` (PATH first, then `~/.shellbud/bin/`). If found, run `--check-availability`.
2. **Provider choice (macOS only):** if AFM is available, present a menu — AFM (default) or Ollama. If AFM is unavailable or the bridge isn't found, fall through to Ollama silently.
3. **AFM path:** save config with `provider=afm` and the resolved bridge path, done.
4. **Ollama path (all platforms):** detect if Ollama is installed → offer to install. Detect if running → offer to start. Check for models → offer to pull. User picks model → save config.

All actions require user consent. The tool never installs or modifies anything silently.

## Data Flow

### One-Shot Mode (`sb "query"`)

```
User input          "what git branch am I on"
    │
    ▼
Config load         ~/.shellbud/config.yaml → provider, model, host
    │
    ▼
Environment         shellenv.Gather() → cwd, git, dir listing, OS, env vars
    │
    ▼
Build messages      [system: ChatSystemPrompt(env), user: query]
    │
    ▼
Provider.Chat       Messages → selected provider backend → assistant response
    │
    ▼
ParseChatResponse   Validate JSON schema, normalize commands
    │
    ├─ Invalid JSON → display raw text, done (fail closed)
    │
    ├─ No commands → display text, done
    │
    └─ Commands found → for each:
        │
        ▼
    Safety.Classify     Regex patterns → Safe or Destructive
        │
        ▼
    Confirm             Run this? / Are you sure?
        │
        ▼
    executor.Run        $SHELL -c "command" (inherits stdio)
```

### Chat Mode (`sb chat`)

```
sb> prompt
    │
    ▼
Environment refresh    shellenv.Gather() (fresh each turn)
    │
    ▼
Build messages         [system: fresh env context] + [history] + [user: input]
    │
    ▼
Provider.Chat          → selected provider backend → response
    │
    ▼
Add to history         assistant message appended (capped at 50)
    │
    ▼
ParseChatResponse      Validate JSON schema, normalize commands
    │
    ├─ Invalid JSON → display raw text
    │
    ├─ No commands → display text
    │
    └─ Commands found → for each:
        │
        ▼
    [r]un / [e]xplain / [s]kip
        │
        ├─ Run → RunCapture() → output displayed AND added to history
        ├─ Explain → immediate LLM call → parsed text displayed
        └─ Skip → continue
    │
    ▼
Loop back to sb> prompt
```

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/ollama/ollama/api` | Ollama Chat API client |
| `gopkg.in/yaml.v3` | Config file parsing |

The Swift bridge (`bridge/afm/`) is not a Go dependency — it is a standalone executable built separately (`make build-bridge`) and distributed alongside the Go binary via Homebrew on macOS arm64.

## Quality Gates

AI-assisted changes are constrained by hard validation gates:

- `make validate` runs format checks, vet, tests, race tests, lint, and coverage checks.
- Coverage thresholds are enforced by `scripts/check_coverage.sh`:
  - total `>= 85%`
  - critical packages `>= 90%` (`cmd`, `internal/repl`, `internal/provider`, `internal/safety`, `internal/prompt`)
  - `internal/setup >= 70%` temporary floor
- CI jobs (`format`, `test`, `lint`, `coverage`) run on PRs and pushes to `main`.
- CODEOWNERS protects high-risk runtime paths with required owner review when branch protection enables it.
- Release workflow (`.github/workflows/release.yml`) runs the same `make validate` gate before publishing.

## Distribution

Releases are built by [goreleaser](https://goreleaser.com) and distributed via Homebrew:

```bash
brew install hpkotak/tap/sb
```

goreleaser overrides the binary name from `shellbud` (the Go module name) to `sb` using the `binary` field in `.goreleaser.yml`. Binaries are built for darwin/linux on amd64/arm64 with version injected via ldflags (`-X github.com/hpkotak/shellbud/cmd.version`).

See [docs/release-policy.md](release-policy.md) for the full release process.
