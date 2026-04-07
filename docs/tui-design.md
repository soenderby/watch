# Watch TUI Design

## Purpose

Watch provides real-time awareness of all agent activity across projects. The TUI is the primary interface — it runs persistently in a "home" tmux session and gives the operator a global view of everything happening.

The TUI optimizes for a single operator who wants maximum information density, keyboard-driven navigation, and zero interruptions.

## Implementation Status (0.1.0-dev)

Implemented:
- Overview → Agent Detail → Instance Detail navigation stack
- Poll-driven live refresh (3s)
- Agent-scoped events in detail views
- `g` tmux jump and `l` run log pager

Deferred:
- Queue counters are rendered only when queue data is available (poller currently marks queue unavailable)

## Core Concepts

### Agent-Centric Model

The TUI displays agents, not tmux sessions. An agent is a persistent identity (defined in the identity registry) that may have multiple concurrent instances (tmux sessions). The agent is the thing the operator cares about. Sessions are infrastructure.

Tmux sessions not associated with a registered agent identity are invisible to watch. Watch monitors agents, not tmux.

See `docs/core-abstractions.md` for the full data model.

### Zoom-Based Hierarchical Navigation

The TUI is a navigation stack. The default view is an ultra-dense overview of all agents. The operator selects an item and drills in. Each level fills the screen with more detail about the selected item. `esc` always goes back.

This model was chosen over alternatives (split panes, tab views, scrolling lists) because:

1. **Density at the overview level.** The operator glances at one screen and knows the state of everything. No scrolling, no switching tabs.
2. **Detail on demand.** When something needs attention, the operator drills in. The detail view has room for full context without crowding the overview.
3. **Simple mental model.** One screen at a time. Deeper means more detail. `esc` means less detail. No spatial layout to remember.

## Navigation Levels

### Level 0: Overview

The default view. Shows all agents on one screen.

```
watch                                                  5 agents  10:45

> orca-worker (orca)                                          3 ready
    agent-1  running  orca-688  3m     agent-2  done  orca-9dy  1m

  orca-worker (myproject)                                     0 ready
    agent-1  running  mp-42  2m

  librarian (ai-resources)
    session  active  2m

  reviewer
    session  active  15m

j/k move  enter expand  g jump  q quit  ? help
```

**Content:**
- Each agent is a row with its name, project association (if any), and inline instance summary.
- Orca agents show their instances with slot names (agent-1, agent-2), state, issue, and duration. When 5 or more instances exist, the first 4 are shown followed by `+N more`. Instances ordered by most recently active.
- Non-orca agents show their instances with basic state and age.
- Queue state (ready count) is shown inline for orca project agents.

**Cursor behavior:**
- The cursor (`>`) moves between agents. It appears exactly once and is not used as a symbol elsewhere.
- `enter` on an agent opens Level 1 agent detail.
- `g` on an agent with exactly one instance jumps to that instance's tmux session. On an agent with multiple instances, `g` does nothing (drill in first to select a specific instance).

### Level 1: Agent Detail

Expanded view of one agent. Shows all instances and agent-scoped events.

For an orca agent:

```
watch / orca-worker (orca)                                   esc back

  /mnt/c/code/orca                          3 ready  1 in progress

> agent-1  running  orca-688  "Delete cockpit surface"           3m
  agent-2  done     orca-9dy  "Smoke test"                       1m

  events
    10:44  agent-2  done         orca-9dy   merged
    10:42  agent-1  started      orca-688
    10:40  agent-2  started      orca-9dy
    10:38  agent-1  instance up
    10:38  agent-2  instance up

j/k move  enter detail  g jump  esc back  ? help
```

For a non-orca agent:

```
watch / librarian (ai-resources)                             esc back

> session  active  2m  windows: 2

  events
    10:43  instance up

j/k move  g jump  esc back  ? help
```

**Design rationale — agent-scoped events:** Events are scoped to the agent being viewed. A busy agent's events do not drown out a quiet agent's events. Each agent's event history is independent.

**Cursor behavior:**
- Cursor moves between instances.
- `enter` on an orca instance opens Level 2 instance detail.
- `enter` on a non-orca instance does nothing (there is no deeper detail).
- `g` on any instance jumps to that instance's tmux session.

### Level 2: Instance Detail (Orca)

Full detail for one orca instance. Run history, events, and access to live output.

