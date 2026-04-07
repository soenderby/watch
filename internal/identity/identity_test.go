package identity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile_Missing(t *testing.T) {
	agents, err := LoadFile("/nonexistent/path/agents.json")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if len(agents) != 0 {
		t.Fatalf("expected empty list for missing file, got %d agents", len(agents))
	}
}

func TestLoadFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agents.json")
	data := `{
		"agents": [
			{"name": "worker", "project": "myproject", "description": "A worker"},
			{"name": "reviewer", "description": "Reviews code"}
		]
	}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	agents, err := LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}
	if agents[0].Name != "worker" {
		t.Errorf("expected name 'worker', got %q", agents[0].Name)
	}
	if agents[0].Project != "myproject" {
		t.Errorf("expected project 'myproject', got %q", agents[0].Project)
	}
	if agents[1].Name != "reviewer" {
		t.Errorf("expected name 'reviewer', got %q", agents[1].Name)
	}
	if agents[1].Project != "" {
		t.Errorf("expected empty project for reviewer, got %q", agents[1].Project)
	}
}

func TestLoadFile_MatchRules(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agents.json")
	data := `{
		"agents": [
			{
				"name": "reviewer",
				"match": {
					"session_pattern": "review-*",
					"path_prefix": "/code/review"
				}
			}
		]
	}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	agents, err := LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Match == nil {
		t.Fatal("expected match rules")
	}
	if agents[0].Match.SessionPattern != "review-*" {
		t.Fatalf("expected session pattern review-*, got %q", agents[0].Match.SessionPattern)
	}
	if agents[0].Match.PathPrefix != "/code/review" {
		t.Fatalf("expected path prefix /code/review, got %q", agents[0].Match.PathPrefix)
	}
}

