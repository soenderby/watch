package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/soenderby/watch/internal/orca"
	"github.com/soenderby/watch/internal/tmux"
)

// StatusSummary is the machine-readable status output.
type StatusSummary struct {
	TotalSessions int `json:"total_sessions"`
	OrcaSessions  int `json:"orca_sessions"`
	OtherSessions int `json:"other_sessions"`
	Attached      int `json:"attached"`
}

func runStatus(args []string) error {
	outputJSON := false
	for _, arg := range args {
		switch arg {
		case "--json":
			outputJSON = true
		case "--help", "-h":
			fmt.Println("Usage: watch status [--json]")
			fmt.Println("\nOne-line summary of tmux session state.")
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

	summary := StatusSummary{
		TotalSessions: len(sessions),
	}

	for _, s := range sessions {
		if orca.IsOrcaSession(s.Name, "") {
			summary.OrcaSessions++
		} else {
			summary.OtherSessions++
		}
		if s.Attached {
			summary.Attached++
		}
	}

	if outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}

	fmt.Printf("%d sessions (%d orca, %d other)\n", summary.TotalSessions, summary.OrcaSessions, summary.OtherSessions)
	return nil
}
