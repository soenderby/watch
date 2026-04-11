package snapshot

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/soenderby/watch/internal/identity"
	"github.com/soenderby/watch/internal/model"
	"github.com/soenderby/watch/internal/orca"
)

var t0 = time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)

func TestBuild_Empty(t *testing.T) {
	snap := Build(Input{Timestamp: t0})
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(snap.Agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(snap.Agents))
	}
}

func TestBuild_OrcaMatching(t *testing.T) {
	reg := registryWith(t, []identity.AgentIdentity{
		{Name: "worker", Project: "orca"},
	})

	snap := Build(Input{
		Timestamp: t0,
		Registry:  reg,
		Projects:  []ProjectConfig{{Name: "orca", Path: "/code/orca"}},
		TmuxSessions: []TmuxSession{
			{Name: "orca-agent-1-20260320T100000Z", Windows: 1, Activity: t0},
		},
		Artifacts: map[string]ProjectArtifacts{
			"orca": {
				Sessions: []orca.SessionInfo{
					{
						SessionID: "1-20260320T100000Z",
						Runs: []orca.RunInfo{
							{RunID: "0001", HasSummary: true, Summary: &orca.Summary{
								IssueID: "orca-42", Result: "completed", Merged: true,
							}},
						},
					},
				},
			},
		},
	})

	if len(snap.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(snap.Agents))
	}
	agent := snap.Agents[0]
	if agent.Name != "worker" {
		t.Errorf("expected agent 'worker', got %q", agent.Name)
	}
	if agent.Project != "orca" {
		t.Errorf("expected project 'orca', got %q", agent.Project)
	}
	if len(agent.Instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(agent.Instances))
	}
	inst := agent.Instances[0]
	if inst.Orca == nil {
		t.Fatal("expected orca enrichment")
	}
	if inst.Orca.AgentName != "1" {
		t.Errorf("expected agent slot '1', got %q", inst.Orca.AgentName)
	}
	if len(inst.Orca.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(inst.Orca.Runs))
	}
	if inst.Orca.Runs[0].IssueID != "orca-42" {
		t.Errorf("expected issue 'orca-42', got %q", inst.Orca.Runs[0].IssueID)
	}
}

func TestBuild_NonOrcaMatching(t *testing.T) {
	reg := registryWith(t, []identity.AgentIdentity{
		{Name: "librarian", Project: "ai-resources"},
	})

	snap := Build(Input{
		Timestamp: t0,
		Registry:  reg,
		Projects:  []ProjectConfig{{Name: "ai-resources", Path: "/code/ai-resources"}},
		TmuxSessions: []TmuxSession{
			{Name: "my-session", Path: "/code/ai-resources/subdir", Activity: t0},
		},
		Artifacts: map[string]ProjectArtifacts{},
	})

	if len(snap.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(snap.Agents))
	}
	agent := snap.Agents[0]
	if agent.Name != "librarian" {
		t.Errorf("expected 'librarian', got %q", agent.Name)
	}
	inst := agent.Instances[0]
	if inst.Orca != nil {
		t.Error("expected no orca enrichment for non-orca session")
	}
	if inst.State != model.InstanceStateActive {
		t.Errorf("expected state 'active', got %q", inst.State)
	}
}

func TestBuild_UnmatchedIgnored(t *testing.T) {
	reg := registryWith(t, []identity.AgentIdentity{
		{Name: "worker", Project: "orca"},
	})

	snap := Build(Input{
		Timestamp: t0,
		Registry:  reg,
		Projects:  []ProjectConfig{{Name: "orca", Path: "/code/orca"}},
		TmuxSessions: []TmuxSession{
			{Name: "random-session", Path: "/some/other/path"},
			{Name: "htop-monitor"},
		},
		Artifacts: map[string]ProjectArtifacts{},
	})

	if len(snap.Agents) != 0 {
		t.Fatalf("expected 0 agents (unmatched ignored), got %d", len(snap.Agents))
	}
}

