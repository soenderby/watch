# Watch

Operator supervision tool for agent activity. Monitors tmux sessions across projects, enriches orca batch sessions with artifact data, and provides interactive navigation.

Part of the orca ecosystem. See [orca/docs/ecosystem.md](https://github.com/soenderby/orca/blob/main/docs/ecosystem.md) for the broader vision.

## Install

```bash
go install github.com/soenderby/watch/cmd/watch@latest
```

Or build from source:

```bash
go build -o watch ./cmd/watch/
```

## Usage

```bash
# Register a project for orca artifact enrichment
watch project add myproject /path/to/repo

# List all tmux sessions
watch list
watch list --json

# Quick status summary
watch status

# TUI mode (not yet implemented)
watch
```

## Commands

| Command | Purpose |
|---|---|
| `list [--json]` | List all tmux sessions with state and orca enrichment |
| `status [--json]` | One-line summary of session counts |
| `project add <name> <path>` | Register a project |
| `project remove <name>` | Unregister a project |
| `project list [--json]` | List registered projects |
| `version` | Print version |

## How It Works

Watch monitors all tmux sessions on the machine. It identifies orca sessions by naming convention (`orca-agent-*`) and enriches them with data from the project's `agent-logs/` directory (run results, issue IDs).

Non-orca sessions (interactive conversations, ad-hoc tasks) are shown with basic tmux state.

## Configuration

Config lives at `~/.config/watch/config.json`. Managed via `watch project` commands.

## Status

Early development. CLI mode works. TUI mode, session jump, and notification buffer are planned.
