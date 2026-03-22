// Package poller periodically produces snapshots and derives events.
package poller

import (
	"time"

	"github.com/soenderby/watch/internal/config"
	"github.com/soenderby/watch/internal/events"
	"github.com/soenderby/watch/internal/identity"
	"github.com/soenderby/watch/internal/model"
	"github.com/soenderby/watch/internal/orca"
	"github.com/soenderby/watch/internal/snapshot"
	"github.com/soenderby/watch/internal/tmux"
)

// Poller produces snapshots and derives events by diffing consecutive snapshots.
type Poller struct {
	configPath string
	store      *events.Store
	prev       *model.Snapshot
}

// New creates a new poller.
func New(configPath string, store *events.Store) *Poller {
	return &Poller{
		configPath: configPath,
		store:      store,
	}
}

// Result holds the output of a single poll.
type Result struct {
	Snapshot  *model.Snapshot
	NewEvents []model.Event
}

// Poll reads current state, builds a snapshot, diffs against the previous
// snapshot to derive events, and adds them to the store.
func (p *Poller) Poll() (*Result, error) {
	// Load config.
	cfg, err := config.Load(p.configPath)
	if err != nil {
		cfg = &config.Config{}
	}

	// Build project sources for identity registry.
	var identitySources []identity.ProjectSource
	var projectConfigs []snapshot.ProjectConfig
	for _, proj := range cfg.Projects {
		identitySources = append(identitySources, identity.ProjectSource{
			Name: proj.Name,
			Path: proj.Path,
		})
		projectConfigs = append(projectConfigs, snapshot.ProjectConfig{
			Name: proj.Name,
			Path: proj.Path,
		})
	}

	// Load agent identity registry.
	globalAgentsPath, err := identity.DefaultGlobalPath()
	if err != nil {
		globalAgentsPath = ""
	}
	reg, err := identity.BuildRegistry(globalAgentsPath, identitySources)
	if err != nil {
		reg, _ = identity.BuildRegistry("", nil)
	}

	// Read tmux sessions.
	tmuxSessions, err := tmux.ListSessions()
	if err != nil {
		tmuxSessions = nil
	}

	// Convert tmux sessions to snapshot input.
	var inputSessions []snapshot.TmuxSession
	for _, ts := range tmuxSessions {
		inputSessions = append(inputSessions, snapshot.TmuxSession{
			Name:     ts.Name,
			Windows:  ts.Windows,
			Created:  ts.Created,
			Attached: ts.Attached,
			Activity: ts.Activity,
			Path:     ts.Path,
		})
	}

	// Read orca artifacts and queue state per project.
	artifacts := make(map[string]snapshot.ProjectArtifacts)
	for _, proj := range cfg.Projects {
		sessions, err := orca.FindSessions(proj.Path, 3)
		if err != nil {
			sessions = nil
		}
		queue := readQueueState(proj.Path)
		artifacts[proj.Name] = snapshot.ProjectArtifacts{
			Sessions: sessions,
			Queue:    queue,
		}
	}

	// Build snapshot.
	snap := snapshot.Build(snapshot.Input{
		Timestamp:    time.Now(),
		TmuxSessions: inputSessions,
		Registry:     reg,
		Projects:     projectConfigs,
		Artifacts:    artifacts,
	})

	// Diff and derive events.
	newEvents := events.Diff(p.prev, snap)
	if len(newEvents) > 0 {
		p.store.Add(newEvents)
	}

	p.prev = snap

	return &Result{
		Snapshot:  snap,
		NewEvents: newEvents,
	}, nil
}

// readQueueState shells out to br to read queue state for a project.
func readQueueState(projectPath string) model.QueueState {
	// For now, return unavailable. Queue reading requires br to be installed
	// and a .beads directory to exist. This will be implemented when the
	// poller is integrated with real projects.
	return model.QueueState{Available: false}
}
