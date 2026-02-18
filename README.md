# ShellBud (`sb`)

A context-aware shell assistant — interact with your shell in natural language.

```
$ sb what git branch am I on

You're on the main branch.
```

    $ sb chat
    ShellBud Chat (type 'exit' to quit)

    sb> find the largest files in this project

      > find . -type f -exec du -h {} + | sort -rh | head -20
      [r]un / [e]xplain / [s]kip: r

    (output displayed)

    sb> what was the biggest one?

## Features

- **Two modes**: one-shot (`sb "query"`) and interactive chat (`sb chat`)
- **Pluggable providers**: `ollama`, `openai`, or `afm` bridge command
- **Context-aware**: knows your cwd, git branch, directory contents, OS, and shell
- **Conversational**: chat mode remembers what you asked and what commands produced
- **Safe**: destructive commands (`rm`, `sudo`, `dd`) require double confirmation
- **Fail-closed execution**: commands run only when the model returns valid structured output
- **Injection-hardened**: untrusted env data (commit messages, filenames, env vars) is delimited and sanitized before reaching the LLM
- **Preflight checks**: provider availability verified before first query — misconfiguration fails fast with an actionable `sb setup` hint
- **Offline-capable**: runs entirely on-device with `ollama` or Apple Foundation Models (`afm`)
- **Run / Explain / Skip**: review commands before executing, ask for explanations

## Safety Model

ShellBud asks the model for structured responses:

```json
{"text":"...","commands":["..."]}
```

ShellBud enforces JSON mode per provider when available (`ollama`/`openai`) and passes `expect_json` to `afm` bridges. Provider responses are normalized before command parsing.

Only commands from valid structured responses are executable. If the model returns malformed or unstructured output, ShellBud still displays it, but does not offer command execution.

`explain` responses in chat mode are displayed as plain assistant text and are never treated as executable command payloads.

### Prompt Injection Hardening

Environment data gathered from the shell (git commit messages, branch names, directory listings, env var values) is potentially untrusted. ShellBud defends against prompt injection in two ways:

1. **Newline sanitization** — embedded newlines in untrusted fields are replaced with ` ↵ ` before the snapshot is embedded in the system prompt. This prevents injected content from appearing as a new line (and thus a new instruction) in the prompt.
2. **Explicit delimiters** — the entire environment block is wrapped in `<environment>...</environment>` XML tags with an explicit instruction to the model to treat that content as opaque data, not instructions.

## Install

### Homebrew (macOS/Linux)

```bash
brew install hpkotak/tap/sb
```

On macOS Apple Silicon, this also installs `afm-bridge` for Apple Foundation Models support.

### From Source

Requires [Go 1.22+](https://go.dev/dl/).

```bash
go install github.com/hpkotak/shellbud@latest
GOBIN="${GOBIN:-$(go env GOPATH)/bin}" && mv "$GOBIN/shellbud" "$GOBIN/sb"
```

### Prerequisites

- **Ollama (default):** [Ollama](https://ollama.com) must be installed separately. Works on macOS and Linux.
- **AFM (macOS only):** Requires macOS 26+ on Apple Silicon with Apple Intelligence enabled. No additional install needed — the bridge is included via Homebrew or can be built with `make install-bridge`.

## Quick Start

```bash
# First-time setup (on macOS, offers AFM or Ollama; configures chosen provider)
sb setup

# One-shot: ask a question, get a command
sb find all log files larger than 100MB
sb show disk usage sorted by size
sb what's using port 8080

# Interactive chat session
sb chat

# Override model for a single query
sb --model codellama:7b write a bash loop from 1 to 10
```

## Configuration

Config lives at `~/.shellbud/config.yaml`.

```bash
sb config show                          # View current config
sb config set provider ollama           # ollama | openai | afm
sb config set model codellama:7b        # Change default model
sb config set ollama.host http://host:11434  # Custom Ollama host
sb config set openai.host https://api.openai.com/v1
sb config set afm.command ~/.shellbud/bin/afm-bridge
```

Provider notes:
- `openai` reads API key from `OPENAI_API_KEY`.
- `afm` uses a Swift bridge for Apple Foundation Models. The bridge path defaults to `afm-bridge` (found via PATH or `~/.shellbud/bin/`). `sb setup` configures this automatically on macOS.

## Architecture

See [docs/design.md](docs/design.md) for architecture decisions and design rationale.

## Releases

Tagged releases are built automatically via [goreleaser](https://goreleaser.com) and published to [GitHub Releases](https://github.com/hpkotak/shellbud/releases). The Homebrew formula is updated automatically on each release.

For release process details, see [docs/release-policy.md](docs/release-policy.md).

## Development Validation

Use this before opening a PR or publishing:

```bash
make validate
```

This runs:
- `goimports` check (`fmt-check`)
- `go vet ./...`
- `go test ./...`
- `go test -race ./...`
- `golangci-lint run ./...`
- coverage gate (`scripts/check_coverage.sh`)

GitHub Actions runs the same validation on every pull request and on pushes to `main`.

Coverage thresholds:
- Total: `>= 85%`
- Critical packages (`cmd`, `internal/repl`, `internal/provider`, `internal/safety`, `internal/prompt`): `>= 90%`
- `internal/setup`: `>= 70%` (temporary floor)

To run the same checks on every local commit:

```bash
make hooks
```

This configures Git to use `.githooks/pre-commit`, which runs `make validate`.

For branch protection and risk-based review setup, see [docs/release-policy.md](docs/release-policy.md).

## License

MIT
