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

type Provider interface {
    Chat(ctx context.Context, messages []Message) (string, error)
    Name() string
    Available(ctx context.Context) error
}
```

`Message` is defined in the `provider` package (not imported from Ollama) so callers stay decoupled from the backend. The Ollama implementation converts `[]provider.Message` to `[]api.Message` internally.

**Why this matters:** Adding a new LLM backend means implementing 3 methods and one type conversion. Nothing else changes.

### 2. Ollama via Chat API

Conversations use Ollama's `Chat` endpoint (not `Generate`). One-shot mode is simply a single-turn chat. This gives a unified code path for both modes and enables conversation history in chat mode.

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

Destructive command detection uses 17 compiled regex patterns, not LLM classification.

**Why:** Safety checks must be deterministic, fast, and independent of the LLM. A regex match on `rm`, `sudo`, `dd` etc. is predictable and testable. Trusting the LLM to classify its own output would be circular.

- One-shot mode: safe commands show "Run this? [Y/n]" (default yes), destructive show "Are you sure? [y/N]" (default no)
- Chat mode: all commands show "[r]un / [e]xplain / [s]kip", destructive commands require an additional "Are you sure? [y/N]" confirmation after choosing run

### 5. Response Parsing

The LLM is instructed to use fenced code blocks (` ```bash ... ``` `) for commands. `ParseChatResponse()` extracts commands from code blocks via regex, returning both the full text (for display) and the extracted commands (for the action prompt).

Responses can be:
- **Text only** — explanation, answer to a question. Displayed as-is, no action prompt.
- **Text + commands** — explanation with one or more commands in code blocks. Text displayed, then each command gets run/explain/skip.
- **Commands only** — just code blocks. Treated same as above.

### 6. Output Capture (chat mode)

`RunCapture()` uses `io.MultiWriter` to simultaneously display output to the terminal and buffer it. The captured output (truncated at 8KB) is added to conversation history as a user message so the LLM can reference it in follow-up turns.

### 7. Config: YAML, Not Viper

Three config fields don't need a framework. Raw `gopkg.in/yaml.v3` is simpler and has fewer dependencies.

### 8. Setup Flow

First-run setup handles the entire onboarding:
1. Detect if Ollama is installed → offer to install
2. Detect if Ollama is running → offer to start
3. Check for models → offer to pull recommended ones
4. User picks default model → save config

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
Provider.Chat       Messages → Ollama Chat API → assistant response
    │
    ▼
ParseChatResponse   Extract commands from code blocks (if any)
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
Provider.Chat          → Ollama Chat API → response
    │
    ▼
Add to history         assistant message appended (capped at 50)
    │
    ▼
ParseChatResponse      Extract commands from code blocks
    │
    ├─ No commands → display text
    │
    └─ Commands found → for each:
        │
        ▼
    [r]un / [e]xplain / [s]kip
        │
        ├─ Run → RunCapture() → output displayed AND added to history
        ├─ Explain → immediate LLM call → explanation displayed
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
