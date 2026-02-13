# ShellBud Design Document

## Problem

Shell commands are hard to remember. Tools like `tar`, `find`, `ffmpeg`, `awk` have complex flag syntax that you use just often enough to need them but not enough to memorize. The current workflow is: forget syntax, Google it, copy-paste from Stack Overflow, adapt to your use case.

## Solution

`sb` (ShellBud) translates natural language to shell commands using a local LLM.

```
$ sb compress this folder as tar.gz
  tar -czvf archive.tar.gz ./folder

  Run this? [Y/n]:
```

The tool asks for confirmation before executing, with extra safety checks for destructive commands.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    CLI Layer (cmd/)                  │
│  root.go: parse args → translate → safety → execute │
│  setup.go: first-run configuration                  │
│  config.go: view/update settings                    │
└──────────┬──────────┬──────────┬──────────┬─────────┘
           │          │          │          │
     ┌─────▼──┐  ┌────▼───┐ ┌───▼────┐ ┌───▼─────┐
     │Provider│  │ Safety │ │Executor│ │ Config  │
     │        │  │        │ │        │ │         │
     │Ollama  │  │ Regex  │ │Confirm │ │ YAML    │
     │(future:│  │patterns│ │+ exec  │ │load/save│
     │AFM,    │  │        │ │        │ │         │
     │Claude) │  └────────┘ └────────┘ └─────────┘
     └───┬────┘
         │
    ┌────▼────┐
    │ Prompt  │
    │         │
    │System + │
    │ parser  │
    └─────────┘
```

## Key Design Decisions

### 1. Provider Abstraction

The `Provider` interface decouples the CLI from any specific LLM backend:

```go
type Provider interface {
    Translate(ctx context.Context, query string) (string, error)
    Name() string
    Available(ctx context.Context) error
}
```

V1 implements Ollama. The interface enables future backends (Apple Foundation Models, Claude API) without changing the core flow.

**Why this matters:** Adding a new LLM backend means implementing 3 methods. Nothing else changes.

### 2. Ollama via `Generate` (not `Chat`)

Each translation is an independent request — no conversation history needed. `Generate` is simpler: one system prompt, one user prompt, one response. `Chat` would add unnecessary complexity for stateless command translation.

### 3. Safety: Regex, Not LLM

Destructive command detection uses compiled regex patterns, not LLM classification.

**Why:** Safety checks must be deterministic, fast, and independent of the LLM. A regex match on `rm`, `sudo`, `dd` etc. is predictable and testable. Trusting the LLM to classify its own output would be circular.

- Safe commands: "Run this? [Y/n]" (default yes)
- Destructive commands: "Are you sure? [y/N]" (default no)

### 4. Prompt Engineering + Defensive Parsing

The system prompt instructs the LLM to output only the raw command. But LLMs don't always follow instructions, so the response parser defensively handles:
- Markdown code fences (` ```bash ... ``` `)
- Inline backticks
- "$ " prompt prefix
- Explanatory text after the command

**The parser is the reliability layer.** The prompt is a best-effort instruction; the parser handles reality. Both are tested.

### 5. Config: YAML, Not Viper

Three config fields don't need a framework. Raw `gopkg.in/yaml.v3` is simpler, has fewer dependencies, and teaches more Go.

### 6. Setup Flow

First-run setup handles the entire onboarding:
1. Detect if Ollama is installed → offer to install
2. Detect if Ollama is running → offer to start
3. Check for models → offer to pull recommended ones
4. User picks default model → save config

All actions require user consent. The tool never installs or modifies anything silently.

## Data Flow

```
User input          "compress this folder as tar.gz"
    │
    ▼
Args joining        strings.Join(os.Args[1:], " ")
    │
    ▼
Config load         ~/.shellbud/config.yaml → provider, model, host
    │
    ▼
Provider.Translate  System prompt + user prompt → Ollama API → raw response
    │
    ▼
ParseResponse       Strip fences/backticks/prefix → clean command
    │
    ▼
Safety.Classify     Regex patterns → Safe or Destructive
    │
    ▼
Confirm             Prompt user (default varies by safety level)
    │
    ▼
Executor.Run        $SHELL -c "command" (inherits stdin/stdout/stderr)
```

## Future Roadmap

| Feature | Description | Complexity |
|---------|-------------|------------|
| Apple Foundation Models | Swift helper binary, on-device, free, offline | Medium |
| Cloud models (Claude API) | New provider implementation | Low |
| Save & recall | Bookmark commands with labels, search later | Medium |
| Smart history | Tagged, searchable shell history | Medium |
| Shell completions | Tab completion for `sb` subcommands | Low (cobra built-in) |

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/ollama/ollama/api` | Ollama client |
| `gopkg.in/yaml.v3` | Config file parsing |
