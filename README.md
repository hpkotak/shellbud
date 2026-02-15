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
- **Context-aware**: knows your cwd, git branch, directory contents, OS, and shell
- **Conversational**: chat mode remembers what you asked and what commands produced
- **Safe**: destructive commands (`rm`, `sudo`, `dd`) require double confirmation
- **Offline**: runs entirely on your machine via Ollama, no cloud API needed
- **Run / Explain / Skip**: review commands before executing, ask for explanations

## Install

Requires [Go 1.22+](https://go.dev/dl/) and [Ollama](https://ollama.com).

```bash
go install github.com/hpkotak/shellbud@latest
```

## Quick Start

```bash
# First-time setup (installs Ollama if needed, pulls a model)
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
sb config set model codellama:7b        # Change default model
sb config set ollama.host http://host:11434  # Custom Ollama host
```

## Architecture

See [docs/design.md](docs/design.md) for architecture decisions and design rationale.

## License

MIT