func TestLoadFile_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agents.json")
	if err := os.WriteFile(path, []byte(`{not json}`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestBuildRegistry_GlobalOnly(t *testing.T) {
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "agents.json")
	writeAgents(t, globalPath, []AgentIdentity{
		{Name: "worker", Project: "proj", Description: "A worker"},
		{Name: "reviewer", Description: "Reviews code"},
	})

	reg, err := BuildRegistry(globalPath, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.Len() != 2 {
		t.Fatalf("expected 2 agents, got %d", reg.Len())
	}
}

func TestBuildRegistry_MergeProjectLocal(t *testing.T) {
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global-agents.json")
	writeAgents(t, globalPath, []AgentIdentity{
		{Name: "worker", Project: "proj", Description: "Global worker"},
	})

	projDir := filepath.Join(dir, "myproject")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeAgents(t, filepath.Join(projDir, "agents.json"), []AgentIdentity{
		{Name: "local-agent", Description: "Project-local agent"},
	})

	reg, err := BuildRegistry(globalPath, []ProjectSource{
		{Name: "myproject", Path: projDir},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.Len() != 2 {
		t.Fatalf("expected 2 agents, got %d", reg.Len())
	}

	// local-agent should have project set automatically.
	local := reg.ByName("local-agent")
	if local == nil {
		t.Fatal("expected to find local-agent")
	}
	if local.Project != "myproject" {
		t.Errorf("expected project 'myproject', got %q", local.Project)
	}
}

func TestBuildRegistry_ProjectLocalRelativePathPrefix(t *testing.T) {
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global-agents.json")
	writeAgents(t, globalPath, nil)

	projDir := filepath.Join(dir, "myproject")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeAgents(t, filepath.Join(projDir, "agents.json"), []AgentIdentity{
		{
			Name:  "reviewer",
			Match: &MatchRules{PathPrefix: "worktrees/reviewer"},
		},
	})

	reg, err := BuildRegistry(globalPath, []ProjectSource{{Name: "myproject", Path: projDir}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	a := reg.ByName("reviewer")
	if a == nil || a.Match == nil {
		t.Fatal("expected reviewer with match rules")
	}
	want := filepath.Join(projDir, "worktrees/reviewer")
	if a.Match.PathPrefix != want {
		t.Fatalf("expected normalized path prefix %q, got %q", want, a.Match.PathPrefix)
	}
}

func TestBuildRegistry_GlobalPrecedence(t *testing.T) {
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global-agents.json")
	writeAgents(t, globalPath, []AgentIdentity{
		{Name: "worker", Project: "global-proj", Description: "Global version"},
	})

	projDir := filepath.Join(dir, "myproject")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeAgents(t, filepath.Join(projDir, "agents.json"), []AgentIdentity{
		{Name: "worker", Project: "local-proj", Description: "Local version"},
	})

	reg, err := BuildRegistry(globalPath, []ProjectSource{
		{Name: "myproject", Path: projDir},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.Len() != 1 {
		t.Fatalf("expected 1 agent (global wins), got %d", reg.Len())
	}

	w := reg.ByName("worker")
	if w == nil {
		t.Fatal("expected to find worker")
	}
	if w.Description != "Global version" {
		t.Errorf("expected global description, got %q", w.Description)
	}
	if w.Project != "global-proj" {
		t.Errorf("expected global project, got %q", w.Project)
	}
}

func TestBuildRegistry_MissingGlobalFile(t *testing.T) {
	reg, err := BuildRegistry("/nonexistent/global.json", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.Len() != 0 {
		t.Fatalf("expected empty registry, got %d agents", reg.Len())
	}
}

func TestBuildRegistry_MalformedProjectLocal(t *testing.T) {
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global-agents.json")
	writeAgents(t, globalPath, []AgentIdentity{
		{Name: "worker", Description: "Global worker"},
	})

	projDir := filepath.Join(dir, "badproject")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "agents.json"), []byte(`{bad`), 0644); err != nil {
		t.Fatal(err)
	}

	// Malformed project-local should not cause failure.
	reg, err := BuildRegistry(globalPath, []ProjectSource{
		{Name: "badproject", Path: projDir},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.Len() != 1 {
		t.Fatalf("expected 1 agent (global only), got %d", reg.Len())
	}
}

func TestRegistry_Queries(t *testing.T) {
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "agents.json")
	writeAgents(t, globalPath, []AgentIdentity{
		{Name: "worker", Project: "orca", Description: "Batch worker"},
		{Name: "tester", Project: "orca", Description: "Test runner"},
		{Name: "reviewer", Description: "Code reviewer"},
		{Name: "librarian", Project: "ai-resources", Description: "Knowledge manager"},
	})

	reg, err := BuildRegistry(globalPath, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("All", func(t *testing.T) {
		all := reg.All()
		if len(all) != 4 {
			t.Fatalf("expected 4, got %d", len(all))
		}
	})

	t.Run("ByName found", func(t *testing.T) {
		a := reg.ByName("reviewer")
		if a == nil {
			t.Fatal("expected to find reviewer")
		}
		if a.Name != "reviewer" {
			t.Errorf("expected 'reviewer', got %q", a.Name)
		}
	})

	t.Run("ByName not found", func(t *testing.T) {
		a := reg.ByName("nonexistent")
		if a != nil {
			t.Fatal("expected nil for nonexistent agent")
		}
	})

	t.Run("ForProject", func(t *testing.T) {
		orcaAgents := reg.ForProject("orca")
		if len(orcaAgents) != 2 {
			t.Fatalf("expected 2 orca agents, got %d", len(orcaAgents))
		}
	})

	t.Run("ForProject empty", func(t *testing.T) {
		agents := reg.ForProject("nonexistent")
		if len(agents) != 0 {
			t.Fatalf("expected 0 agents for unknown project, got %d", len(agents))
		}
	})

	t.Run("Global", func(t *testing.T) {
		global := reg.Global()
		if len(global) != 1 {
			t.Fatalf("expected 1 global agent, got %d", len(global))
		}
		if global[0].Name != "reviewer" {
			t.Errorf("expected 'reviewer', got %q", global[0].Name)
		}
	})
}

func TestBuildRegistry_LocalLocalDedup(t *testing.T) {
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global-agents.json")
	writeAgents(t, globalPath, nil)

	proj1 := filepath.Join(dir, "proj1")
	proj2 := filepath.Join(dir, "proj2")
	for _, p := range []string{proj1, proj2} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	writeAgents(t, filepath.Join(proj1, "agents.json"), []AgentIdentity{
		{Name: "shared", Description: "From proj1"},
	})
	writeAgents(t, filepath.Join(proj2, "agents.json"), []AgentIdentity{
		{Name: "shared", Description: "From proj2"},
	})

	reg, err := BuildRegistry(globalPath, []ProjectSource{
		{Name: "proj1", Path: proj1},
		{Name: "proj2", Path: proj2},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.Len() != 1 {
		t.Fatalf("expected 1 agent (first wins), got %d", reg.Len())
	}

	a := reg.ByName("shared")
	if a.Description != "From proj1" {
		t.Errorf("expected first project to win, got description %q", a.Description)
	}
}

func TestAgentIdentity_MatchesSession(t *testing.T) {
	agent := AgentIdentity{
		Name: "reviewer",
		Match: &MatchRules{
			SessionPattern: "review-*",
			PathPrefix:     "/code/orca",
		},
	}

	if !agent.MatchesSession("review-1", "/code/orca/worktrees/r") {
		t.Fatal("expected match")
	}
	if agent.MatchesSession("worker-1", "/code/orca/worktrees/r") {
		t.Fatal("expected non-match by session")
	}
	if agent.MatchesSession("review-1", "/tmp") {
		t.Fatal("expected non-match by path")
	}
}

func TestResolvePathPrefix(t *testing.T) {
	if got := ResolvePathPrefix("worktrees/reviewer", "/code/orca"); got != "/code/orca/worktrees/reviewer" {
		t.Fatalf("unexpected resolved prefix: %q", got)
	}
	if got := ResolvePathPrefix("/abs/path", "/code/orca"); got != "/abs/path" {
		t.Fatalf("unexpected absolute prefix: %q", got)
	}
}

func TestAgentIdentity_HasExplicitMatch(t *testing.T) {
	tests := []struct {
		name  string
		agent AgentIdentity
		want  bool
	}{
		{name: "no rules", agent: AgentIdentity{Name: "a"}, want: false},
		{name: "empty rules", agent: AgentIdentity{Name: "a", Match: &MatchRules{}}, want: false},
		{name: "session rule", agent: AgentIdentity{Name: "a", Match: &MatchRules{SessionPattern: "x-*"}}, want: true},
		{name: "path rule", agent: AgentIdentity{Name: "a", Match: &MatchRules{PathPrefix: "/x"}}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.agent.HasExplicitMatch(); got != tt.want {
				t.Fatalf("HasExplicitMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func writeAgents(t *testing.T, path string, agents []AgentIdentity) {
	t.Helper()
	if agents == nil {
		agents = []AgentIdentity{}
	}
	f := agentsFile{Agents: agents}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
