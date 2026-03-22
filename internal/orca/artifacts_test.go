package orca

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestIsOrcaSession(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		prefix string
		want   bool
	}{
		{"standard", "orca-agent-1-20260320T091355Z", "", true},
		{"agent 2", "orca-agent-2-20260315T142237Z", "", true},
		{"custom prefix", "custom-1-20260320T091355Z", "custom", true},
		{"no timestamp", "orca-agent-1", "", false},
		{"wrong prefix", "other-agent-1-20260320T091355Z", "", false},
		{"empty", "", "", false},
		{"prefix only", "orca-agent-", "", false},
		{"just prefix", "orca-agent", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsOrcaSession(tt.input, tt.prefix)
			if got != tt.want {
				t.Errorf("IsOrcaSession(%q, %q) = %v, want %v", tt.input, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestExtractAgentSlot(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		prefix string
		want   string
	}{
		{"standard", "orca-agent-1-20260320T091355Z", "", "1"},
		{"agent 2", "orca-agent-2-20260315T142237Z", "", "2"},
		{"custom prefix", "custom-1-20260320T091355Z", "custom", "1"},
		{"not orca", "other-1-20260320T091355Z", "", ""},
		{"no timestamp", "orca-agent-1", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractAgentSlot(tt.input, tt.prefix)
			if got != tt.want {
				t.Errorf("ExtractAgentSlot(%q, %q) = %q, want %q", tt.input, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestExtractSessionID(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		prefix string
		want   string
	}{
		{"standard", "orca-agent-1-20260320T091355Z", "", "1-20260320T091355Z"},
		{"agent 2", "orca-agent-2-20260315T142237Z", "", "2-20260315T142237Z"},
		{"custom prefix", "custom-1-20260320T091355Z", "custom", "1-20260320T091355Z"},
		{"not orca", "other-1-20260320T091355Z", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractSessionID(tt.input, tt.prefix)
			if got != tt.want {
				t.Errorf("ExtractSessionID(%q, %q) = %q, want %q", tt.input, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestFindSessions(t *testing.T) {
	// Build a fake artifact directory.
	root := t.TempDir()
	sessDir := filepath.Join(root, AgentLogsDir, SessionsSubdir, "2026", "03", "20")
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Session with one completed run.
	runDir := filepath.Join(sessDir, "agent-1-20260320T100000Z", RunsSubdir, "0001-20260320T100001Z")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	summary := Summary{
		IssueID: "orca-42",
		Result:  "completed",
		Merged:  true,
		Notes:   "test run",
	}
	data, _ := json.Marshal(summary)
	if err := os.WriteFile(filepath.Join(runDir, SummaryFile), data, 0644); err != nil {
		t.Fatal(err)
	}

	// Session with no runs.
	emptySession := filepath.Join(sessDir, "agent-2-20260320T100000Z")
	if err := os.MkdirAll(emptySession, 0755); err != nil {
		t.Fatal(err)
	}

	sessions, err := FindSessions(root, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	// Find the session with runs.
	var withRuns *SessionInfo
	for i := range sessions {
		if sessions[i].SessionID == "agent-1-20260320T100000Z" {
			withRuns = &sessions[i]
		}
	}
	if withRuns == nil {
		t.Fatal("expected to find session agent-1-20260320T100000Z")
	}
	if len(withRuns.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(withRuns.Runs))
	}
	if !withRuns.Runs[0].HasSummary {
		t.Error("expected run to have summary")
	}
	if withRuns.Runs[0].Summary.IssueID != "orca-42" {
		t.Errorf("expected issue orca-42, got %q", withRuns.Runs[0].Summary.IssueID)
	}
	if withRuns.Runs[0].LogPath == "" {
		t.Error("expected LogPath to be set")
	}
	if withRuns.Runs[0].SummaryPath == "" {
		t.Error("expected SummaryPath to be set")
	}
}

func TestFindSessions_NoArtifactDir(t *testing.T) {
	sessions, err := FindSessions("/nonexistent/path", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}
