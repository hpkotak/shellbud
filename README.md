# ShellBud (`sb`)

Translate natural language to shell commands using a local LLM.

```
$ sb compress this folder as tar.gz
  tar -czvf archive.tar.gz ./folder

  Run this? [Y/n]:
```

## Features

- Natural language to shell command translation via Ollama
- Safety detection for destructive commands (`rm`, `sudo`, `dd`, etc.)
- Interactive setup — handles Ollama installation and model configuration
- OS-aware — adapts commands for macOS and Linux
- Offline — runs entirely on your machine, no cloud API needed

## Install

Requires [Go 1.22+](https://go.dev/dl/) and [Ollama](https://ollama.com).

```bash
go install github.com/hpkotak/shellbud@latest
```

## Quick Start

```bash
# First-time setup (installs Ollama if needed, pulls a model)
sb setup

# Translate natural language to commands
sb find all log files larger than 100MB
sb show disk usage sorted by size
sb list running docker containers

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
