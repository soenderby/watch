// Package identityflow implements identity discovery and adoption workflows.
package identityflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/soenderby/watch/internal/config"
	"github.com/soenderby/watch/internal/identity"
	"github.com/soenderby/watch/internal/orca"
	"github.com/soenderby/watch/internal/tmux"
)

// Suggestion contains suggested explicit match rules for a candidate session.
type Suggestion struct {
	SessionPattern string `json:"session_pattern,omitempty"`
	PathPrefix     string `json:"path_prefix,omitempty"`
}

// Candidate is an unmatched tmux session that could be adopted as an identity.
type Candidate struct {
	SessionName     string     `json:"session_name"`
	Path            string     `json:"path,omitempty"`
	Windows         int        `json:"windows"`
	Activity        time.Time  `json:"activity"`
	ActivityAge     string     `json:"activity_age"`
	InferredProject string     `json:"inferred_project,omitempty"`
	Reason          string     `json:"reason"`
	Confidence      string     `json:"confidence"`
	Suggested       Suggestion `json:"suggested_match,omitempty"`
}

// DiscoverCandidates returns unmatched sessions that are likely agent sessions.
func DiscoverCandidates(sessions []tmux.Session, matched map[string]bool, projects []config.Project, projectFilter string, includeLow bool) []Candidate {
	candidates := make([]Candidate, 0)
	for _, s := range sessions {
		if matched[s.Name] {
			continue
		}
		candidate := buildCandidate(s, projects)
		if projectFilter != "" && candidate.InferredProject != projectFilter {
			continue
		}
		if !includeLow && candidate.Confidence == "low" {
			continue
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func buildCandidate(s tmux.Session, projects []config.Project) Candidate {
	inferredProject, _ := InferProjectForPath(s.Path, projects)
	reason := "unmatched_tmux_session"
	confidence := "low"

	if inferredProject != "" {
		reason = "under_registered_project"
		confidence = "high"
	} else if orca.IsOrcaSession(s.Name, "") {
		reason = "orca_like_name_no_match"
		confidence = "medium"
	} else if looksAgentLikeName(s.Name) {
		reason = "agent_like_name"
		confidence = "medium"
	}

	return Candidate{
		SessionName:     s.Name,
		Path:            s.Path,
		Windows:         s.Windows,
		Activity:        s.Activity,
		ActivityAge:     formatAge(time.Since(s.Activity)),
		InferredProject: inferredProject,
		Reason:          reason,
		Confidence:      confidence,
		Suggested: Suggestion{
			SessionPattern: SuggestSessionPattern(s.Name),
			PathPrefix:     s.Path,
		},
	}
}

// InferProjectForPath returns the most specific project containing path.
func InferProjectForPath(path string, projects []config.Project) (string, bool) {
	if path == "" {
		return "", false
	}
	best := ""
	bestLen := -1
	tie := false
	for _, p := range projects {
		if !isPathWithin(path, p.Path) {
			continue
		}
		l := len(filepath.Clean(p.Path))
		if l > bestLen {
			best = p.Name
			bestLen = l
			tie = false
			continue
		}
		if l == bestLen {
			tie = true
		}
	}
	if best == "" || tie {
		return "", false
	}
	return best, true
}

func looksAgentLikeName(name string) bool {
	lower := strings.ToLower(name)
	for _, part := range []string{"agent", "review", "writer", "worker", "librarian"} {
		if strings.Contains(lower, part) {
			return true
		}
	}
	return false
}

// SuggestSessionPattern suggests a match.session_pattern for a session name.
func SuggestSessionPattern(name string) string {
	if orca.IsOrcaSession(name, "") {
		slot := orca.ExtractAgentSlot(name, "")
		if slot != "" {
			return "orca-agent-" + slot + "-*"
		}
	}
	if idx := strings.LastIndex(name, "-"); idx > 0 {
		suffix := name[idx+1:]
		if isDigits(suffix) {
			return name[:idx+1] + "*"
		}
	}
	return name
}

// SuggestPathPrefix suggests a match.path_prefix for a session path.
func SuggestPathPrefix(sessionPath string, project *config.Project) string {
	if sessionPath == "" {
		return ""
	}
	if project == nil {
		return filepath.Clean(sessionPath)
	}
	rel, err := filepath.Rel(project.Path, sessionPath)
	if err != nil {
		return filepath.Clean(sessionPath)
	}
	if rel == "." {
		return ""
	}
	if strings.HasPrefix(rel, "..") {
		return filepath.Clean(sessionPath)
	}
	return filepath.Clean(rel)
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isPathWithin(child, parent string) bool {
	child = filepath.Clean(child)
	parent = filepath.Clean(parent)
	if child == parent {
		return true
	}
	return strings.HasPrefix(child, parent+string(filepath.Separator))
}

func formatAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// FindSessionByName returns the named tmux session.
func FindSessionByName(sessions []tmux.Session, name string) (tmux.Session, bool) {
	for _, s := range sessions {
		if s.Name == name {
			return s, true
		}
	}
	return tmux.Session{}, false
}

// PreviewMatches returns sessions currently matched by identity rules.
func PreviewMatches(id identity.AgentIdentity, sessions []tmux.Session, project *config.Project) []string {
	effective := id
	if effective.Match != nil && effective.Match.PathPrefix != "" && project != nil {
		copy := *effective.Match
		copy.PathPrefix = identity.ResolvePathPrefix(copy.PathPrefix, project.Path)
		effective.Match = &copy
	}

	var matched []string
	for _, s := range sessions {
		if project != nil && s.Path != "" && !isPathWithin(s.Path, project.Path) {
			continue
		}
		if effective.MatchesSession(s.Name, s.Path) {
			matched = append(matched, s.Name)
		}
	}
	return matched
}

// ValidateIdentityForAdopt validates identity configuration before write.
func ValidateIdentityForAdopt(id identity.AgentIdentity, projectScoped bool) error {
	if strings.TrimSpace(id.Name) == "" {
		return fmt.Errorf("identity name cannot be empty")
	}
	if id.Match != nil && id.Match.SessionPattern != "" {
		if _, err := filepath.Match(id.Match.SessionPattern, "probe"); err != nil {
			return fmt.Errorf("invalid session pattern: %w", err)
		}
	}
	if !projectScoped && !id.HasExplicitMatch() {
		return fmt.Errorf("global identities require at least one explicit match rule")
	}
	return nil
}

// EnsureNameNotInFile validates that no identity with name exists in file.
func EnsureNameNotInFile(path, name string) error {
	agents, err := identity.LoadFile(path)
	if err != nil {
		return fmt.Errorf("load target file: %w", err)
	}
	for _, a := range agents {
		if a.Name == name {
			return fmt.Errorf("identity %q already exists in %s", name, path)
		}
	}
	return nil
}

// AppendIdentityToFile appends an identity to the agents file at path.
func AppendIdentityToFile(path string, agent identity.AgentIdentity) error {
	agents, err := identity.LoadFile(path)
	if err != nil {
		return fmt.Errorf("load identities: %w", err)
	}
	agents = append(agents, agent)
	return writeAgentsFile(path, agents)
}

type agentsFile struct {
	Agents []identity.AgentIdentity `json:"agents"`
}

func writeAgentsFile(path string, agents []identity.AgentIdentity) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	payload := agentsFile{Agents: agents}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal identities: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), ".agents-*.json")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace target file: %w", err)
	}
	return nil
}
