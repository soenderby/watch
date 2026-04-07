package main

import (
	"bufio"
	"errors"
	"strings"
	"testing"

	"github.com/soenderby/watch/internal/config"
	"github.com/soenderby/watch/internal/tmux"
)

func TestParseAdoptOptions(t *testing.T) {
	opts, err := parseAdoptOptions([]string{"session-1", "--name", "reviewer", "--project", "orca", "--yes"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.SessionName != "session-1" {
		t.Fatalf("expected session-1, got %q", opts.SessionName)
	}
	if opts.Name != "reviewer" {
		t.Fatalf("expected reviewer, got %q", opts.Name)
	}
	if opts.Project != "orca" {
		t.Fatalf("expected orca, got %q", opts.Project)
	}
	if !opts.Yes {
		t.Fatal("expected yes=true")
	}
}

func TestParseAdoptOptions_Help(t *testing.T) {
	_, err := parseAdoptOptions([]string{"session-1", "--help"})
	if !errors.Is(err, errHelpRequested) {
		t.Fatalf("expected errHelpRequested, got %v", err)
	}
}

func TestResolveAdoptIdentity_ProjectLocalDefaults(t *testing.T) {
	cfg := &config.Config{Projects: []config.Project{{Name: "orca", Path: "/code/orca"}}}
	reader := bufio.NewReader(strings.NewReader("\n\n\n\n"))
	session := tmux.Session{Name: "review-1", Path: "/code/orca/worktrees/reviewer"}

	resolved, err := resolveAdoptIdentity(adoptOptions{Name: "reviewer"}, reader, session, "orca", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.TargetFile != "/code/orca/agents.json" {
		t.Fatalf("expected project-local target file, got %q", resolved.TargetFile)
	}
	if resolved.Identity.Project != "" {
		t.Fatalf("expected empty project for project-local file, got %q", resolved.Identity.Project)
	}
	if resolved.Identity.Match == nil {
		t.Fatal("expected match rules")
	}
}

func TestResolveAdoptIdentity_NameRequiredWithYes(t *testing.T) {
	cfg := &config.Config{}
	reader := bufio.NewReader(strings.NewReader(""))
	session := tmux.Session{Name: "review-1", Path: "/tmp"}

	_, err := resolveAdoptIdentity(adoptOptions{Yes: true, Global: true}, reader, session, "", cfg)
	if err == nil {
		t.Fatal("expected error when --yes used without --name")
	}
}
