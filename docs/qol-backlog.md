# Quality of Life Backlog

A list of potential improvements grouped by effort. Each entry includes
rationale, files to touch, implementation notes, and test guidance.

Pick items freely — none of them depend on each other unless noted.

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

**Files:**
- `cmd/watch/status.go`

**Implementation:**
- After building the summary, also load the registry and count
  identities.
- Change text output to: `N agents, M instances, K identities`.
- Add `Identities int` field to the JSON output struct.

**Tests:**
- Existing tests should continue to pass. Optional: add a test for the
  struct shape.

**Effort:** ~10 min.

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

**Files:**
- `cmd/watch/events.go` (new)
- `cmd/watch/main.go` (register)

**Implementation:**
- Run a single poll so the store has current state.
- Print either `store.All()` or `store.ForAgent(name)` if `--agent` is
  set.
- Support `--json` and `--limit <N>`.
- Note that events are only the diff from the previous snapshot; the
  first poll will show `instance_up` for everything. Document this.

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

- **"Diagnostics bundle"** — 1, 5, 7, 8. ~75 min. Addresses visibility
  and discoverability across the whole CLI.

- **"TUI polish bundle"** — 11, 13, 16. ~2 hours. Addresses the biggest
  daily-use annoyances in the TUI.

- **"Identity workflow bundle"** — 5, 6, 9. ~100 min. Closes every
  remaining gap in the identity CLI.

- **"Scripting bundle"** — 3, 14, 15, 18, 19. ~3 hours. Makes watch
  usable as a data source for other tools.

---

## Items to deliberately skip for now

- Queue state integration — waiting on the new task queue tool.
- Issue title resolution via `br show` — secondary information.
- Metrics.jsonl enrichment — secondary data.
- Lore extraction of `identity` — waits until lore exists.
