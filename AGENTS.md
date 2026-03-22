# Agent Instructions

Watch is an operator supervision tool for monitoring agent activity across projects. It is part of the orca ecosystem — see `docs/ecosystem.md` in the orca repo for the broader vision.

Watch is a Go project. The codebase should be small, clean, and habitable. Agents should accelerate evolution, not accelerate complexity.

---

## Start of Session

Read these before doing anything:

1. **This file** — project identity, conventions, quality expectations.
2. **`README.md`** — what watch does and how to use it.
3. **`docs/tui-design.md`** — TUI design specification.
4. **`docs/core-abstractions.md`** — data model and architecture.

Read deeper context only when the task requires it. Do not load everything into context by default.

---

## Project Structure

```
watch/
├── cmd/watch/         CLI entrypoint and command implementations
├── internal/
│   ├── identity/      Agent identity registry (planned for extraction)
│   ├── model/         Core types: Snapshot, Agent, Instance, Run, Event
│   ├── snapshot/      SnapshotBuilder — assembles snapshots from raw sources
│   ├── events/        Snapshot diffing and EventStore
│   ├── poller/        Periodic snapshot production
│   ├── tmux/          tmux session discovery and interaction
│   ├── orca/          Orca artifact reading
│   └── config/        Project registry
├── docs/              Design documents
└── go.mod
```

---

## Code Quality Standards

### Design

- **Modules must be deep.** Every package should hide complexity behind a simple interface. If removing a layer and inlining its implementation would not make callers harder to understand, the layer does not earn its existence.
- **Decompose around decisions, not workflow.** Package boundaries should hide volatile decisions (data sources, matching strategies, storage format), not mirror processing steps.
- **Domain-shaped, not framework-shaped.** Names, types, and boundaries should reflect what watch does (agents, instances, snapshots, events), not technical plumbing (handlers, managers, helpers, utils).
- **Simple, not easy.** Prefer fewer moving parts and less indirection over familiar patterns that add coupling. Do not introduce abstraction layers preemptively.
- **Data flows in one direction.** Raw sources → builder → snapshot → diff → events → store → views. No component reaches back into a previous layer.

### Implementation

- **Small functions with clear names.** A function should do one thing. Its name should say what that thing is.
- **Errors are values, not panics.** Return errors. Handle them at the appropriate level. Do not panic except in truly unrecoverable situations (program startup misconfig).
- **No global state.** Pass dependencies explicitly. No init() functions with side effects. No package-level mutable variables.
- **Interfaces are discovered, not designed.** Define an interface only when you have two or more concrete implementations, or when you need it for testing. Do not create interfaces preemptively.
- **Keep the dependency graph acyclic.** `model` depends on nothing. Everything else depends on `model`. No circular imports.

### Testing

- **Test behavior, not implementation.** Tests should verify what a function does, not how it does it. When the implementation changes, tests should only break if behavior changes.
- **Table-driven tests for pure functions.** Snapshot diffing, state derivation, matching logic — these are pure functions with clear input/output contracts. Use table-driven tests.
- **Test files live next to their code.** `snapshot/builder_test.go` tests `snapshot/builder.go`.
- **No test helpers that hide assertions.** Every test should make its assertions visible. A test that calls `assertEverythingIsCorrect(t, result)` is unreadable.
- **Tests must run without external dependencies.** No tmux required, no filesystem assumptions, no network. Use interfaces or test doubles where needed to isolate from tmux and the filesystem.
- **Run tests before committing.** `go test ./...` must pass. `go vet ./...` must pass.

### Review Checklist

Before accepting any change, verify:

- [ ] **Domain fit.** Do names and boundaries reflect agents, instances, and snapshots — not technical plumbing?
- [ ] **Module depth.** Does every package hide complexity? Could any layer be removed without loss?
- [ ] **Dependency direction.** Does `model` remain dependency-free? Is the graph still acyclic?
- [ ] **Simplicity.** Is this the simplest way to achieve the goal? Could anything be removed?
- [ ] **Test coverage.** Are the important behaviors tested? Not line coverage — behavior coverage.
- [ ] **Changeability.** Is the next change now easier or harder?
- [ ] **Proportionality.** Is the scope of change proportional to the task?

---

## Go Conventions

- **Format:** `gofmt` / `goimports`. Non-negotiable.
- **Naming:** Follow Go conventions. Exported names are `PascalCase`. Unexported are `camelCase`. No underscores in Go names. Package names are short, lowercase, single-word when possible.
- **Error messages:** Lowercase, no trailing punctuation. Prefix with the operation: `"read config: %w"`.
- **Comments:** Package comments on every package. Exported function comments when the name alone is not sufficient. No comments that restate the code.
- **Struct tags:** `json` tags on all types that may be serialized. Use `omitempty` for optional fields.

---

## Commands

```bash
# Build
go build -o watch ./cmd/watch/

# Test
go test ./...

# Vet
go vet ./...

# Format
goimports -w .

# All checks (run before committing)
go vet ./... && go test ./... && go build -o watch ./cmd/watch/
```

---

## Commit Conventions

- Short, descriptive messages in imperative mood.
- Prefix with the package or area when useful: `model: add Agent type`, `snapshot: implement builder`, `docs: update TUI design`.
- Small, logical commits. One concern per commit.
- Do not commit generated binaries.

---

## Decision Record

When a change affects the overall architecture — package boundaries, data model structure, dependency direction, or integration contracts — document the rationale. Either:

1. Update the relevant design document (`docs/core-abstractions.md`, `docs/tui-design.md`).
2. Or add a brief note in the commit message explaining *why*, not just *what*.

The goal is that a future reader (human or agent) can understand not just what the code does, but why it is shaped this way.
