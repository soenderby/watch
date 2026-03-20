# Watch Core Abstractions

This document defines the data model and core logic layers that the TUI and CLI render from. These abstractions are designed, implemented, and tested independently before the TUI is built.

See `docs/tui-design.md` for the view layer that consumes these abstractions.

## Design Principles

1. **Data flows in one direction.** Raw sources → builder → snapshot → diff → events → store → views. No component reaches back into a previous layer.
2. **The TUI is a thin view.** It sees Snapshot and EventStore. It never calls tmux or reads artifacts directly.
3. **Extensibility through optional fields.** New session types, enrichment sources, and event types are added as new fields or string values. Existing code does not break.
4. **Single source of truth.** Sessions exist once in the snapshot. Projects reference them, not copy them.
5. **Testable without a terminal.** Every abstraction can be tested with unit tests and CLI tools. No bubbletea dependency outside the TUI package.

## Package Structure

```
internal/
  model/          Snapshot, Session, Project, Run, Event types (no dependencies)
  snapshot/       SnapshotBuilder — assembles Snapshot from raw sources
  events/         Diff function + EventStore
  poller/         Poller — periodic snapshot + event production
  tmux/           tmux session discovery and interaction (exists)
  orca/           orca artifact reading (exists)
  config/         project registry (exists)
```

`model` has no dependencies on any other package. Every other package depends on `model`. The dependency graph is acyclic:

```
model
  ↑
  ├── snapshot (depends on: model, tmux, orca, config)
  ├── events   (depends on: model)
  ├── poller   (depends on: model, snapshot, events, config)
  └── TUI/CLI  (depends on: model, poller, events)
```

---

## 1. Data Model (`internal/model`)

### Snapshot

The top-level point-in-time state of everything.

```
Snapshot
  Timestamp      time.Time       when this snapshot was taken
  Sessions       []*Session      all sessions, each appears exactly once
  Projects       []*Project      orca projects, each references sessions
```

Methods:
- `StandaloneSessions() []*Session` — sessions not claimed by any project (Type != "orca")
- `ProjectByName(name string) *Project` — find a project by name, nil if not found
- `SessionByName(name string) *Session` — find a session by tmux name, nil if not found

### Session

One tmux session, enriched with type-specific data.

```
Session
  Name           string          tmux session name (unique within a snapshot)
  Type           string          "orca" or "standalone" (extensible)
  Tmux           TmuxState       raw tmux data, always present
  Orca           *OrcaAgent      orca enrichment, nil for non-orca sessions
```

Type is a string, not a Go enum. New session types (e.g., "lore", "ci") can be added without modifying existing code. Type-specific enrichment is an optional pointer field — nil means not applicable. New enrichment types are added as new pointer fields on Session. Existing code that does not reference the new fields is unaffected.

### TmuxState

Raw tmux data for a session. Not interpreted — the view layer decides what "idle" or "active" means.

```
TmuxState
  Windows        int             window count
  Created        time.Time       session creation time
  Attached       bool            whether a client is attached
  Activity       time.Time       last activity timestamp
```

### OrcaAgent

Orca-specific enrichment for an agent session.

```
OrcaAgent
  ProjectName    string          registered project this belongs to
  AgentName      string          e.g., "agent-1"
  SessionID      string          orca session ID from artifact directory
  State          string          derived state (see below)
  CurrentRun     *Run            latest/active run, nil if no runs found
  Runs           []Run           all runs, newest first
```

**State derivation.** State is computed by the snapshot builder, not stored in any artifact. The rules:

| tmux alive | latest run has summary | summary result | → State |
|---|---|---|---|
| yes | no | — | "running" |
| yes | yes, result != failed/blocked | — | "running" (between runs) |
| yes | yes, result = failed | — | "failed" |
| yes | yes, result = blocked | — | "blocked" |
| no | yes, result = completed | — | "done" |
| no | yes, result = failed | — | "failed" |
| no | yes, result = blocked | — | "blocked" |
| no | no | — | "idle" |

### Run

One agent run within a session.

```
Run
  RunID          string          e.g., "0001-20260320T091356748983159Z"
  Result         string          "completed" | "blocked" | "no_work" | "failed" | ""
  IssueID        string          from summary.json, empty if not claimed
  Merged         bool            from summary.json
  Duration       time.Duration   from metrics or computed from timestamps
  Tokens         int             token count, 0 if unavailable
  HasSummary     bool            whether summary.json existed and was parseable
  Notes          string          from summary.json notes field
  LogPath        string          absolute path to run.log
  SummaryPath    string          absolute path to summary.json
```

Issue titles are not included. The summary.json contains issue IDs only. Title resolution (via `br show`) is a future enhancement. The field can be added to Run without structural changes when needed.

### Project

A registered orca project with references to its agent sessions.

```
Project
  Name           string          from config
  Path           string          filesystem path to the repo
  Queue          QueueState      queue summary
  Agents         []*Session      references to sessions in the snapshot
```

