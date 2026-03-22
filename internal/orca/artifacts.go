// Package orca reads orca run artifacts from a project's agent-logs directory.
package orca

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	DefaultSessionPrefix = "orca-agent"
	AgentLogsDir         = "agent-logs"
	SessionsSubdir       = "sessions"
	RunsSubdir           = "runs"
	SummaryFile          = "summary.json"
	MetricsFile          = "metrics.jsonl"
	LogFile              = "run.log"
)

// Summary represents the structured run summary from summary.json.
type Summary struct {
	IssueID          string   `json:"issue_id"`
	Result           string   `json:"result"`
	IssueStatus      string   `json:"issue_status"`
	Merged           bool     `json:"merged"`
	LoopAction       string   `json:"loop_action"`
	LoopActionReason string   `json:"loop_action_reason"`
	Notes            string   `json:"notes"`
	DiscoveryIDs     []string `json:"discovery_ids,omitempty"`
}

// RunInfo represents a single run within a session, as found in the artifact directory.
type RunInfo struct {
	RunID       string   `json:"run_id"`
	HasSummary  bool     `json:"has_summary"`
	Summary     *Summary `json:"summary,omitempty"`
	LogPath     string   `json:"log_path"`     // absolute path to run.log
	SummaryPath string   `json:"summary_path"` // absolute path to summary.json
}

// SessionInfo represents an orca session found in the artifact directory.
type SessionInfo struct {
	SessionID string    `json:"session_id"`
	DatePath  string    `json:"date_path"` // YYYY/MM/DD
	Runs      []RunInfo `json:"runs"`
	LatestRun *RunInfo  `json:"latest_run,omitempty"`
}

// orcaTimestampPattern matches the timestamp suffix in orca session names.
var orcaTimestampPattern = regexp.MustCompile(`-(\d{8}T\d{6}Z)$`)

// IsOrcaSession reports whether a tmux session name matches the orca naming convention.
// The convention is <prefix>-<index>-<timestamp>, e.g. "orca-agent-1-20260320T091355Z".
func IsOrcaSession(name string, prefix string) bool {
	if prefix == "" {
		prefix = DefaultSessionPrefix
	}
	if !strings.HasPrefix(name, prefix+"-") {
		return false
	}
	// Must end with a timestamp.
	return orcaTimestampPattern.MatchString(name)
}

// ExtractAgentSlot extracts the agent slot identifier from an orca tmux session name.
// For "orca-agent-1-20260320T091355Z" with prefix "orca-agent", returns "agent-1".
// Returns empty string if the name doesn't match the convention.
func ExtractAgentSlot(name string, prefix string) string {
	if prefix == "" {
		prefix = DefaultSessionPrefix
	}
	if !strings.HasPrefix(name, prefix+"-") {
		return ""
	}
	// Strip prefix and trailing timestamp.
	rest := strings.TrimPrefix(name, prefix+"-")
	loc := orcaTimestampPattern.FindStringIndex(rest)
	if loc == nil {
		return ""
	}
	slot := rest[:loc[0]]
	if slot == "" {
		return ""
	}
	// Remove trailing dash if present.
	return strings.TrimSuffix(slot, "-")
}

// ExtractSessionID extracts the orca session ID from an orca tmux session name.
// For "orca-agent-1-20260320T091355Z" with prefix "orca-agent", returns
// "agent-1-20260320T091355Z" (the part after the prefix).
func ExtractSessionID(name string, prefix string) string {
	if prefix == "" {
		prefix = DefaultSessionPrefix
	}
	if !strings.HasPrefix(name, prefix+"-") {
		return ""
	}
	return strings.TrimPrefix(name, prefix+"-")
}

// FindSessions scans a project's agent-logs directory for session artifacts.
// It looks at the most recent N date directories to avoid scanning the entire history.
func FindSessions(repoPath string, maxDateDirs int) ([]SessionInfo, error) {
	sessionsRoot := filepath.Join(repoPath, AgentLogsDir, SessionsSubdir)
	if _, err := os.Stat(sessionsRoot); os.IsNotExist(err) {
		return nil, nil
	}

	dateDirs, err := findDateDirs(sessionsRoot, maxDateDirs)
	if err != nil {
		return nil, err
	}

	var sessions []SessionInfo
	for _, dateDir := range dateDirs {
		datePath := dateDir.relative
		entries, err := os.ReadDir(dateDir.absolute)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			sessionID := entry.Name()
			sessionPath := filepath.Join(dateDir.absolute, sessionID)
			info := SessionInfo{
				SessionID: sessionID,
				DatePath:  datePath,
			}
			info.Runs = findRuns(sessionPath)
			if len(info.Runs) > 0 {
				latest := info.Runs[len(info.Runs)-1]
				info.LatestRun = &latest
			}
			sessions = append(sessions, info)
		}
	}
	return sessions, nil
}

// ReadSummary reads a summary.json file and returns the parsed summary.
func ReadSummary(path string) (*Summary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Summary
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

type dateDir struct {
	absolute string
	relative string // YYYY/MM/DD
}

func findDateDirs(sessionsRoot string, maxDirs int) ([]dateDir, error) {
	if maxDirs <= 0 {
		maxDirs = 7
	}

	var dirs []dateDir

	years, err := os.ReadDir(sessionsRoot)
	if err != nil {
		return nil, err
	}
	for _, y := range years {
		if !y.IsDir() {
			continue
		}
		yearPath := filepath.Join(sessionsRoot, y.Name())
		months, err := os.ReadDir(yearPath)
		if err != nil {
			continue
		}
		for _, m := range months {
			if !m.IsDir() {
				continue
			}
			monthPath := filepath.Join(yearPath, m.Name())
			days, err := os.ReadDir(monthPath)
			if err != nil {
				continue
			}
			for _, d := range days {
				if !d.IsDir() {
					continue
				}
				dirs = append(dirs, dateDir{
					absolute: filepath.Join(monthPath, d.Name()),
					relative: filepath.Join(y.Name(), m.Name(), d.Name()),
				})
			}
		}
	}

	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].relative > dirs[j].relative
	})

	if len(dirs) > maxDirs {
		dirs = dirs[:maxDirs]
	}
	return dirs, nil
}

func findRuns(sessionPath string) []RunInfo {
	runsPath := filepath.Join(sessionPath, RunsSubdir)
	entries, err := os.ReadDir(runsPath)
	if err != nil {
		return nil
	}

	var runs []RunInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		runID := entry.Name()
		runPath := filepath.Join(runsPath, runID)
		summaryPath := filepath.Join(runPath, SummaryFile)
		logPath := filepath.Join(runPath, LogFile)

		ri := RunInfo{
			RunID:       runID,
			LogPath:     logPath,
			SummaryPath: summaryPath,
		}
		if summary, err := ReadSummary(summaryPath); err == nil {
			ri.HasSummary = true
			ri.Summary = summary
		}
		runs = append(runs, ri)
	}

	sort.Slice(runs, func(i, j int) bool {
		return runs[i].RunID < runs[j].RunID
	})
	return runs
}
