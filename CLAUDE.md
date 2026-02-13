# ShellBud

CLI tool that translates natural language to shell commands. Binary name: `sb`.

## Build & Test

```bash
go build -o sb .              # Build
go test ./...                  # Run all tests
go vet ./...                   # Static analysis
golangci-lint run ./...        # Lint (must pass before commit)
```

## Project Structure

- `main.go` — Entry point, delegates to `cmd.Execute()`
- `cmd/` — Cobra command handlers (root, setup, config)
- `internal/provider/` — LLM provider interface + Ollama implementation
- `internal/prompt/` — System prompt and LLM response parsing
- `internal/safety/` — Destructive command detection (regex-based)
- `internal/executor/` — Command confirmation and execution
- `internal/config/` — Config file management (~/.shellbud/config.yaml)
- `internal/setup/` — First-run interactive setup flow
- `internal/platform/` — OS/shell detection

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

## Pre-Commit Checklist

1. `go build -o sb .` — compiles
2. `go vet ./...` — no issues
3. `go test ./...` — all pass
4. `golangci-lint run ./...` — 0 issues
5. No binaries staged (`sb`, `*.exe`, `*.test`)
6. No secrets staged (`.env`, API keys)
7. README.md exists and is current