Agents is a slice of pointers into the snapshot's Sessions slice. Not copies. Ordered by most recently active (most recent first).

### QueueState

Summary of a project's br queue.

```
QueueState
  Ready          int             issues ready to be worked
  InProgress     int             issues currently in progress
  Available      bool            whether queue state could be read
```

The Available field distinguishes "zero ready issues" from "could not read the queue." When br is not installed or the project has no .beads directory, Available is false and the counts are zero.

---

## 2. Snapshot Builder (`internal/snapshot`)

Assembles a Snapshot from raw data sources.

### Inputs

1. `tmux.ListSessions()` — raw tmux session list
2. `config.Config` — registered projects with paths
3. `orca.FindSessions()` — artifact data per project path
4. Queue state per project (via `br` commands)

### Interface

```
Builder
  Build() → (*model.Snapshot, error)
```

The builder reads all inputs on each call. It does not cache. Caching is the poller's concern.

### Assembly Logic

```
1. Read tmux sessions. Create a *Session for each, Type = "standalone".

2. For each registered project:
   a. Scan its agent-logs directory for session artifact data.
   b. For each tmux session, check if its name matches the orca naming convention
      AND if the project has artifact data for a matching session ID.
   c. For matching sessions:
      - Set Type = "orca"
      - Populate Orca field (agent name, session ID, runs, current run)
      - Derive State from tmux liveness + latest run summary
   d. Create the Project with references to matched sessions.
   e. Read queue state (ready/in_progress counts).
   f. Order agents by most recent activity.

3. Sessions not claimed by any project remain Type = "standalone".

4. Return Snapshot with current timestamp.
```

### Matching Logic

Orca tmux sessions follow the naming convention `<prefix>-<index>-<timestamp>` (e.g., `orca-agent-1-20260320T091355Z`). Artifact session directories are named `<agent-name>-<timestamp>` (e.g., `agent-1-20260320T091355Z`).

Matching: a tmux session name matches a project's artifact session when the tmux name (after stripping the prefix) ends with the artifact session ID, or the artifact session ID is contained within the tmux name.

A tmux session can match at most one project. If multiple projects could claim the same session (unlikely but possible with overlapping prefixes), the first match wins. This is a known simplification.

### Error Handling

The builder is tolerant. If tmux is unavailable, it returns an empty session list. If a project's artifact directory is unreadable, that project gets an empty agent list. If queue state cannot be read, QueueState.Available is false. The builder never fails entirely — it returns the best snapshot it can assemble.

### Tests

- Basic: tmux sessions + one project with artifacts → correct snapshot
- Matching: tmux session name correctly matched to artifact session ID
- No match: tmux session with orca-like name but no matching artifacts → standalone
- Missing artifacts: project registered but no agent-logs directory → project with empty agents
- State derivation: all combinations from the state table
- Multiple projects: sessions correctly assigned to their projects
- No tmux: builder returns snapshot with empty sessions
- Queue unavailable: QueueState.Available = false, counts = 0
- Agent ordering: most recently active agent first in project

---

## 3. Snapshot Diffing and Event Derivation (`internal/events`)

Pure function that compares two snapshots and produces events.

### Event

```
Event
  Timestamp      time.Time       when the event was detected
  Type           string          event type (see below)
  SessionName    string          tmux session name
  ProjectName    string          empty for standalone sessions
  AgentName      string          empty for standalone sessions
  RunID          string          empty for session-level events
  IssueID        string          empty when not applicable
  Result         string          empty for non-completion events
  Merged         bool
```

Event Type is a string for extensibility. Initial values:

| Type | Meaning |
|---|---|
| `session_up` | tmux session appeared |
| `session_down` | tmux session disappeared |
| `run_started` | new run directory detected for an orca agent |
| `run_completed` | run gained a summary (or summary changed) |

### Diff Function

```
Diff(prev, curr *model.Snapshot) → []Event
```

Logic:

```
1. Index prev sessions by name.
2. Index curr sessions by name.

3. For each session in curr not in prev:
   → emit session_up event

4. For each session in prev not in curr:
   → emit session_down event

5. For each orca session present in both:
   a. Compare run counts. If curr has more runs than prev:
      → emit run_started for each new run
   b. For each run present in both, if curr has a summary that prev did not:
      → emit run_completed with result, issue, merged from summary

6. All emitted events use curr.Timestamp as their timestamp.

7. Return events sorted by type priority: session_up, run_started, run_completed, session_down.
```

### First Snapshot

When prev is nil (first poll), the diff function treats it as an empty snapshot. All current sessions generate `session_up` events. This bootstraps the event history so that the operator sees the initial state as events.

### Tests

- Session appears → session_up
- Session disappears → session_down
- New orca run (run directory exists, no summary yet) → run_started
- Run gains summary → run_completed with correct fields
- No change → empty event list
- Multiple events in one diff (two agents finish simultaneously)
- First snapshot (nil prev) → session_up for all sessions
- Standalone session appears → event with empty project/agent fields
- Orca session disappears → session_down with project/agent fields populated

