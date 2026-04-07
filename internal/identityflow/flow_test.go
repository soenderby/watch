package identityflow

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/soenderby/watch/internal/config"
	"github.com/soenderby/watch/internal/identity"
	"github.com/soenderby/watch/internal/tmux"
)

func TestInferProjectForPath(t *testing.T) {
	projects := []config.Project{
		{Name: "root", Path: "/code"},
		{Name: "orca", Path: "/code/orca"},
	}

	got, ok := InferProjectForPath("/code/orca/worktrees/a", projects)
	if !ok || got != "orca" {
		t.Fatalf("expected project orca, got %q (ok=%v)", got, ok)
	}

	if _, ok := InferProjectForPath("/tmp/x", projects); ok {
		t.Fatal("expected no project match")
	}
}

func TestSuggestSessionPattern(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "orca-agent-2-20260320T100000Z", want: "orca-agent-2-*"},
		{in: "review-1", want: "review-*"},
		{in: "librarian", want: "librarian"},
	}

	for _, tt := range tests {
		if got := SuggestSessionPattern(tt.in); got != tt.want {
			t.Fatalf("SuggestSessionPattern(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSuggestPathPrefix(t *testing.T) {
	proj := &config.Project{Name: "orca", Path: "/code/orca"}
	if got := SuggestPathPrefix("/code/orca/worktrees/reviewer", proj); got != filepath.Clean("worktrees/reviewer") {
		t.Fatalf("unexpected project-local suggestion: %q", got)
	}

	if got := SuggestPathPrefix("/tmp/x", nil); got != "/tmp/x" {
		t.Fatalf("unexpected global suggestion: %q", got)
	}
}

func TestPreviewMatches_ProjectRelativePrefixResolved(t *testing.T) {
	proj := &config.Project{Name: "orca", Path: "/code/orca"}
	id := identity.AgentIdentity{
		Name: "reviewer",
		Match: &identity.MatchRules{
			PathPrefix: "worktrees/reviewer",
		},
	}

	sessions := []tmux.Session{
		{Name: "review-1", Path: "/code/orca/worktrees/reviewer", Activity: time.Now()},
		{Name: "review-2", Path: "/code/orca/worktrees/other", Activity: time.Now()},
	}

	matched := PreviewMatches(id, sessions, proj)
	if len(matched) != 1 || matched[0] != "review-1" {
		t.Fatalf("expected only review-1 to match, got %v", matched)
	}
}

func TestValidateIdentityForAdopt(t *testing.T) {
	globalNoRules := identity.AgentIdentity{Name: "reviewer"}
	if err := ValidateIdentityForAdopt(globalNoRules, false); err == nil {
		t.Fatal("expected error for global identity without explicit rules")
	}

	globalWithRules := identity.AgentIdentity{
		Name:  "reviewer",
		Match: &identity.MatchRules{SessionPattern: "review-*"},
	}
	if err := ValidateIdentityForAdopt(globalWithRules, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