func TestBuild_DeterministicAgentOrder(t *testing.T) {
	reg := registryWith(t, []identity.AgentIdentity{
		{Name: "zeta", Project: "p"},
		{Name: "alpha", Project: "p"},
		{Name: "mike", Project: "p"},
	})

	// Give each identity exactly one matching session so all three
	// agents appear in the snapshot.
	input := Input{
		Timestamp: t0,
		Registry:  reg,
		Projects:  []ProjectConfig{{Name: "p", Path: "/code/p"}},
		TmuxSessions: []TmuxSession{
			{Name: "zeta-1", Path: "/code/p", Activity: t0},
		},
	}

	// Because there are multiple fallback-eligible identities, no agent
	// would be attributed. Give them explicit rules instead.
	reg = registryWith(t, []identity.AgentIdentity{
		{Name: "zeta", Project: "p", Match: &identity.MatchRules{SessionPattern: "zeta-*"}},
		{Name: "alpha", Project: "p", Match: &identity.MatchRules{SessionPattern: "alpha-*"}},
		{Name: "mike", Project: "p", Match: &identity.MatchRules{SessionPattern: "mike-*"}},
	})
	input.Registry = reg
	input.TmuxSessions = []TmuxSession{
		{Name: "zeta-1", Path: "/code/p", Activity: t0},
		{Name: "alpha-1", Path: "/code/p", Activity: t0},
		{Name: "mike-1", Path: "/code/p", Activity: t0},
	}

	// Build several times; order must be stable.
	var firstOrder []string
	for i := 0; i < 10; i++ {
		snap := Build(input)
		names := make([]string, 0, len(snap.Agents))
		for _, a := range snap.Agents {
			names = append(names, a.Name)
		}
		if i == 0 {
			firstOrder = names
			continue
		}
		if len(names) != len(firstOrder) {
			t.Fatalf("iteration %d: length mismatch", i)
		}
		for j := range names {
			if names[j] != firstOrder[j] {
				t.Fatalf("iteration %d: order changed, got %v want %v", i, names, firstOrder)
			}
		}
	}

	want := []string{"alpha", "mike", "zeta"}
	if len(firstOrder) != len(want) {
		t.Fatalf("expected %d agents, got %d", len(want), len(firstOrder))
	}
	for i := range want {
		if firstOrder[i] != want[i] {
			t.Fatalf("expected agents sorted by name %v, got %v", want, firstOrder)
		}
	}
}

func TestBuild_MultipleInstances(t *testing.T) {
	reg := registryWith(t, []identity.AgentIdentity{
		{Name: "worker", Project: "orca"},
	})

	snap := Build(Input{
		Timestamp: t0,
		Registry:  reg,
		Projects:  []ProjectConfig{{Name: "orca", Path: "/code/orca"}},
		TmuxSessions: []TmuxSession{
			{Name: "orca-agent-1-20260320T100000Z", Activity: t0.Add(-1 * time.Minute)},
			{Name: "orca-agent-2-20260320T100000Z", Activity: t0},
		},
		Artifacts: map[string]ProjectArtifacts{
			"orca": {
				Sessions: []orca.SessionInfo{
					{SessionID: "1-20260320T100000Z"},
					{SessionID: "2-20260320T100000Z"},
				},
			},
		},
	})

	if len(snap.Agents) != 1 {
		t.Fatalf("expected 1 agent with 2 instances, got %d agents", len(snap.Agents))
	}
	if len(snap.Agents[0].Instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(snap.Agents[0].Instances))
	}
	// Most recently active should be first.
	if snap.Agents[0].Instances[0].SessionName != "orca-agent-2-20260320T100000Z" {
		t.Errorf("expected most recent first, got %q", snap.Agents[0].Instances[0].SessionName)
	}
}

func TestBuild_MatchingRules_OrcaSessionPattern(t *testing.T) {
	reg := registryWith(t, []identity.AgentIdentity{
		{Name: "worker-a", Project: "orca", Match: &identity.MatchRules{SessionPattern: "orca-agent-1-*"}},
		{Name: "worker-b", Project: "orca", Match: &identity.MatchRules{SessionPattern: "orca-agent-2-*"}},
	})

	snap := Build(Input{
		Timestamp: t0,
		Registry:  reg,
		Projects:  []ProjectConfig{{Name: "orca", Path: "/code/orca"}},
		TmuxSessions: []TmuxSession{
			{Name: "orca-agent-2-20260320T100000Z", Activity: t0},
		},
		Artifacts: map[string]ProjectArtifacts{
			"orca": {
				Sessions: []orca.SessionInfo{{SessionID: "2-20260320T100000Z"}},
			},
		},
	})

	if len(snap.Agents) != 1 {
		t.Fatalf("expected 1 matched agent, got %d", len(snap.Agents))
	}
	if snap.Agents[0].Name != "worker-b" {
		t.Fatalf("expected worker-b, got %q", snap.Agents[0].Name)
	}
}

