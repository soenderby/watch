# Watch TUI Design

## Purpose

Watch provides real-time awareness of all agent activity across projects. The TUI is the primary interface — it runs persistently in a "home" tmux session and gives the operator a global view of everything happening.

The TUI optimizes for a single operator who wants maximum information density, keyboard-driven navigation, and zero interruptions.

## Core Model: Zoom-Based Hierarchical Navigation

The TUI is a navigation stack. The default view is an ultra-dense overview of all sessions. The operator selects an item and drills in. Each level fills the screen with more detail about the selected item. `esc` always goes back.

This model was chosen over alternatives (split panes, tab views, scrolling lists) because:

1. **Density at the overview level.** The operator glances at one screen and knows the state of everything. No scrolling, no switching tabs.
2. **Detail on demand.** When something needs attention, the operator drills in. The detail view has room for full context without crowding the overview.
3. **Simple mental model.** One screen at a time. Deeper means more detail. `esc` means less detail. No spatial layout to remember.

## Navigation Levels

### Level 0: Overview

The default view. Shows everything on one screen.

```
watch                                              7 sessions  10:45

> orca                                                        3 ready
    agent-1  running  orca-688  3m     agent-2  done  orca-9dy  1m

  myproject                                                   0 ready
    agent-1  running  mp-42  2m

  project-a    active    2m
  project-b    active   15m
  scratch      idle      1h

j/k move  enter expand  g jump  q quit  ? help
```

**Content:**
- Orca projects are groups with inline agent previews. Agents are ordered by most recently active. When a project has 5 or more agents, the first 4 are shown followed by `+N more`.
- Non-orca sessions are individual top-level items showing name, state, and age.
- Queue state (ready count) is shown inline with each orca project.

**Cursor behavior:**
- The cursor moves between top-level items: orca project groups and standalone sessions.
- `enter` on a project group opens Level 1 project view.
- `enter` on a standalone session opens Level 1 session view.
- `g` on a standalone session jumps to that tmux session. `g` on a project group does nothing (drill in first to select a specific agent).

### Level 1: Orca Project

Expanded view of one orca project. Shows agents with full issue titles and project-scoped events.

```
watch / orca                                                 esc back

  /mnt/c/code/orca                          3 ready  1 in progress

> agent-1  running  orca-688  "Delete cockpit surface"           3m
  agent-2  done     orca-9dy  "Smoke test"                       1m

  events
    10:44  agent-2  done     orca-9dy   merged
    10:42  agent-1  started  orca-688
    10:40  agent-2  started  orca-9dy
    10:38  agent-1  session up
    10:38  agent-2  session up

j/k move  enter detail  g jump  esc back  ? help
```

**Design rationale — project-scoped events:** Events are scoped to the project being viewed. A global event stream would let busy projects drown out quiet ones, making it easy to miss a completion event for a project that only has one agent. Per-project event scoping ensures every project's activity is visible when the operator looks at it.

**Cursor behavior:**
- Cursor moves between agents.
- `enter` on an agent opens Level 2 agent detail.
- `g` on an agent jumps to that agent's tmux session.

### Level 1: Standalone Session

Minimal detail view for non-orca sessions.

```
watch / project-a                                            esc back

  windows:  2
  created:  10:43:00 (2m ago)

  g jump  esc back
```

There is not much to show for a non-orca session beyond what tmux provides. This level exists for navigation consistency — `enter` always drills in, even when the detail is sparse.

### Level 2: Agent Detail

Full detail for one orca agent. Run history, events, and access to live output.

```
watch / orca / agent-1                                       esc back

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
    10:34:12  session up

j/k move  g jump  l log  esc back  ? help
```

**Cursor behavior:**
- Cursor moves between runs.
- `g` jumps to the agent's tmux session. This is how the operator sees live agent output.
- `l` opens the current run's `run.log` in a pager view within watch. `esc` or `q` exits the pager and returns to the detail view.

## Navigation Stack

Every `enter` pushes a view onto the stack. Every `esc` pops. The stack determines what is on screen.