```
watch / orca-worker (orca) / agent-1                         esc back

  state:     running
  issue:     orca-688 "Delete cockpit surface"
  duration:  3m12s
  session:   orca-agent-1-20260320T091355Z

  runs
> #3  running  orca-688                  3m
  #2  done     orca-446  merged  4m47s  72k tokens
  #1  done     orca-445  merged  3m20s  58k tokens

  events
    10:42:15  run started   orca-688
    10:38:20  run done      orca-446  merged
    10:35:00  run done      orca-445  merged
    10:34:12  instance up

j/k move  g jump  l log  esc back  ? help
```

**Cursor behavior:**
- Cursor moves between runs.
- `g` jumps to the instance's tmux session. This is how the operator sees live agent output. Watch stays running.
- `l` opens the current run's `run.log` in a pager view within watch. `esc` or `q` exits the pager.

## Navigation Stack

Every `enter` pushes a view onto the stack. Every `esc` pops.

```
Level 0: Overview
  enter on agent          →  Level 1: Agent Detail

Level 1: Agent Detail
  enter on orca instance  →  Level 2: Instance Detail

Level 2: Instance Detail
  g                       →  jump to instance tmux session
  l                       →  view run.log in pager

esc at any level          →  go back one level
g on any jumpable item    →  switch tmux client to that session
```

## Keybindings

| Key | Context | Action |
|---|---|---|
| `j` / `↓` | All levels | Move cursor down |
| `k` / `↑` | All levels | Move cursor up |
| `enter` | All levels | Drill into selected item |
| `esc` | Levels 1+ | Go back one level |
| `g` | On instance | Jump to tmux session (watch stays running) |
| `l` | Level 2 | View current run log in pager |
| `r` | All levels | Force refresh |
| `q` | All levels | Quit watch |
| `?` | All levels | Toggle help overlay |

**Keybinding decisions:**
- vim-style `j`/`k` navigation. Arrow keys also work.
- `g` for jump rather than `enter` because `enter` means drill in. Jumping and drilling are different actions.
- Keybinding reference is always visible at the bottom. Shows only keys relevant to the current level.

## Visual Design Decisions

### No symbols or emoji for state

Session state is conveyed through text: `running`, `done`, `failed`, `blocked`, `active`, `idle`. Symbols add visual noise without proportional information value.

### Cursor is `>` and appears exactly once

The `>` character is reserved exclusively for the cursor position. It is not used as a decorator, list marker, or separator anywhere else in the UI.

### Monochrome

No color coding. The TUI must be fully usable without color. Color may be added later as an optional enhancement.

### Dense by default

The overview fits all agents on one screen for typical workloads (5-15 agents). Orca instances are packed two per line. No blank lines between items except between agents.

### Breadcrumb header

The header shows the navigation path (`watch / orca-worker (orca) / agent-1`) so the operator always knows where they are. `esc back` on the right.

## Data Model

The TUI renders from a Snapshot and queries an EventStore. It does not read tmux state or artifacts directly. See `docs/core-abstractions.md` for the full data model specification.

Key points for the TUI:
- A Snapshot contains only agents with active instances. No orphaned entries.
- Events are scoped per agent. Queried via `EventStore.ForAgent(name)`.
- Agents can have multiple concurrent instances. The TUI displays all of them.
- Unmatched tmux sessions (not associated with any agent) are invisible.

## Data Refresh

- Poll every 2-3 seconds. Each tick produces a new Snapshot and derives events from the diff.
- No filesystem watchers. Polling is simple and predictable.
- Current behavior: the app updates on each poll tick. Opportunistic redraw optimization is deferred.

## Implementation

### Technology

- **bubbletea** — Elm-architecture TUI framework for Go.
- **lipgloss** — Terminal styling (padding, alignment). Used sparingly given the monochrome constraint.
- **No generic list/table components from bubbles.** The navigation model is custom enough that hand-rolled views are simpler.

### Architecture

1. **Navigation stack** — a slice of view models. Each view renders itself and handles keys. `enter` pushes, `esc` pops.
2. **Data store** — holds the current Snapshot and EventStore. Updated on each poll tick.
3. **Poller adapter** — a `tea.Cmd` that invokes the poller and sends update messages.
4. **View models** — overview, agent detail, instance detail, log pager. Each receives the shared data store.

### Build Sequence

1. Level 0 overview with agents and inline instances. Static snapshot, cursor navigation, no events.
2. `enter`/`esc` navigation between Level 0 and Level 1.
3. `g` jump from any instance.
4. Polling and live refresh.
5. Event detection from snapshot diffs. Events shown at Level 1.
6. Level 2 instance detail with run history.
7. `l` log pager at Level 2.
