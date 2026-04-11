# Quality of Life Backlog

A list of potential improvements grouped by effort. Each entry includes
rationale, files to touch, implementation notes, and test guidance.

Pick items freely — none of them depend on each other unless noted.

---

## Ecosystem alignment (from user-story review)

These items come from reviewing watch against the orca ecosystem docs
(`orca/docs/ecosystem.md`, `orca/docs/user-stories.md`,
`orca/docs/decision-log.md` DL-003). They close specific gaps between
the current implementation and the documented vision, rather than
general QoL polish.

Pick order recommendation: E3 first (trivial), then E1 (biggest UX
gap), then E4 (morning glance), then E2, then E5 (doc cleanup).

---

### E1. Global event buffer view (scrollable TUI screen)

**Value:** Matches user story W1 ("scroll through the event/notification
buffer") and scenarios 1, 4, 9 which describe passive monitoring
through an accumulating buffer. This is the **biggest current ecosystem
gap** — `events.Store.All()` already returns merged events newest-first,
but no TUI surface renders it.

**Design approach:** Add a new view to the existing navigation stack
rather than a persistent side/bottom panel. This keeps the zoom-based
model consistent and is much simpler than a split layout. The user has
room on screen for a larger view; the events view can fill the full
content area.

From the overview, press `e` to push the events view. `esc` pops back.

**Files:**
- `internal/tui/events_view.go` (new)
- `internal/tui/tui.go` (register footer + wire `e` key in overview delegation)
- `internal/tui/overview.go` (handle `e` key → push events view)

**Implementation:**

The view is structurally similar to `log_pager.go` — a windowed
scrollable list backed by a slice.

```go
type eventsView struct {
    offset int
}

func (v *eventsView) title() string { return "events" }

func (v *eventsView) update(msg tea.Msg, snap *model.Snapshot, store *events.Store) (view, action) {
    // handle j/k/d/u/G/g scroll + esc
}

func (v *eventsView) render(snap *model.Snapshot, store *events.Store, width, height int) string {
    all := store.All() // already newest-first
    // clamp offset to [0, len(all)-1]
    // render window: all[offset : offset+height]
    // format: TIME  AGENT  TYPE  DETAILS
}
```

Rendering format (one event per line):

```
10:44  worker-2  run_completed  orca-9dy  completed  merged
10:42  worker-1  run_started    orca-688
10:38  worker-1  instance_up
```

Include a `[offset/total]` footer indicator so the operator knows where
they are in the buffer.

**Keybindings inside the events view:**
- `j` / `down`: offset++
- `k` / `up`: offset--
- `d`: offset += visibleLines/2
- `u`: offset -= visibleLines/2
- `G`: jump to oldest
- `g`: jump to newest
- `esc`: pop back
- `r`: force refresh

**Tests:**
- Scroll clamping: offset cannot go below 0 or past len-1.
- Render window with empty store.
- Render window with 3 events, height=24.
- Render window with 100 events, offset=50, height=10 — check only
  visible slice is rendered.

**Notes / open question:**
- When the operator is deep in the events view and a new event arrives,
  should the offset stay where it is (stable) or jump to top (follow)?
  Recommendation: **stay**, so scrolling feels stable. Add a `f` key
  later if follow-mode is wanted.
- When entering the view, should the offset default to 0 (newest) or
  remember across pushes? Recommendation: **always 0 on push**. The
  operator wanted to see events; they want the newest.

**Effort:** ~90 min.

---

### E2. Unmatched sessions view with one-keystroke adopt

**Value:** Closes the tension between user-story #9 ("ad-hoc sessions
should not require ceremony") and DL-003 ("unmatched sessions are
invisible"). The identity discover/adopt CLI already exists; this
surfaces it inside the TUI so the operator never has to drop to a
shell.

W1 also says the TUI should show sessions grouped or labeled by type
("orca agent, project, ad-hoc"). This delivers the "ad-hoc" bucket.

**Design approach:** A dedicated view accessed via `u` from overview.
Rendering logic is separate from the agents list, which keeps the
overview uncluttered. The view shows unmatched sessions and a suggested
`watch identity adopt` command for each.

Inline TUI adoption (shelling out to the interactive flow while the
TUI is running) is complex — bubbletea controls the terminal. For the
first version, the view **displays the exact command** to copy-paste
and does not try to adopt in-process.

**Files:**
- `internal/model/model.go` (add `UnmatchedSession` type and `Snapshot.Unmatched []UnmatchedSession`)
- `internal/snapshot/builder.go` (populate unmatched during matching)
- `internal/tui/unmatched_view.go` (new)
- `internal/tui/overview.go` (handle `u` key → push unmatched view; show summary line "N unmatched — press u" if > 0)
- `internal/tui/tui.go` (footer text for the new view)

**Data model addition:**

```go
type UnmatchedSession struct {
    SessionName string    `json:"session_name"`
    Path        string    `json:"path,omitempty"`
    Activity    time.Time `json:"activity"`
    Windows     int       `json:"windows"`
}

// Added to Snapshot:
Unmatched []UnmatchedSession `json:"unmatched,omitempty"`
```

**Builder changes:**

In `snapshot.Build`, replace the `_ = matched` line with explicit
unmatched collection:

```go
if !matched {
    snap.Unmatched = append(snap.Unmatched, model.UnmatchedSession{
        SessionName: ts.Name,
        Path:        ts.Path,
        Activity:    ts.Activity,
        Windows:     ts.Windows,
    })
}
```

Sort deterministically (by name) at the end, same as agents.

**View render:**

```
Unmatched sessions (3)

> control       /home/jsk                        2h
  task-queue    /mnt/c/code/task-queue           18s
  htop          /                                1d

To adopt the selected session:
  watch identity adopt control
```

Cursor `>` moves through the list. Footer shows a copyable adopt command
based on cursor position.

**Keybindings:**
- `j` / `k`: move cursor
- `enter`: (later, Phase 2) could launch adopt; for now noop
- `esc`: pop back

**Tests:**
- snapshot builder test: unmatched sessions populated in Snapshot.
- Unmatched view render with 0, 1, many sessions.
- Cursor movement bounds.

**Notes:**
- Keep the default overview summary line short: `3 unmatched — press u`.
  Only render it when count > 0.
- Phase 2 (later): actually run adopt from within the TUI using
  `tea.ExecProcess` to suspend/resume bubbletea around the interactive
  adopt flow.

**Effort:** ~2 hours.

---

### E3. `watch status` state breakdown

**Value:** User story W2 quotes the exact format:
*"watch status — one-line summary: 3 active, 2 finished, 0 failed"*.
Our current output (`3 agents, 3 instances`) doesn't match.

**Files:**
- `cmd/watch/status.go`

**Implementation:**
- After loading the snapshot, count agents by `agent.State`.
- Text output:
  ```
  3 agents: 2 running, 1 done, 0 failed, 0 blocked
  ```
- JSON: extend `StatusSummary` with `ByState map[string]int`.

```go
type StatusSummary struct {
    Agents    int            `json:"agents"`
    Instances int            `json:"instances"`
    ByState   map[string]int `json:"by_state"`
}
```

**Tests:**
- Unit test a small counting helper: `countByState([]*model.Agent) map[string]int`.
- Test JSON output shape stability.

**Effort:** ~15 min.

---

### E4. "New since last seen" tracking

**Value:** User story #1 ("Starting the Day") says watch should show
*"which have new output since last seen"*. This is the morning-glance
affordance. It answers "what's different since yesterday?" without the
operator having to drill into every agent.

**Design decisions (recommended defaults):**

1. **Scope** — per-agent last-seen timestamp. Simpler than per-event.
2. **Trigger** — mark seen when the operator enters agent detail for
   that agent. Consistent with "I looked at it."
3. **Storage** — `~/.local/state/watch/seen.json` (XDG state dir).
4. **Persistence** — save on every mark. File is tiny; no need for
   debouncing.
5. **Display** — show a `●` marker next to agents that have events
   newer than their last-seen timestamp.
6. **Agents never seen** — treat as "all events are new."
7. **CLI `list`** — does not consult seen state (CLI is stateless).

**Files:**
- `internal/seen/seen.go` (new package)
- `internal/seen/seen_test.go` (new)
- `internal/tui/tui.go` (wire store into app, pass to views)
- `internal/tui/overview.go` (render marker, needs access to store)
- `internal/tui/agent_detail.go` (mark seen on render)

**Package interface:**

```go
package seen

type Store struct {
    path     string
    seen     map[string]time.Time
}

func Load(path string) (*Store, error)      // empty store if file missing
func DefaultPath() (string, error)           // ~/.local/state/watch/seen.json
func (s *Store) Get(agent string) time.Time  // zero time if unknown
func (s *Store) Mark(agent string, at time.Time) error  // updates + persists
func (s *Store) HasNew(agent string, events []model.Event) bool
```

`HasNew` is the display predicate: returns true if any event's
timestamp is after the agent's last-seen timestamp (or if never seen).

**TUI wiring:**

The overview view already receives `store *events.Store` in
`render()`. Extend view interface or add a second parameter for the
seen store — or pass both via a small `viewContext` struct to avoid
signature churn if more state arrives later.

Minimally invasive path: add `seen *seen.Store` as a field on `App` and
pass it through like `events.Store`.

```go
// in overview.render:
if seenStore.HasNew(agent.Name, store.ForAgent(agent.Name)) {
    header = cursor + "● " + agent.Name
} else {
    header = cursor + "  " + agent.Name
}
```

`agent_detail.update` marks seen when the view renders (or on entry,
via a dedicated `OnEnter` hook if we add one).

**Tests:**
- Seen store: load, save, get/set, load-after-save roundtrip.
- Missing file → empty store, no error.
- `HasNew` truth table: never seen / all old / some new.
- Temp-file based test for atomic save.

**Notes:**
- XDG state dir lookup: use `os.UserConfigDir()` parent, or hardcode to
  `~/.local/state/watch/` — check `os.UserHomeDir()` and add the path.
- Persistence failures should warn but not crash the TUI.
- First run with no seen file: everyone has `●`. That's correct — this
  is the first time the operator has looked.

**Effort:** ~90 min.

---

### E5. Docs alignment: update orca user-stories scenario #9

**Value:** Resolves the contradiction between `user-stories.md` #9
("ad-hoc sessions should not require registration or ceremony") and
`decision-log.md` DL-003 ("unmatched sessions are invisible to watch").
DL-003 is newer and authoritative; the user story should reflect it.

**Files:**
- `/mnt/c/code/orca/docs/user-stories.md` (note: orca repo, not watch)

**Edit:**
- Scenario #9 "Ad-Hoc Tasks":
  - Remove or rewrite the claim that "Watch discovers tmux sessions
    automatically" and "Ad-hoc sessions should not require registration
    or ceremony."
  - Replace with a description aligned to DL-003: ad-hoc sessions are
    visible in watch's unmatched sessions view (once E2 lands) and can
    be adopted into the identity registry with a single command. For
    very short-lived sessions, adoption is optional — watch simply does
    not display them.
  - Add a cross-reference to DL-003.

**Effort:** ~10 min. Do this **after** E2 so the doc can reference the
unmatched view by name.

---

## Small (≤30 minutes each)

### 1. `watch doctor`

**Value:** Cheapest diagnostic command. Answers "what is this binary and
what can it see?" in one call. Would have prevented the earlier local vs
global binary drift issue.

**Files:**
- `cmd/watch/doctor.go` (new)
- `cmd/watch/main.go` (register command)

**Implementation:**
- Print resolved binary path (`os.Executable()`)
- Print version via `versionString()`
- Print config path (`config.DefaultPath()`)
- Print global identity registry path (`identity.DefaultGlobalPath()`)
- Print `tmux.Installed()` result
- Print registered projects and, for each, whether its `agents.json`
  exists
- Print registered identity count

**Tests:**
- No unit tests needed. This is a thin reporting command. Smoke test in
  `dev-sync.sh` is sufficient.

**Effort:** ~20 min.

---

### 2. Color agent states (opt-out)

**Value:** Scannable at-a-glance state. Design doc requires the TUI to
remain fully usable without color, so this must be a non-essential
enhancement.

**Files:**
- `internal/tui/overview.go`
- `internal/tui/agent_detail.go`
- `cmd/watch/main.go` (add `--no-color`)
- possibly a new `internal/tui/style.go`

**Implementation:**
- Use `lipgloss` styles conditionally based on `termenv` TTY detection
  and a `--no-color` flag.
- States to color: `running` (neutral/dim), `failed` (red), `blocked`
  (yellow), `done` (green).
- Do not color anything else. Keep the monochrome structural layout
  intact — color is decoration only.

**Tests:**
- Ensure render output with color disabled is byte-identical to today.
- Behavior tests in `tui_test.go` should assert on the unstyled string.

**Effort:** ~30 min.

---

### 3. `watch list --watch`

**Value:** Repeated polling for CLI users without running the full TUI.
Good for quick monitoring in a second tmux pane.

**Files:**
- `cmd/watch/list.go`

**Implementation:**
- Add `--watch` flag (optionally `--interval <seconds>`, default 3s).
- Loop: clear screen (`\033[2J\033[H`), call `singlePoll()`, render
  the same table as today, sleep interval.
- Stop on SIGINT (handled by default).

**Tests:**
- Skip. Behavior is the existing list output; only the loop is new.

**Effort:** ~15 min.

---

### 4. Sort projects deterministically

**Value:** Same risk class as the agent-order bug we already fixed. The
project list comes from the config file (which is stable), but better
to enforce it in rendering.

**Files:**
- `cmd/watch/project.go`

**Implementation:**
- In `runProjectList`, sort `cfg.Projects` by name before rendering.
- `cfg.Projects` itself is a slice and currently preserves insertion
  order, so this is a display-only change.

**Tests:**
- Table-driven test around a sort helper. Nothing heavier needed.

**Effort:** ~10 min.

---

### 5. `watch identity list`

**Value:** Right now there is no way to see registered identities
without reading JSON files. Closes an obvious gap next to `discover`
and `adopt`.

**Files:**
- `cmd/watch/identity.go`

**Implementation:**
- Load the registry the same way `adopt` does.
- Print columns: NAME, PROJECT, SOURCE (global/project-local), MATCH.
- Support `--json`.
- Support `--project <name>` filter.

**Tests:**
- Add a small test in `cmd/watch/identity_test.go` that constructs a
  registry and asserts the rendered output contains expected rows.

**Effort:** ~25 min.

---

### 6. `watch identity remove <name>`

**Value:** Symmetric with `adopt`. Currently removal requires hand-edit.

**Files:**
- `cmd/watch/identity.go`
- `internal/identityflow/flow.go` (add `RemoveIdentityFromFile`)

**Implementation:**
- Resolve which file contains the identity (global or project-local).
- Refuse to remove across files by default; require `--file` to be
  explicit when ambiguous.
- Confirm before writing (skip with `--yes`).
- Use the same atomic temp+rename write used by `AppendIdentityToFile`.

**Tests:**
- Unit test `RemoveIdentityFromFile` in `identityflow` using temp files.

**Effort:** ~30 min.

---

### 7. Show identity count in `status`

**Value:** Immediate feedback when adding identities. Confirms the
registry was loaded.

**Note:** Best implemented together with **E3** (state breakdown) —
both touch `cmd/watch/status.go` and extend `StatusSummary`. Combine
into one change.

**Files:**
- `cmd/watch/status.go`

**Implementation:**
- After building the summary, also load the registry and count
  identities.
- Change text output to: `N agents: X running, Y done, Z failed, K identities registered`.
- Add `Identities int` field to the JSON output struct.

**Tests:**
- Existing tests should continue to pass. Optional: add a test for the
  struct shape.

**Effort:** ~10 min (free when bundled with E3).

---

### 8. Friendlier empty states

**Value:** Guides new users toward the next command. Currently an empty
`list` just says `(no agents with active instances)` which doesn't
explain why.

**Files:**
- `cmd/watch/list.go`
- `cmd/watch/status.go`
- `internal/tui/overview.go`

**Implementation:**
- If no projects are registered: suggest `watch project add`.
- If projects exist but no identities: suggest `watch identity discover`.
- If identities exist but no matches: suggest `watch identity discover --all`.
- TUI overview mirrors the same hints.

**Tests:**
- Add small render tests to `tui_test.go` that exercise empty states.

**Effort:** ~20 min.

---

## Medium (30–90 minutes each)

### 9. `identity adopt` interactive candidate picker

**Value:** Running `watch identity adopt` with no args currently errors.
It should instead run `discover`, present a numbered list, let the user
pick one, and fall through into the normal adopt flow.

**Files:**
- `cmd/watch/identity.go`

**Implementation:**
- In `runIdentityAdopt`, if no positional argument is given and stdin is
  a TTY, call `identityflow.DiscoverCandidates` and print a numbered
  list. Read a number. Use that candidate's session name as the target.
- Behavior with `--yes` and no session name remains an error.

**Tests:**
- Factor candidate selection into a small helper that takes an `io.Reader`
  so it can be unit-tested with `strings.NewReader`.

**Effort:** ~45 min.

---

### 10. TUI `/` filter

**Value:** Very common TUI pattern. Useful when the overview has many
agents.

**Files:**
- `internal/tui/overview.go`
- `internal/tui/tui.go`

**Implementation:**
- Add a `filter string` field to `overviewView`.
- `/` enters filter mode. Typing appends to the filter. `esc` clears it.
- `render` filters agents where `strings.Contains(strings.ToLower(agent.Name), filter)`.
- Show filter at the bottom of the screen: `filter: <text>`.

**Tests:**
- Unit test the filtering logic (pure function on `[]*model.Agent`).

**Effort:** ~60 min.

---

### 11. TUI cursor persistence across polls

**Value:** Today, every poll rebuilds the snapshot and the cursor stays
at its index. If agents are added/removed, the cursor may point to a
different agent or jump unexpectedly.

**Files:**
- `internal/tui/overview.go`
- `internal/tui/agent_detail.go`

**Implementation:**
- Track cursor by **name** rather than index in each view.
- On render, look up the index of the selected name in the current
  snapshot. If the name no longer exists, fall back to clamped index.

**Tests:**
- Test that when an agent is added before the cursor, the selected
  agent is still the same one by name.
- Test clamping fallback when the selected name disappears.

**Effort:** ~60 min.

---

### 12. Log pager search

**Value:** The pager already loads and displays `run.log`; search is the
natural missing feature.

**Files:**
- `internal/tui/log_pager.go`

**Implementation:**
- Add a `search string` and `searchHits []int` (line indices).
- `/` enters search mode. Typing builds the query. `enter` jumps to
  the next match. `n` / `N` move through hits. `esc` clears search.

**Tests:**
- Unit test the pure search helper (input: lines + query, output: hit
  indices).

**Effort:** ~75 min.

---

### 13. TUI status line for last poll

**Value:** Failures are silent today. If the poller returns an error the
snapshot stays at its old state and the user cannot tell.

**Files:**
- `internal/tui/tui.go`

**Implementation:**
- Track `lastPollAt time.Time` and `lastPollErr error` on `App`.
- Update them in `Update` when handling `pollMsg`.
- Render a single footer line above the key hints: either `polled 10:42`
  or `poll failed: <error>`.

**Tests:**
- Render test for both success and failure states.

**Effort:** ~40 min.

---

### 14. Global `--config <path>` flag

**Value:** Enables running multiple configurations side by side (e.g.
one for real work, one for testing). Currently the config path is
implicit.

**Files:**
- `cmd/watch/main.go`
- Every `run*` function in `cmd/watch/` that currently calls
  `configPath()`

**Implementation:**
- Parse `--config` from `os.Args` before dispatching.
- Thread the resolved path into each subcommand via a small `cliContext`
  struct or explicit argument.
- Default remains `config.DefaultPath()`.

**Tests:**
- Test the arg parser in isolation. Don't try to test every command.

**Effort:** ~60 min.

---

### 15. `watch list --agent <name>`

**Value:** Cheap filter for scripts and quick single-agent checks.

**Files:**
- `cmd/watch/list.go`

**Implementation:**
- Add `--agent <name>` flag.
- After `singlePoll()`, filter `snap.Agents` before rendering.
- Works for both text and JSON output.

**Tests:**
- Add a small test for the filter helper.

**Effort:** ~20 min (promoted from Small if combined with `--watch`).

---

### 16. Preserve cursor on force-refresh

**Value:** Subsumed by #11 if implemented together. List separately
because it can also be solved cheaply by doing nothing in `r` except
triggering `doPoll()`.

**Files:**
- `internal/tui/tui.go`

**Implementation:**
- Confirm that `r` does not reset any view's cursor. It currently does
  not. Add a test so future refactors keep it that way.

**Tests:**
- Test that after `handleKey("r")`, the current view pointer and cursor
  state are unchanged.

**Effort:** ~15 min.

---

## Larger but still bounded (half a day each)

### 17. TUI overview grouping toggle

**Value:** When many projects and agents exist, the flat list becomes
hard to scan. Grouping by project makes the overview more structured.

**Files:**
- `internal/tui/overview.go`

**Implementation:**
- Add a `grouped bool` flag on `overviewView`.
- Press a key (e.g. `v` for view-mode) to toggle.
- In grouped mode, render a project heading and then the agents belonging
  to that project, followed by a "Global" section.

**Tests:**
- Test both render modes.

**Effort:** ~2 hours.

---

### 18. `watch events` CLI command

**Value:** Surfaces the event store without running the TUI. Useful for
piping into external tools and for debugging matching/diff behavior.
The CLI counterpart to **E1**.

**Note:** Because events are derived from snapshot diffs, a single-poll
CLI invocation always sees "first snapshot" — every current instance
produces an `instance_up` event. This is documented behavior, not a
bug, but worth surfacing in the command help.

**Files:**
- `cmd/watch/events.go` (new)
- `cmd/watch/main.go` (register)

**Implementation:**
- Run a single poll so the store has current state.
- Print either `store.All()` or `store.ForAgent(name)` if `--agent` is
  set.
- Support `--json` and `--limit <N>`.

**Tests:**
- Command-level test that ensures empty output is `[]` in JSON mode.

**Effort:** ~90 min.

---

### 19. `watch list --json --follow` NDJSON streaming

**Value:** Enables external tools to consume watch data as a stream.

**Files:**
- `cmd/watch/list.go`

**Implementation:**
- Add `--follow` (or reuse `--watch`).
- Emit one JSON object per poll, newline-separated. Flush `stdout`
  after each emission.
- Exit on SIGINT.

**Tests:**
- Not needed. Manual check is enough.

**Effort:** ~45 min.

---

### 20. TUI preferences file

**Value:** Persistent defaults for poll interval, color preference, and
default view mode.

**Files:**
- `internal/config/config.go` (extend)
- `internal/tui/tui.go` (consume)

**Implementation:**
- Extend `Config` with an optional `TUI` sub-struct:
  `{ PollInterval string, Color string, DefaultView string }`.
- Backwards-compatible: missing fields fall back to current defaults.
- Load in `runTUI` and pass into `tui.New`.

**Tests:**
- Test that an existing config with no `tui` block still loads.
- Test that values are applied.

**Effort:** ~2 hours.

---

## Suggested bundles

Pick a bundle for a focused session:

- **"Ecosystem alignment bundle"** — E3, E1, E4. ~3.5 hours. Closes the
  biggest gaps between watch and the user stories in one focused pass:
  state breakdown, scrollable events buffer, morning-glance new markers.

- **"Identity alignment bundle"** — E2, E5. ~2 hours. Adds the unmatched
  sessions view and aligns the orca docs with DL-003. Do E2 first so
  E5 can reference the new view by name.

- **"Diagnostics bundle"** — 1, 5, 7+E3, 8. ~90 min. Addresses
  visibility and discoverability across the whole CLI.

- **"TUI polish bundle"** — 11, 13, 16. ~2 hours. Addresses the biggest
  daily-use annoyances in the TUI.

- **"Identity workflow bundle"** — 5, 6, 9. ~100 min. Closes every
  remaining gap in the identity CLI.

- **"Scripting bundle"** — 3, 14, 15, 18, 19. ~3 hours. Makes watch
  usable as a data source for other tools. Note that 18 is the CLI
  half of E1; consider doing them together so the data paths match.

---

## Items to deliberately skip for now

- Queue state integration — waiting on the new task queue tool.
- Issue title resolution via `br show` — secondary information.
- Metrics.jsonl enrichment — secondary data.
- Lore extraction of `identity` — waits until lore exists.
