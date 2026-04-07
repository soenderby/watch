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

// Sources provides the raw data the poller needs. Tests can replace
// the default implementation to avoid real tmux/filesystem dependencies.
type Sources struct {
	LoadConfig    func() (*config.Config, error)
	LoadRegistry  func(cfg *config.Config) (*identity.Registry, error)
	ListSessions  func() ([]tmux.Session, error)
	ReadArtifacts func(cfg *config.Config) map[string]snapshot.ProjectArtifacts
}

// Poller produces snapshots and derives events by diffing consecutive snapshots.
type Poller struct {
	sources Sources
	store   *events.Store
	prev    *model.Snapshot
}

// New creates a new poller with default sources that read from real
// tmux sessions, filesystem config, and orca artifacts.
func New(configPath string, store *events.Store) *Poller {
	return &Poller{
		sources: defaultSources(configPath),
		store:   store,
	}
}

// NewWithSources creates a poller with injectable data sources.
func NewWithSources(sources Sources, store *events.Store) *Poller {
	return &Poller{
		sources: sources,
		store:   store,
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
	cfg, err := p.sources.LoadConfig()
	if err != nil {
		cfg = &config.Config{}
	}

	reg, err := p.sources.LoadRegistry(cfg)
	if err != nil {
		reg, _ = identity.BuildRegistry("", nil)
	}

	tmuxSessions, err := p.sources.ListSessions()
	if err != nil {
		tmuxSessions = nil
	}

	var projectConfigs []snapshot.ProjectConfig
	for _, proj := range cfg.Projects {
		projectConfigs = append(projectConfigs, snapshot.ProjectConfig{
			Name: proj.Name,
			Path: proj.Path,
		})
	}

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

	artifacts := p.sources.ReadArtifacts(cfg)

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

func defaultSources(configPath string) Sources {
	return Sources{
		LoadConfig: func() (*config.Config, error) {
			return config.Load(configPath)
		},
		LoadRegistry: func(cfg *config.Config) (*identity.Registry, error) {
			var sources []identity.ProjectSource
			for _, proj := range cfg.Projects {
				sources = append(sources, identity.ProjectSource{
					Name: proj.Name,
					Path: proj.Path,
				})
			}
			globalPath, err := identity.DefaultGlobalPath()
			if err != nil {
				globalPath = ""
			}
			return identity.BuildRegistry(globalPath, sources)
		},
		ListSessions: tmux.ListSessions,
		ReadArtifacts: func(cfg *config.Config) map[string]snapshot.ProjectArtifacts {
			artifacts := make(map[string]snapshot.ProjectArtifacts)
			for _, proj := range cfg.Projects {
				sessions, err := orca.FindSessions(proj.Path, 3)
				if err != nil {
					sessions = nil
				}
				artifacts[proj.Name] = snapshot.ProjectArtifacts{
					Sessions: sessions,
					Queue:    readQueueState(proj.Path),
				}
			}
			return artifacts
		},
	}
}

// readQueueState shells out to br to read queue state for a project.
func readQueueState(projectPath string) model.QueueState {
	// For now, return unavailable. Queue reading requires br to be installed
	// and a .beads directory to exist. This will be implemented when the
	// poller is integrated with real projects.
	return model.QueueState{Available: false}
}