---

## 4. Event Store (`internal/events`)

Accumulates events over time. Queryable by project. Capped to prevent unbounded memory growth.

### Interface

```
EventStore
  cap            int             max events per scope

  Add(events []Event)
    Appends each event to its project scope (keyed by ProjectName,
    empty string for standalone). Trims oldest events when cap is exceeded.

  ForProject(name string) []Event
    Returns events for the named project, newest first.

  Standalone() []Event
    Returns events for standalone sessions (ProjectName = ""), newest first.

  All() []Event
    Returns all events across all scopes, merged and sorted newest first.

  Clear()
    Removes all events. Used when config changes invalidate event history.
```

### Scoping

Events are stored in a map keyed by project name. Standalone session events use the empty string as their key. This ensures that:
- Events from different projects never appear mixed when querying by project.
- A busy project's events do not push out a quiet project's events.
- Each project independently respects the cap.

### Tests

- Add events, query by project — correct scoping
- Cap enforcement — oldest events dropped when cap exceeded
- Events from different projects do not interfere
- Standalone events stored under empty string key
- All() returns merged and sorted results
- ForProject on unknown name returns empty slice
- Clear removes everything

---

## 5. Poller (`internal/poller`)

Periodically produces snapshots and derives events. This is the engine that drives the TUI's data.

### Interface

```
Poller
  interval       time.Duration   poll interval
  configPath     string          path to watch config file
  store          *events.EventStore
  builder        snapshot.Builder
  prev           *model.Snapshot  previous snapshot for diffing

  Poll() → (snapshot *model.Snapshot, newEvents []Event, err error)
    1. Reload config (picks up newly added/removed projects)
    2. Call builder.Build() to assemble current snapshot
    3. If prev is not nil, call events.Diff(prev, curr) to derive events
    4. Add new events to store
    5. Set prev = curr
    6. Return snapshot and new events
```

### bubbletea Integration

When used in the TUI, the poller is invoked via a `tea.Cmd` that sleeps for the poll interval, then calls `Poll()` and returns the result as a `tea.Msg`. The TUI model receives this message and updates its view state.

The poller itself has no dependency on bubbletea. The integration is a thin adapter in the TUI package.

### Standalone Use

The poller can be used without the TUI:

```
for {
    snap, events, err := poller.Poll()
    // print snapshot summary, log events, etc.
    time.Sleep(interval)
}
```

This is useful for testing and for the CLI commands (`watch list`, `watch status`) which do a single poll without entering the TUI.

### Error Handling

The poller does not fail on transient errors. If tmux is unavailable or a project's artifacts are unreadable, the builder returns a partial snapshot and the poller works with what it has. Errors are logged, not propagated as fatal.

### Tests

- First poll: returns snapshot, no events (or session_up events for existing sessions)
- Second poll with no changes: returns snapshot, empty events
- Second poll with changes: returns snapshot with correct events
- Config change between polls: new project picked up, removed project no longer scanned
- tmux unavailable: partial snapshot with empty sessions, no crash
- Artifact directory unreadable: partial snapshot, project has empty agents

---

## Composition

```
config.Load() ─────────────┐
                            │
tmux.ListSessions() ────────┼──→ snapshot.Builder.Build() ──→ model.Snapshot
                            │                                      │
orca.FindSessions() ────────┘                                      │
                                                                   │
                                       ┌───────────────────────────┘
                                       │
                                 events.Diff(prev, curr) ──→ []model.Event
                                                                   │
                                                                   ▼
                                                          events.EventStore.Add()
                                                                   │
                                                                   ▼
                                                    EventStore.ForProject() / .All()
                                                                   │
                                                                   ▼
                                                              TUI views
```

The TUI receives a Snapshot and queries the EventStore. It does not know how snapshots are built, how events are derived, or how tmux is queried. If any upstream component changes (new data source, different artifact format, alternative to tmux), the TUI does not change.

---

## Deferred Decisions

### Issue Titles

The TUI design shows issue titles at Level 1 and Level 2. The summary.json contains issue IDs only. Resolving titles would require `br show <id> --json` per issue per poll. This is deferred. The model can accommodate a title field on Run when it is added. For now, the TUI shows IDs only.

### Queue State

QueueState requires shelling out to `br` (two commands per project per poll). This is included from the start because the user typically has 1-2 projects and the latency is acceptable. If polling performance becomes a problem, queue reads can be made less frequent than the main poll interval (e.g., every 5th tick).

### Metrics Integration

The current design reads run data from summary.json files. The metrics.jsonl file contains additional data (tokens, durations, harness version) that could enrich the snapshot. This is not included in the initial build. If needed, the snapshot builder can be extended to read metrics.jsonl without changing the model — just populate additional fields on Run.

### Session Persistence

Events and snapshots are in-memory only. If watch restarts, event history is lost. This is intentional — watch is live-only. Historical review is a separate workflow. If persistence is ever needed, the EventStore interface supports it without changing consumers.
