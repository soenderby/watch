package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/soenderby/watch/internal/config"
	"github.com/soenderby/watch/internal/orca"
	"github.com/soenderby/watch/internal/tmux"
)

// SessionEntry is the unified view of a tmux session with optional orca enrichment.
type SessionEntry struct {
	Name       string `json:"name"`
	Type       string `json:"type"` // "orca" or "other"
	Windows    int    `json:"windows"`
	Attached   bool   `json:"attached"`
	CreatedAge string `json:"created_age"`
	// Orca-specific fields (nil for non-orca sessions)
	OrcaProject *string `json:"orca_project,omitempty"`
	OrcaResult  *string `json:"orca_result,omitempty"`
	OrcaIssue   *string `json:"orca_issue,omitempty"`
}

func runList(args []string) error {
	outputJSON := false
	for _, arg := range args {
		switch arg {
		case "--json":
			outputJSON = true
		case "--help", "-h":
			fmt.Println("Usage: watch list [--json]")
			fmt.Println("\nList all tmux sessions with state. Orca sessions are enriched with artifact data.")
			return nil
		default:
			return fmt.Errorf("unknown option: %s", arg)
		}
	}

	if !tmux.Installed() {
		return fmt.Errorf("tmux is not installed")
	}

	sessions, err := tmux.ListSessions()
	if err != nil {
		return err
	}

	// Load orca artifact data from registered projects
	orcaSessions := loadOrcaSessions()

	entries := buildEntries(sessions, orcaSessions)

	if outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	printTable(entries)
	return nil
}

func loadOrcaSessions() map[string]orcaMatch {
	result := make(map[string]orcaMatch)

	cfgPath, err := config.DefaultPath()
	if err != nil {
		return result
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return result
	}

	for _, proj := range cfg.Projects {
		sessions, err := orca.FindSessions(proj.Path, 3)
		if err != nil {
			continue
		}
		for _, s := range sessions {
			// Match by session ID prefix in tmux session name
			// Orca tmux sessions are named: <prefix>-<index>-<timestamp>
			// Session IDs in artifacts are: <agent-name>-<timestamp>
			// We match by checking if the tmux name contains the session ID
			result[s.SessionID] = orcaMatch{
				project: proj.Name,
				info:    s,
			}
		}
	}
	return result
}

type orcaMatch struct {
	project string
	info    orca.SessionInfo
}

func buildEntries(sessions []tmux.Session, orcaSessions map[string]orcaMatch) []SessionEntry {
	var entries []SessionEntry

	for _, s := range sessions {
		entry := SessionEntry{
			Name:       s.Name,
			Type:       "other",
			Windows:    s.Windows,
			Attached:   s.Attached,
			CreatedAge: formatAge(time.Since(s.Created)),
		}

		// Check if this is an orca session
		if orca.IsOrcaSession(s.Name, "") {
			entry.Type = "orca"

			// Try to find matching artifact data
			for sessionID, match := range orcaSessions {
				if strings.Contains(s.Name, sessionID) || strings.HasSuffix(s.Name, sessionID) {
					proj := match.project
					entry.OrcaProject = &proj
					if match.info.LatestRun != nil && match.info.LatestRun.Summary != nil {
						result := match.info.LatestRun.Summary.Result
						entry.OrcaResult = &result
						if match.info.LatestRun.Summary.IssueID != "" {
							issue := match.info.LatestRun.Summary.IssueID
							entry.OrcaIssue = &issue
						}
					}
					break
				}
			}
		}

		entries = append(entries, entry)
	}
	return entries
}

func printTable(entries []SessionEntry) {
	if len(entries) == 0 {
		fmt.Println("(no tmux sessions)")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tTYPE\tWINDOWS\tAGE\tRESULT\tISSUE")
	for _, e := range entries {
		result := ""
		issue := ""
		if e.OrcaResult != nil {
			result = *e.OrcaResult
		}
		if e.OrcaIssue != nil {
			issue = *e.OrcaIssue
		}
		attached := ""
		if e.Attached {
			attached = " *"
		}
		fmt.Fprintf(w, "%s%s\t%s\t%d\t%s\t%s\t%s\n",
			e.Name, attached, e.Type, e.Windows, e.CreatedAge, result, issue)
	}
	w.Flush()
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
