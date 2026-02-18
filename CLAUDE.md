# ShellBud

Context-aware shell assistant — interact with your shell in natural language. Binary name: `sb`.

## Build & Test

```bash
make build                     # Build sb binary locally
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
- `cmd/root.go` — One-shot mode: `sb "query"` (enriched with environment context)
- `cmd/chat.go` — Chat REPL: `sb chat` (conversational with history)
- `cmd/` — Also: setup, config show/set, version commands
- `internal/provider/` — LLM provider interface (Message, Chat) + Ollama Chat API implementation
- `internal/prompt/` — Chat system prompt, structured response parser (JSON contract, fail-closed execution)
- `internal/shellenv/` — Environment context gathering (cwd, git, dir listing, env vars)
- `internal/repl/` — Interactive REPL loop (run/explain/skip flow, history, output capture)
- `internal/safety/` — Destructive command detection (regex-based)
- `internal/executor/` — Command confirmation, execution, and output capture (Run + RunCapture)
- `internal/config/` — Config file management (~/.shellbud/config.yaml)
- `internal/setup/` — First-run interactive setup flow
- `internal/platform/` — OS/shell detection
- `bridge/afm/` — Swift CLI that calls Apple Foundation Models (macOS 26+, Apple Silicon)

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

## Git Commits

- Never include `Co-Authored-By` in commit messages

## Pre-Commit Checklist

1. `make build` — compiles
2. `make validate` — format check + vet + tests + race + lint + coverage
3. `make hooks` was run at least once locally
4. No binaries staged (`sb`, `shellbud`, `*.exe`, `*.test`)
5. No secrets staged (`.env`, API keys)
6. README.md exists and is current
