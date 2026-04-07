package poller

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/soenderby/watch/internal/config"
	"github.com/soenderby/watch/internal/events"
	"github.com/soenderby/watch/internal/identity"
	"github.com/soenderby/watch/internal/model"
	"github.com/soenderby/watch/internal/orca"
	"github.com/soenderby/watch/internal/snapshot"
	"github.com/soenderby/watch/internal/tmux"
)

func testSources(cfg *config.Config, reg *identity.Registry, sessions []tmux.Session, artifacts map[string]snapshot.ProjectArtifacts) Sources {
	return Sources{
		LoadConfig: func() (*config.Config, error) { return cfg, nil },
		LoadRegistry: func(*config.Config) (*identity.Registry, error) {
			return reg, nil
		},
		ListSessions: func() ([]tmux.Session, error) { return sessions, nil },
		ReadArtifacts: func(*config.Config) map[string]snapshot.ProjectArtifacts {
			if artifacts == nil {
				return map[string]snapshot.ProjectArtifacts{}
			}
			return artifacts
		},
	}
}

func TestNew(t *testing.T) {
	p := New("/nonexistent/config.json", nil)
	if p == nil {
		t.Fatal("expected non-nil poller")
	}
}

func TestPoll_FirstPoll_InstanceUpEvents(t *testing.T) {
	reg := buildTestRegistry(t, []identity.AgentIdentity{
		{Name: "worker", Project: "proj"},
	})
	cfg := &config.Config{
		Projects: []config.Project{{Name: "proj", Path: "/code/proj"}},
	}
	sessions := []tmux.Session{
		{Name: "orca-agent-1-20260320T100000Z", Path: "/code/proj", Activity: time.Now()},
	}
	artifacts := map[string]snapshot.ProjectArtifacts{
		"proj": {Sessions: []orca.SessionInfo{{SessionID: "1-20260320T100000Z"}}},
	}

	store := events.NewStore(50)
	p := NewWithSources(testSources(cfg, reg, sessions, artifacts), store)
	result, err := p.Poll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(result.Snapshot.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(result.Snapshot.Agents))
	}
	if len(result.NewEvents) != 1 {
		t.Fatalf("expected 1 event (instance_up), got %d", len(result.NewEvents))
	}
	if result.NewEvents[0].Type != model.EventInstanceUp {
		t.Fatalf("expected instance_up, got %q", result.NewEvents[0].Type)
	}
}

func TestPoll_SecondPoll_NoChange(t *testing.T) {
	reg := buildTestRegistry(t, []identity.AgentIdentity{
		{Name: "worker", Project: "proj"},
	})
	cfg := &config.Config{
		Projects: []config.Project{{Name: "proj", Path: "/code/proj"}},
	}
	sessions := []tmux.Session{
		{Name: "my-session", Path: "/code/proj/subdir", Activity: time.Now()},
	}

	store := events.NewStore(50)
	p := NewWithSources(testSources(cfg, reg, sessions, nil), store)

	_, err := p.Poll()
	if err != nil {
		t.Fatalf("first poll error: %v", err)
	}

	result, err := p.Poll()
	if err != nil {
		t.Fatalf("second poll error: %v", err)
	}
	if len(result.NewEvents) != 0 {
		t.Fatalf("expected 0 events on unchanged poll, got %d", len(result.NewEvents))
	}
}

func TestPoll_InstanceDisappears(t *testing.T) {
	reg := buildTestRegistry(t, []identity.AgentIdentity{
		{Name: "worker", Project: "proj"},
	})
	cfg := &config.Config{
		Projects: []config.Project{{Name: "proj", Path: "/code/proj"}},
	}
	sessions := []tmux.Session{
		{Name: "my-session", Path: "/code/proj", Activity: time.Now()},
	}

	store := events.NewStore(50)
	src := testSources(cfg, reg, sessions, nil)
	p := NewWithSources(src, store)

	_, err := p.Poll()
	if err != nil {
		t.Fatalf("first poll error: %v", err)
	}

	p.sources.ListSessions = func() ([]tmux.Session, error) { return nil, nil }
	result, err := p.Poll()
	if err != nil {
		t.Fatalf("second poll error: %v", err)
	}
	if len(result.NewEvents) != 1 {
		t.Fatalf("expected 1 event (instance_down), got %d", len(result.NewEvents))
	}
	if result.NewEvents[0].Type != model.EventInstanceDown {
		t.Fatalf("expected instance_down, got %q", result.NewEvents[0].Type)
	}
}

func TestPoll_EventsAddedToStore(t *testing.T) {
	reg := buildTestRegistry(t, []identity.AgentIdentity{
		{Name: "worker", Project: "proj"},
	})
	cfg := &config.Config{
		Projects: []config.Project{{Name: "proj", Path: "/code/proj"}},
	}
	sessions := []tmux.Session{
		{Name: "my-session", Path: "/code/proj", Activity: time.Now()},
	}

	store := events.NewStore(50)
	p := NewWithSources(testSources(cfg, reg, sessions, nil), store)

	_, err := p.Poll()
	if err != nil {
		t.Fatalf("poll error: %v", err)
	}

	storeEvents := store.ForAgent("worker")
	if len(storeEvents) != 1 {
		t.Fatalf("expected 1 event in store, got %d", len(storeEvents))
	}
}

func TestPoll_EmptyConfig(t *testing.T) {
	reg := buildTestRegistry(t, nil)
	cfg := &config.Config{}

	store := events.NewStore(50)
	p := NewWithSources(testSources(cfg, reg, nil, nil), store)
	result, err := p.Poll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Snapshot.Agents) != 0 {
		t.Fatalf("expected 0 agents, got %d", len(result.Snapshot.Agents))
	}
}

func buildTestRegistry(t *testing.T, agents []identity.AgentIdentity) *identity.Registry {
	t.Helper()
	if agents == nil {
		agents = []identity.AgentIdentity{}
	}
	dir := t.TempDir()
	path := dir + "/agents.json"

	type file struct {
		Agents []identity.AgentIdentity `json:"agents"`
	}
	data, err := json.Marshal(file{Agents: agents})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	reg, err := identity.BuildRegistry(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	return reg
}
