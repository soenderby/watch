package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// StatusSummary is the machine-readable status output.
type StatusSummary struct {
	Agents    int `json:"agents"`
	Instances int `json:"instances"`
}

func runStatus(args []string) error {
	outputJSON := false
	for _, arg := range args {
		switch arg {
		case "--json":
			outputJSON = true
		case "--help", "-h":
			fmt.Println("Usage: watch status [--json]")
			fmt.Println("\nOne-line summary of agent state.")
			return nil
		default:
			return fmt.Errorf("unknown option: %s", arg)
		}
	}

	result, err := singlePoll()
	if err != nil {
		return err
	}

	snap := result.Snapshot
	totalInstances := 0
	for _, agent := range snap.Agents {
		totalInstances += len(agent.Instances)
	}

	summary := StatusSummary{
		Agents:    len(snap.Agents),
		Instances: totalInstances,
	}

	if outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}

	fmt.Printf("%d agents, %d instances\n", summary.Agents, summary.Instances)
	return nil
}
