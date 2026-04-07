# Watch Core Abstractions

This document defines the data model and core logic layers that the TUI and CLI render from. These abstractions are designed, implemented, and tested independently before the TUI is built.

See `docs/tui-design.md` for the view layer that consumes these abstractions.

## Design Principles

1. **Agent-centric, not session-centric.** The fundamental unit is the agent — a persistent identity that spans sessions. Tmux sessions are instances of agents, not first-class entities.
2. **Data flows in one direction.** Raw sources → builder → snapshot → diff → events → store → views. No component reaches back into a previous layer.
3. **The TUI is a thin view.** It sees Snapshot and EventStore. It never calls tmux or reads artifacts directly.
4. **Extensibility through optional fields.** New agent types, enrichment sources, and event types are added as new fields or string values. Existing code does not break.
5. **Single source of truth.** Agents exist once in the snapshot. Instances reference them, not copy them.
6. **Testable without a terminal.** Every abstraction can be tested with unit tests and CLI tools. No bubbletea dependency outside the TUI package.
7. **Only agents are visible.** Tmux sessions not associated with a registered agent identity are ignored entirely. Watch monitors agents, not tmux.

## Package Structure

```
internal/
  identity/       Agent identity registry (planned for extraction to lore)
  model/           Snapshot, Agent, Instance, Run, Event types (no dependencies)
  snapshot/        SnapshotBuilder — assembles Snapshot from raw sources
  events/          Diff function + EventStore
  poller/          Poller — periodic snapshot + event production
  tmux/            tmux session discovery and interaction (exists)
  orca/            orca artifact reading (exists)
  config/          project registry (exists)
```

`model` has no dependencies on any other package. `identity` has no dependencies on any other internal package. Every other package depends on `model`. The dependency graph is acyclic:

```
model       identity
  ↑            ↑
  ├── snapshot (depends on: model, identity, tmux, orca, config)
  ├── events   (depends on: model)
  ├── poller   (depends on: model, identity, snapshot, events, config)
  └── TUI/CLI  (depends on: model, identity, poller, events)
```

---

## 1. Agent Identity (`internal/identity`)

This package defines agent identities and the registry that stores them. It is designed for future extraction into lore when that tool is built. Runtime snapshot/poller paths consume identities read-only; CLI flows (e.g., identity adoption) can append new definitions to registry files.

### AgentIdentity

An agent identity is a persistent definition. It describes who an agent is, not what it is currently doing.

```
AgentIdentity
  Name           string       unique identifier (e.g., "orca-worker", "librarian", "reviewer")
  Project        string       optional project association, empty for global agents
  PrimingRef     string       optional path/reference to priming context (AGENTS.md, prompt file)
  Description    string       short human-readable description
  Match          MatchRules?  optional explicit session matching rules

MatchRules
  SessionPattern string       optional glob pattern on tmux session name
  PathPrefix     string       optional path prefix on tmux working directory
```

Name is the primary key. It must be unique across the registry (global + project-local combined).

Project is optional. When set, it associates the agent with a registered project. When empty, the agent is global — it can be used in any context.

PrimingRef is a pointer to the agent's priming context, not the context itself. Watch does not interpret priming — it just knows the reference exists. Lore will eventually own priming interpretation.

Match is optional. When present, all non-empty match fields must match the tmux session for the identity to be selected. Explicit matches are preferred. Identities without match rules are treated as fallback candidates.

### Registry

The registry is the authoritative list of known agent identities. It merges two sources:

1. **Global registry** — `~/.config/watch/agents.json`. Operator-managed. Contains all agent definitions, both global and project-associated.

2. **Project-local definitions** — an optional `agents.json` file in a registered project's repo root. Contains agents specific to that project. Merged with the global registry on load.

