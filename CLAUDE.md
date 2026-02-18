# ShellBud

Context-aware shell assistant — interact with your shell in natural language. Binary name: `sb`.

## Build & Test

```bash
make build                     # Build sb binary locally
make fmt                       # Auto-format Go files with goimports
make fmt-check                 # Check formatting without modifying files
go test ./...                  # Run all tests
go test -race ./...            # Race detection
go vet ./...                   # Static analysis
golangci-lint run ./...        # Lint (must pass before commit)
make coverage                  # Enforced coverage thresholds
make validate                  # Full validation pipeline
make hooks                     # Install local pre-commit hook (runs make validate)
make release-dry-run           # goreleaser snapshot (no publish)
make build-bridge              # Build afm-bridge Swift binary (macOS only)
make install-bridge            # Build + install afm-bridge to ~/.shellbud/bin/
```

## Project Structure

- `main.go` — Entry point, delegates to `cmd.Execute()`
- `cmd/` — Cobra command tree: root (one-shot), `chat`, `setup`, `config show`, `config set`; version exposed as `--version` flag (Cobra built-in via `rootCmd.Version`)
  - `Execute()` is the entry point; `--model` flag overrides config model for query and chat commands
  - One-shot (`sb "query"`) gathers env context, sends single LLM request, offers run/skip per command
- `internal/provider/` — LLM provider interface + Ollama/OpenAI/AFM implementations
  - `Provider` interface: `Chat(ctx, ChatRequest) (ChatResponse, error)`, `Name() string`, `Capabilities() Capabilities`, `Available(ctx) error`
  - `Message{Role, Content}`, `ChatRequest{Messages, Model, ExpectJSON}`, `ChatResponse{Text, Raw, Structured, FinishReason, Usage, Warning}`
  - `Capabilities{JSONMode, Usage, FinishReason}`, `Usage{InputTokens, OutputTokens, TotalTokens}`
  - `BuildConfig{Name, Model, OllamaHost, OpenAIHost, OpenAIAPIKey, AFMCommand}` → `NewFromConfig(BuildConfig) (Provider, error)`
- `internal/config/` — Config file management (~/.shellbud/config.yaml)
  - `Config{Provider, Model, Ollama{Host}, OpenAI{Host}, AFM{Command}}` — yaml-tagged
  - `Load() (*Config, error)`, `Save(*Config) error`, `Default() *Config`, `Validate() error`
  - `ErrNotFound` — returned when config file doesn't exist (distinct from parse errors)
  - `ValidProviders = ["ollama", "openai", "afm"]`; defaults: `DefaultProvider`, `DefaultModel`, `DefaultOllamaHost`, etc.
- `internal/prompt/` — System prompt + structured response parser (JSON contract, fail-closed)
  - `ChatSystemPrompt(envContext string) string` — builds system prompt with env snapshot
  - `ParseChatResponse(raw string) ParsedResponse` — extracts commands only from valid JSON
  - `ParsedResponse{Text, Commands []string, Structured bool}` — Structured=false → no commands offered
  - JSON contract: `{"text":"...","commands":["..."]}` — fail-closed: invalid JSON = display-only
- `internal/shellenv/` — Environment context gathering (best-effort, errors swallowed)
  - `Snapshot{CWD, DirList, OS, Shell, Arch, GitBranch, GitDirty, GitRecent, Env map[string]string}`
  - `Gather() Snapshot`, `(Snapshot).Format() string` — Format renders for system prompt embedding
- `internal/safety/` — Deterministic destructive command detection (regex, not LLM)
  - `Level` type: `Safe`, `Destructive` (iota constants)
  - `Classify(command string) Level` — regex patterns for rm, sudo, dd, mkfs, etc.
- `internal/executor/` — Command confirmation and execution
  - `Confirm(prompt string, defaultYes bool, in io.Reader, out io.Writer) bool`
  - `Run(command string) error` — inherits stdin/stdout/stderr
  - `RunCapture(command string) (output string, exitCode int, err error)` — tees output, truncates at `MaxOutputBytes`
- `internal/repl/` — Interactive chat loop (run/explain/skip flow, history, output capture)
  - `Run(p provider.Provider, in io.Reader, out io.Writer) error`
  - Refreshes env context each turn, maintains conversation history (capped at 50 messages)
- `internal/setup/` — First-run interactive setup (provider detection, model selection)
  - `Run(in io.Reader, out io.Writer) error`
  - On darwin: bridge lookup → `--check-availability` JSON probe → offer AFM or Ollama
- `internal/platform/` — OS/shell detection
- `bridge/afm/` — Swift CLI bridging to Apple Foundation Models (macOS 26+, Apple Silicon)
  - Two targets: `AFMBridgeCore` (testable, no FoundationModels dep) + `afm-bridge` (executable)
  - stdin/stdout JSON contract — Go sends `BridgeRequest{model, messages[], expect_json}`
  - Bridge responds `BridgeResponse{content, finish_reason?, usage?, context_trimmed?}`
  - `--check-availability` mode → `AvailabilityResponse{available, reason?}`

## Conventions

- Standard Go idioms — no Java-in-Go patterns
- `gopkg.in/yaml.v3` for config (not viper)
- Standard `testing` package (not testify)
- Table-driven tests
- `internal/` for all private packages
- Minimal dependencies (cobra, ollama/api, yaml.v3)

## Enforced Patterns

These patterns MUST be applied consistently across the entire codebase.

1. **Never `os.Exit()` outside `main.go`** — return errors up the call chain
2. **All network calls need context with timeout** — no bare `context.Background()` to network calls, no `http.DefaultClient`
3. **Injectable IO everywhere** — functions reading input take `io.Reader`, writing output take `io.Writer`. Reference: `executor.Confirm()`
4. **Constants for defaults** — hardcoded values live in `config.Default()` or package constants, never duplicated across files
5. **Validate at boundaries** — user-provided config values must be validated before saving

## Distribution

**Locked method: goreleaser + Homebrew tap.** Do not introduce alternative distribution mechanisms.

- `.goreleaser.yml` — builds cross-platform binaries named `sb` (overrides module name `shellbud`)
- `.github/workflows/release.yml` — triggered by `v*` tags, runs validation then goreleaser
- Homebrew formula auto-published to `hpkotak/homebrew-tap` via goreleaser
- Version injected at build time: `-X github.com/hpkotak/shellbud/cmd.version`

### Release Flow

```bash
git tag v0.1.0 && git push origin v0.1.0   # Triggers release workflow
```

### Local Testing

```bash
make build                           # Build sb binary locally
make release-dry-run                 # goreleaser snapshot (no publish)
```

### Required GitHub Secrets

- `GITHUB_TOKEN` — automatic in Actions (release assets)
- `TAP_GITHUB_TOKEN` — fine-grained PAT with Contents read/write on `hpkotak/homebrew-tap`

## Local-Only Docs (gitignored)

These files exist on disk but are not tracked in git:

- `docs/next-roadmap-items.md` — open findings, known gaps, suggested next work
- `docs/portfolio.md` — skills narrative and framing for interview context

Both are listed under "Local planning/review artifacts" in `.gitignore`. Read them when planning work or reviewing gaps; do not commit them.

## Git Commits

- Never include `Co-Authored-By` in commit messages

## Pre-Commit Checklist

1. `make build` — compiles
2. `make validate` — format check + vet + tests + race + lint + coverage
3. `make hooks` was run at least once locally
4. No binaries staged (`sb`, `shellbud`, `*.exe`, `*.test`)
5. No secrets staged (`.env`, API keys)
6. README.md exists and is current