func TestBuild_MatchingRules_ProjectAmbiguousWithoutRules(t *testing.T) {
	reg := registryWith(t, []identity.AgentIdentity{
		{Name: "worker-a", Project: "orca"},
		{Name: "worker-b", Project: "orca"},
	})

	snap := Build(Input{
		Timestamp: t0,
		Registry:  reg,
		Projects:  []ProjectConfig{{Name: "orca", Path: "/code/orca"}},
		TmuxSessions: []TmuxSession{
			{Name: "orca-agent-1-20260320T100000Z", Activity: t0},
		},
		Artifacts: map[string]ProjectArtifacts{
			"orca": {
				Sessions: []orca.SessionInfo{{SessionID: "1-20260320T100000Z"}},
			},
		},
	})

	if len(snap.Agents) != 0 {
		t.Fatalf("expected no match for ambiguous identities, got %d agents", len(snap.Agents))
	}
}

func TestBuild_MatchingRules_NonOrcaPathPrefix(t *testing.T) {
	reg := registryWith(t, []identity.AgentIdentity{
		{Name: "writer", Project: "docs", Match: &identity.MatchRules{PathPrefix: "/code/docs/writer"}},
		{Name: "reviewer", Project: "docs", Match: &identity.MatchRules{PathPrefix: "/code/docs/reviewer"}},
	})

	snap := Build(Input{
		Timestamp: t0,
		Registry:  reg,
		Projects:  []ProjectConfig{{Name: "docs", Path: "/code/docs"}},
		TmuxSessions: []TmuxSession{
			{Name: "session-1", Path: "/code/docs/reviewer/task", Activity: t0},
		},
		Artifacts: map[string]ProjectArtifacts{},
	})

	if len(snap.Agents) != 1 {
		t.Fatalf("expected 1 matched agent, got %d", len(snap.Agents))
	}
	if snap.Agents[0].Name != "reviewer" {
		t.Fatalf("expected reviewer, got %q", snap.Agents[0].Name)
	}
}

func TestBuild_MatchingRules_GlobalExplicit(t *testing.T) {
	reg := registryWith(t, []identity.AgentIdentity{
		{Name: "reviewer", Match: &identity.MatchRules{SessionPattern: "review-*"}},
	})

	snap := Build(Input{
		Timestamp: t0,
		Registry:  reg,
		TmuxSessions: []TmuxSession{
			{Name: "review-1", Path: "/tmp", Activity: t0},
		},
	})

	if len(snap.Agents) != 1 {
		t.Fatalf("expected 1 global match, got %d", len(snap.Agents))
	}
	if snap.Agents[0].Name != "reviewer" {
		t.Fatalf("expected reviewer, got %q", snap.Agents[0].Name)
	}
}

func TestBuild_MatchingRules_GlobalExplicitBeatsProjectFallback(t *testing.T) {
	reg := registryWith(t, []identity.AgentIdentity{
		{Name: "project-default", Project: "docs"},
		{Name: "reviewer", Match: &identity.MatchRules{SessionPattern: "review-*"}},
	})

	snap := Build(Input{
		Timestamp: t0,
		Registry:  reg,
		Projects:  []ProjectConfig{{Name: "docs", Path: "/code/docs"}},
		TmuxSessions: []TmuxSession{
			{Name: "review-1", Path: "/code/docs", Activity: t0},
		},
	})

	if len(snap.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(snap.Agents))
	}
	if snap.Agents[0].Name != "reviewer" {
		t.Fatalf("expected reviewer, got %q", snap.Agents[0].Name)
	}
}

func TestBuild_MatchingRules_GlobalNeedsExplicitRules(t *testing.T) {
	reg := registryWith(t, []identity.AgentIdentity{
		{Name: "reviewer"},
	})

	snap := Build(Input{
		Timestamp: t0,
		Registry:  reg,
		TmuxSessions: []TmuxSession{
			{Name: "review-1", Path: "/tmp", Activity: t0},
		},
	})

	if len(snap.Agents) != 0 {
		t.Fatalf("expected no global match without explicit rules, got %d", len(snap.Agents))
	}
}