When both sources define an agent with the same name, the global registry takes precedence (the operator's definition wins over the project default).

```
Registry
  agents         []AgentIdentity

  All() []AgentIdentity
  ByName(name string) *AgentIdentity
  ForProject(name string) []AgentIdentity
  Global() []AgentIdentity               agents with no project association
```

### Registry File Format

Global registry (`~/.config/watch/agents.json`):

```json
{
  "agents": [
    {
      "name": "orca-worker",
      "project": "orca",
      "description": "Batch execution worker",
      "match": {"session_pattern": "orca-agent-1-*"}
    },
    {
      "name": "librarian",
      "project": "ai-resources",
      "priming_ref": "AGENTS.md",
      "description": "Knowledge management agent",
      "match": {"path_prefix": "/mnt/c/code/ai-resources/worktrees/librarian"}
    },
    {
      "name": "reviewer",
      "description": "Careful code reviewer, any project",
      "match": {"session_pattern": "review-*"}
    }
  ]
}
```

Project-local (`<project-root>/agents.json`):

```json
{
  "agents": [
    {
      "name": "orca-worker",
      "description": "Batch execution worker for this project",
      "match": {"path_prefix": "worktrees/agent-1"}
    }
  ]
}
```

Project-local agents automatically have their project field set to the project they were discovered in. Relative `match.path_prefix` values in project-local files are resolved against the project root. Agent names must still be globally unique.

### Extraction Plan

This package is designed for future extraction into lore. When lore is built:

1. Lore absorbs the identity package, extending it with memory, learning, and context management.
2. Watch continues to read agent identities, now from lore's registry instead of its own.
3. The AgentIdentity type may grow new fields (lore-managed), but existing fields remain stable.
4. Watch's dependency on the identity interface does not change — only the backing implementation moves.

---

## 2. Data Model (`internal/model`)

### Snapshot

The top-level point-in-time state of everything watch knows about.

```
Snapshot
  Timestamp      time.Time
  Agents         []*Agent         all agents with at least one active instance
  Projects       []*Project       orca projects from registered config
```

A snapshot only contains agents that have active instances (live tmux sessions). Registered agents with no active instances do not appear. The snapshot is operational state, not a registry dump.

Methods:
- `AgentByName(name string) *Agent` — find an agent by identity name
- `AgentsForProject(project string) []*Agent` — agents associated with a project

### Agent

An agent as observed at snapshot time. Combines identity (who it is) with operational state (what it is doing).

```
Agent
  Identity       AgentIdentity    from the registry
  Instances      []*Instance      currently active tmux sessions, ordered by most recently active
  State          string           derived aggregate state (see below)
```

State is derived from the agent's instances:

| Instances | Condition | → Agent State |
|---|---|---|
| all idle | — | "idle" |
| any running | — | "running" |
| all done, none running | — | "done" |
| any failed, none running | — | "failed" |
| any blocked, none running | — | "blocked" |
| none | — | (agent not in snapshot) |

### Instance

One tmux session associated with an agent. An instance is the operational unit — it has a tmux session, and optionally orca enrichment.

```
Instance
  SessionName    string           tmux session name (unique within a snapshot)
  Tmux           TmuxState        raw tmux data
  State          string           instance-level state (see below)
  Orca           *OrcaRunState    orca enrichment, nil for non-orca instances
```

### TmuxState

Raw tmux data for a session. Not interpreted by the model.

```
TmuxState
  Windows        int
  Created        time.Time
  Attached       bool
  Activity       time.Time        last activity timestamp
```

### Instance State

Instance state is derived differently for orca and non-orca instances.

For orca instances (Orca field is non-nil):

| tmux alive | latest run has summary | summary result | → State |
|---|---|---|---|
| yes | no | — | "running" |
| yes | yes, result != failed/blocked | — | "running" |
| yes | yes, result = failed | — | "failed" |
| yes | yes, result = blocked | — | "blocked" |
| no | yes, result = completed | — | "done" |
| no | yes, result = failed | — | "failed" |
| no | yes, result = blocked | — | "blocked" |
| no | no | — | "idle" |

For non-orca instances:

| tmux alive | recent activity | → State |
|---|---|---|
| yes | within threshold | "active" |
| yes | beyond threshold | "idle" |
| no | — | "done" |

### OrcaRunState

Orca-specific enrichment for an instance.

```
OrcaRunState
  AgentName      string           e.g., "agent-1" (orca slot name)
  SessionID      string           orca session ID from artifact directory
  CurrentRun     *Run             latest/active run, nil if no runs found
  Runs           []Run            all runs, newest first
```

### Run

One agent run within an orca session.

```
Run
  RunID          string           e.g., "0001-20260320T091356748983159Z"
  Result         string           "completed" | "blocked" | "no_work" | "failed" | ""
  IssueID        string           from summary.json, empty if not claimed
  Merged         bool
  Duration       time.Duration
  Tokens         int              0 if unavailable
  HasSummary     bool
  Notes          string           from summary.json
  LogPath        string           absolute path to run.log
  SummaryPath    string           absolute path to summary.json
```

### Project

A registered project. Holds queue state. Does not own agents — agents reference projects through their identity.

```
Project
  Name           string           from config
  Path           string           filesystem path
  Queue          QueueState
```

### QueueState

```
QueueState
  Ready          int
  InProgress     int
  Available      bool             false when queue state could not be read
```

### Extensibility

- **New agent types:** Enrichment for new agent types (e.g., lore agents) is added as new optional pointer fields on Instance, alongside Orca. Existing code that does not reference the new field is unaffected.
- **New identity fields:** AgentIdentity gains new fields when lore extends it. Watch ignores fields it does not use.
- **New event types:** Event.Type is a string. New values are added without changing existing code.
- **Richer agent state:** If new states emerge (e.g., "waiting", "queued"), they are added to the state derivation logic. The string type does not constrain the values.

---

## 3. Snapshot Builder (`internal/snapshot`)

Assembles a Snapshot from raw data sources.

### Inputs

1. `identity.Registry` — registered agent identities
2. `tmux.ListSessions()` — raw tmux session list
3. `config.Config` — registered projects with paths
4. `orca.FindSessions()` — artifact data per project path
5. Queue state per project (via `br` commands)

### Interface

```
Builder
  Build() → (*model.Snapshot, error)
```

The builder reads all inputs on each call. It does not cache.

### Assembly Logic

```
1. Load agent identity registry (global + project-local).
2. Read all tmux sessions.
3. For each registered project:
   a. Scan its agent-logs directory for session artifact data.
   b. Read queue state. Create Project in snapshot.

4. Match tmux sessions to agent identities:
   a. Orca matching: for each tmux session matching the orca naming convention
      (<prefix>-<index>-<timestamp>), locate matching artifact data, then
      select the project identity via explicit rules first and fallback
      identity only when unambiguous.
   b. Non-orca matching: for each tmux session whose working directory
      matches a registered project path, apply project explicit rules,
      then global explicit rules, then project fallback when unambiguous.
      Create an Instance without orca enrichment.
   c. Unmatched or ambiguous sessions are ignored. They do not appear in
      the snapshot.

5. For each agent identity that has at least one matched instance:
   a. Create Agent with identity + instances.
   b. Derive agent state from instance states.
   c. Order instances by most recently active.

6. Agents with no matched instances are not included in the snapshot.

7. Return Snapshot with current timestamp.
```

### Matching Details

**Orca matching.** Orca tmux sessions follow the naming convention `<prefix>-<index>-<timestamp>`. The prefix is typically `orca-agent`. The builder checks each registered project for orca artifacts. When a tmux session name matches the convention and the project has artifact data for a matching session ID, the session becomes an orca instance of that project's agent.

**Non-orca matching.** tmux provides a session's working directory. If a session's working directory is within a registered project's path, the builder considers that project's identities. Explicit identity rules (`match.session_pattern`, `match.path_prefix`) are applied first. If no explicit rule matches and exactly one project identity remains, that fallback identity is used. Ambiguous matches are ignored.

**Global agents.** Agents with no project association can match sessions through explicit rules (`match.session_pattern`, `match.path_prefix`). Global identities without explicit rules are not auto-matched.

### Error Handling

The builder is tolerant. If tmux is unavailable, it returns an empty snapshot. If a project's artifacts are unreadable, that project's agents get no orca enrichment. If queue state cannot be read, QueueState.Available is false. The builder never fails entirely.

### Tests

- Orca matching: tmux session correctly matched to project agent via naming convention
- Non-orca matching: tmux session matched to project agent via working directory
- Unmatched sessions: tmux sessions not matching any agent are excluded
- No instances: registered agent with no active tmux sessions is excluded from snapshot
- Multiple instances: agent with multiple tmux sessions gets all of them
- State derivation: all combinations from the state tables
- Missing artifacts: project registered but no agent-logs → agent with non-orca instances only
- Queue unavailable: QueueState.Available = false
- Instance ordering: most recently active first
- Multiple projects: sessions correctly assigned to their projects' agents
- Explicit session/path rules: correct identity selected when multiple identities exist
- Ambiguous identities: session is ignored rather than mis-attributed
- Global explicit matching: project-less identities can match by rules

---

## 4. Snapshot Diffing and Event Derivation (`internal/events`)

Pure function that compares two snapshots and produces events.

### Event

```
Event
  Timestamp      time.Time
  Type           string           event type (see below)
  AgentName      string           agent identity name
  ProjectName    string           agent's project, empty for global agents
  SessionName    string           tmux session name (instance identifier)
  RunID          string           empty for non-run events
  IssueID        string           empty when not applicable
  Result         string           empty for non-completion events
  Merged         bool
```

Events are always associated with an agent. Since unmatched sessions are invisible, every event has an AgentName.

Event types:

| Type | Meaning |
|---|---|
| `instance_up` | new instance appeared for an agent |
| `instance_down` | instance disappeared for an agent |
| `run_started` | new orca run detected |
| `run_completed` | orca run gained a summary |

### Diff Function

```
Diff(prev, curr *model.Snapshot) → []Event
```

Logic:

```
1. Index prev instances by session name.
2. Index curr instances by session name.

3. For each instance in curr not in prev:
   → emit instance_up

4. For each instance in prev not in curr:
   → emit instance_down

5. For each orca instance present in both:
   a. If curr has more runs than prev → emit run_started for each new run
   b. If a run gained a summary → emit run_completed with result, issue, merged

6. All events use curr.Timestamp.

7. Return events sorted by type priority: instance_up, run_started, run_completed, instance_down.
```

### First Snapshot

When prev is nil, all current instances generate `instance_up` events.

### Tests

- Instance appears → instance_up
- Instance disappears → instance_down
- New orca run → run_started
- Run gains summary → run_completed
- No change → empty events
- Multiple events in one diff
- First snapshot (nil prev) → instance_up for all
- Events carry correct agent and project names

---

## 5. Event Store (`internal/events`)

Accumulates events over time. Queryable by agent. Capped per agent.

### Interface

```
EventStore
  cap            int              max events per agent

  Add(events []Event)
    Appends each event to its agent's scope. Trims oldest when cap exceeded.

  ForAgent(name string) []Event
    Returns events for the named agent, newest first.

  All() []Event
    Returns all events across all agents, merged and sorted newest first.

  Clear()
    Removes all events.
```

### Scoping

Events are stored in a map keyed by agent name. Each agent independently respects the cap. A busy agent's events do not push out a quiet agent's events.

### Tests

- Add events, query by agent — correct scoping
- Cap enforcement — oldest events dropped
- Events from different agents do not interfere
- All() returns merged and sorted results
- ForAgent on unknown name returns empty slice

---

## 6. Poller (`internal/poller`)

Periodically produces snapshots and derives events.

### Interface

```
Poller
  interval       time.Duration
  configPath     string
  store          *events.EventStore
  prev           *model.Snapshot

  Poll() → (snapshot *model.Snapshot, newEvents []Event, err error)
    1. Reload config and agent identity registry
    2. Call builder.Build() to assemble current snapshot
    3. If prev is not nil, call events.Diff(prev, curr)
    4. Add new events to store
    5. Set prev = curr
    6. Return snapshot and new events
```

### bubbletea Integration

The poller is invoked via a `tea.Cmd` that sleeps for the interval, calls `Poll()`, and returns the result as a `tea.Msg`. The poller itself has no bubbletea dependency.

### Standalone Use

The poller can be used without the TUI for testing and for CLI commands (`watch list`, `watch status`) which do a single poll.

### Error Handling

The poller does not fail on transient errors. Partial snapshots are returned when some data sources are unavailable.

### Tests

- First poll: snapshot with instance_up events
- Subsequent poll with no changes: empty events
- Subsequent poll with changes: correct events
- Config change between polls: new project picked up
- tmux unavailable: empty snapshot, no crash
- Artifact errors: partial snapshot

---

## Composition

```
identity.Registry ──────────┐
                             │
config.Load() ───────────────┤
                             │
tmux.ListSessions() ─────────┼──→ snapshot.Builder.Build() ──→ model.Snapshot
                             │                                      │
orca.FindSessions() ─────────┘                                      │
                                                                    │
                                        ┌───────────────────────────┘
                                        │
                                  events.Diff(prev, curr) ──→ []model.Event
                                                                    │
                                                                    ▼
                                                        events.EventStore.Add()
                                                                    │
                                                                    ▼
                                                      EventStore.ForAgent() / .All()
                                                                    │
                                                                    ▼
                                                                TUI views
```

---

## Deferred Decisions

### Issue Titles

Summary.json contains issue IDs only. Title resolution (via `br show`) is deferred. The Run type can accommodate a title field when added.

### Queue State Performance

Queue state requires shelling out to `br` per project per poll. Included from the start for 1-2 projects. If polling performance becomes a problem, queue reads can be made less frequent than the main poll interval.

### Matching Rule Tuning

Global and project-local identities can be matched with explicit rules (`match.session_pattern`, `match.path_prefix`). Ambiguous matches are intentionally dropped. If this proves too strict in practice, the matching strategy can be extended with deterministic precedence/scoring while preserving safety.

### Metrics Integration

The metrics.jsonl file contains additional data (tokens, durations) that could enrich runs. Not included initially. The snapshot builder can be extended to read metrics without model changes.

### Session Persistence

Events and snapshots are in-memory only. Watch is live-only. Historical review is a separate workflow.

### Multiple Agent Identities Per Project

Multiple identities per project are supported through explicit matching rules. If no explicit rule matches and more than one fallback candidate remains, the session is ignored to avoid incorrect attribution.

---

## Implementation Notes

Observations from the initial implementation that inform future work.

### Working Directory Matching

Non-orca session matching uses `tmux`'s reported working directory (`pane_current_path`). This matches any tmux session whose working directory is within a registered project path, which can produce false positives. For example, a session that was opened for an unrelated purpose but whose shell happens to be in a project directory will match as an agent instance.

This is acceptable for the initial implementation because the operator controls the identity registry and can observe incorrect matches. A more robust matching mechanism (explicit session tagging, process inspection, or naming conventions) may be needed if false positives become a persistent problem.

### Queue State

Queue state reading is stubbed in the poller (returns `QueueState.Available = false`). Integrating with `br` requires shelling out per project per poll. This was deferred because the queue state display is secondary to agent/instance visibility. It should be implemented when orca's queue integration is validated cross-project.

### Identity Extraction

The `internal/identity` package is designed with extraction to lore in mind. It has no dependencies on other watch internal packages. The file format (`agents.json`) and the `Registry` interface are intentionally simple. When lore is built, this package moves there and watch becomes a consumer of lore's registry rather than owning it.

### Event Derivation

Events are derived from snapshot diffs, not from real-time observation. This means events that happen between poll intervals (e.g., a session appears and disappears within 3 seconds) are missed. This is acceptable for the current poll-based model. If higher fidelity is needed, filesystem watchers or tmux hooks could supplement polling, but the complexity cost is not justified yet.
