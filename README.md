# Watch

Operator supervision tool for agent activity.

Watch builds an agent-centric view from tmux sessions, project config, agent identities, and orca artifacts. It can be used from both CLI and TUI.

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
# Register a project
watch project add myproject /path/to/repo

# Inspect current state
watch list
watch status

# TUI mode
watch
```

## Commands

| Command | Purpose |
|---|---|
| `watch` | Launch the interactive TUI |
| `list [--json]` | List agents with active instances |
| `status [--json]` | One-line summary of active agent/instance counts |
| `project add <name> <path>` | Register a project |
| `project remove <name>` | Unregister a project |
| `project list [--json]` | List registered projects |
| `version` | Print version |

## Configuration

### Project registry

Project config lives at `~/.config/watch/config.json` and is managed via `watch project ...` commands.

### Agent identity registry

Watch only shows sessions that can be matched to a registered agent identity.

Identity definitions are loaded from:

1. `~/.config/watch/agents.json` (global)
2. `<project-root>/agents.json` (project-local, merged)

When names collide, global entries win.

Minimal example:

```json
{
  "agents": [
    {"name": "orca-worker", "project": "orca", "description": "Batch worker"},
    {"name": "librarian", "project": "ai-resources", "description": "Knowledge agent"}
  ]
}
```

## How It Works

Data flow:

1. Read config + identity registry
2. Read tmux sessions
3. Read project artifacts (`agent-logs/sessions/...`)
4. Build a snapshot of active agents/instances
5. Diff snapshots to derive events
6. Render CLI/TUI from snapshot + event store

Matching behavior:

- **Orca sessions**: matched by orca naming convention + artifact session ID.
- **Non-orca sessions**: matched by tmux working directory under a registered project path.
- **Unmatched sessions**: ignored (not shown).

## Status

Active development (`0.1.0-dev`).

Implemented:
- agent-centric snapshot model
- snapshot diff + per-agent event store
- CLI: `list`, `status`, `project ...`
- TUI navigation (overview → agent detail → instance detail)
- tmux jump (`g`) and run log pager (`l`)

Deferred / not yet implemented:
- queue state via `br` integration (currently unavailable)
- automatic matching for global (project-less) agents
- richer matching when multiple distinct non-orca identities exist in one project
- help overlay (`?`) in TUI
