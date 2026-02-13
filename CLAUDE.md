# ShellBud

CLI tool that translates natural language to shell commands. Binary name: `sb`.

## Build & Test

```bash
go build -o sb .        # Build
go test ./...            # Run all tests
go vet ./...             # Static analysis
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