func TestBuild_StateDerivation(t *testing.T) {
	tests := []struct {
		name       string
		hasSummary bool
		result     string
		wantState  string
	}{
		{"running no summary", false, "", model.InstanceStateRunning},
		{"completed", true, "completed", model.InstanceStateRunning}, // tmux alive + completed = between runs
		{"failed", true, "failed", model.InstanceStateFailed},
		{"blocked", true, "blocked", model.InstanceStateBlocked},
		{"no_work", true, "no_work", model.InstanceStateRunning},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := registryWith(t, []identity.AgentIdentity{
				{Name: "worker", Project: "orca"},
			})

			var summary *orca.Summary
			if tt.hasSummary {
				summary = &orca.Summary{Result: tt.result}
			}
			runs := []orca.RunInfo{{
				RunID:      "0001",
				HasSummary: tt.hasSummary,
				Summary:    summary,
			}}

			snap := Build(Input{
				Timestamp: t0,
				Registry:  reg,
				Projects:  []ProjectConfig{{Name: "orca", Path: "/code/orca"}},
				TmuxSessions: []TmuxSession{
					{Name: "orca-agent-1-20260320T100000Z"},
				},
				Artifacts: map[string]ProjectArtifacts{
					"orca": {
						Sessions: []orca.SessionInfo{
							{SessionID: "1-20260320T100000Z", Runs: runs},
						},
					},
				},
			})

			if len(snap.Agents) != 1 || len(snap.Agents[0].Instances) != 1 {
				t.Fatal("expected 1 agent with 1 instance")
			}
			got := snap.Agents[0].Instances[0].State
			if got != tt.wantState {
				t.Errorf("expected state %q, got %q", tt.wantState, got)
			}
		})
	}
}

func TestBuild_AgentStateAggregate(t *testing.T) {
	reg := registryWith(t, []identity.AgentIdentity{
		{Name: "worker", Project: "orca"},
	})

	snap := Build(Input{
		Timestamp: t0,
		Registry:  reg,
		Projects:  []ProjectConfig{{Name: "orca", Path: "/code/orca"}},
		TmuxSessions: []TmuxSession{
			{Name: "orca-agent-1-20260320T100000Z"},
			{Name: "orca-agent-2-20260320T100000Z"},
		},
		Artifacts: map[string]ProjectArtifacts{
			"orca": {
				Sessions: []orca.SessionInfo{
					{SessionID: "1-20260320T100000Z", Runs: []orca.RunInfo{
						{RunID: "0001", HasSummary: true, Summary: &orca.Summary{Result: "failed"}},
					}},
					{SessionID: "2-20260320T100000Z"}, // no runs = running
				},
			},
		},
	})

	if snap.Agents[0].State != model.AgentStateRunning {
		t.Errorf("expected agent state 'running', got %q", snap.Agents[0].State)
	}
}

func TestBuild_QueueState(t *testing.T) {
	reg := registryWith(t, []identity.AgentIdentity{
		{Name: "worker", Project: "orca"},
	})

	snap := Build(Input{
		Timestamp: t0,
		Registry:  reg,
		Projects:  []ProjectConfig{{Name: "orca", Path: "/code/orca"}},
		Artifacts: map[string]ProjectArtifacts{
			"orca": {
				Queue: model.QueueState{Ready: 3, InProgress: 1, Available: true},
			},
		},
	})

	proj := snap.ProjectByName("orca")
	if proj == nil {
		t.Fatal("expected project 'orca'")
	}
	if proj.Queue.Ready != 3 {
		t.Errorf("expected 3 ready, got %d", proj.Queue.Ready)
	}
	if !proj.Queue.Available {
		t.Error("expected queue available")
	}
}

func TestIsPathWithin(t *testing.T) {
	tests := []struct {
		child, parent string
		want          bool
	}{
		{"/code/orca", "/code/orca", true},
		{"/code/orca/subdir", "/code/orca", true},
		{"/code/orca-other", "/code/orca", false},
		{"/code/other", "/code/orca", false},
		{"/code", "/code/orca", false},
	}
	for _, tt := range tests {
		got := isPathWithin(tt.child, tt.parent)
		if got != tt.want {
			t.Errorf("isPathWithin(%q, %q) = %v, want %v", tt.child, tt.parent, got, tt.want)
		}
	}
}

// registryWith creates a test registry from a slice of identities.
func registryWith(t *testing.T, agents []identity.AgentIdentity) *identity.Registry {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/agents.json"

	payload := map[string]any{"agents": agents}
	data, err := json.Marshal(payload)
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