```
Level 0: Overview
  enter on project group  →  Level 1: Orca Project
  enter on session        →  Level 1: Standalone Session

Level 1: Orca Project
  enter on agent          →  Level 2: Agent Detail

Level 2: Agent Detail
  g                       →  jump to agent tmux session
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
| `g` | On session or agent | Jump to tmux session (watch stays running) |
| `l` | Level 2 agent detail | View current run log in pager |
| `r` | All levels | Force refresh |
| `q` | All levels | Quit watch |
| `?` | All levels | Toggle help overlay |

**Keybinding decisions:**
- vim-style `j`/`k` navigation, consistent with most terminal UIs. Arrow keys also work.
- `g` for jump rather than `enter` because `enter` means drill in. Jumping to a tmux session and drilling into a detail view are different actions and should be different keys.
- Keybinding reference is always visible at the bottom of the screen. It shows only the keys relevant to the current level. This removes the need to memorize bindings or consult external help.

## Visual Design Decisions

### No symbols or emoji for state

Session state is conveyed through text: `running`, `done`, `failed`, `blocked`, `active`, `idle`. Symbols and emoji were rejected because:

1. They add visual noise without proportional information value.
2. They are ambiguous across fonts and terminals.
3. When overused, they make the display harder to scan, not easier.

### Cursor is `>` and appears exactly once

The cursor must be instantly findable. Using `>` or any other symbol elsewhere in the UI (as decorators, list markers, or separators) would make the cursor ambiguous. The `>` character is reserved exclusively for the cursor position.

### Monochrome

No color coding. The TUI must be fully usable on any terminal without assuming color support or theme compatibility. Color may be added later as an optional enhancement if it provides clear scannability benefits, but the base design does not depend on it.

### Dense by default

The overview fits all sessions on one screen for typical workloads (5-15 sessions). Orca agents are packed two per line within project groups. Non-orca sessions are one per line. No blank lines between items except between groups.

### Breadcrumb header

The header shows the navigation path (`watch / orca / agent-1`) so the operator always knows where they are. `esc back` on the right reinforces how to return.

## Data Model

### Session Types

The TUI displays two kinds of items:

1. **Orca project groups.** Identified by registered project config. Each group contains zero or more orca agent sessions, identified by tmux session naming convention. Enriched with artifact data (run results, issue IDs, durations) read from the project's `agent-logs/` directory.

2. **Standalone sessions.** Any tmux session that is not identified as an orca agent session. Shown with basic tmux state only (name, window count, created time, activity).

### Events

Events are derived by diffing tmux and artifact state snapshots on each poll cycle. An event is generated when:

- A tmux session appears or disappears.
- An orca run summary appears or changes (new run started, run completed, result changed).

Events are:
- **In-memory only.** No persistence. Watch is live-only; historical review is a separate workflow.
- **Scoped per project** for orca sessions. Each project maintains its own event list.
- **Capped.** A fixed number of recent events are retained (e.g., last 50 per project). Older events are discarded.

### Refresh

- Poll every 2-3 seconds. Each tick re-reads the tmux session list and scans registered project artifact directories.
- No filesystem watchers or inotify. Polling is simple, predictable, and sufficient for the 2-3 second latency target.
- Refresh is opportunistic: if nothing changed, the display is not redrawn.

## Implementation

### Technology

- **bubbletea** — Elm-architecture TUI framework for Go. Provides the update loop, key handling, and rendering model.
- **lipgloss** — Terminal styling library (padding, alignment, borders). Used sparingly given the monochrome constraint.
- **No generic list/table components from bubbles.** The navigation model (zoom-based stack with mixed item types) is custom enough that hand-rolled views are simpler than adapting generic components.

### Architecture

The TUI application consists of:

1. **A navigation stack** — a slice of view models. Each view knows how to render itself and handle keys. `enter` pushes, `esc` pops.
2. **A data store** — holds the current snapshot of tmux sessions, orca project state, and event history. Updated on each poll tick.
3. **A poller** — a goroutine that periodically re-reads tmux and artifact state and sends update messages to the bubbletea program.
4. **View models** for each level — overview, project detail, session detail, agent detail, log pager. Each is a bubbletea model that receives the shared data store and renders its level.

### Build Sequence

1. Level 0 overview with project groups and standalone sessions. Static snapshot, cursor navigation, no events.
2. `enter`/`esc` navigation between Level 0 and Level 1.
3. `g` jump from any session or agent.
4. Polling and live refresh.
5. Event detection from snapshot diffs. Events shown at Level 1.
6. Level 2 agent detail with run history.
7. `l` log pager at Level 2.
