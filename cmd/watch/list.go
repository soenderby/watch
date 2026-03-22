package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/soenderby/watch/internal/events"
	"github.com/soenderby/watch/internal/poller"
)

func runList(args []string) error {
	outputJSON := false
	for _, arg := range args {
		switch arg {
		case "--json":
			outputJSON = true
		case "--help", "-h":
			fmt.Println("Usage: watch list [--json]")
			fmt.Println("\nList all agents with their active instances.")
			return nil
		default:
			return fmt.Errorf("unknown option: %s", arg)
		}
	}

	result, err := singlePoll()
	if err != nil {
		return err
	}

	if outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result.Snapshot)
	}

	snap := result.Snapshot
	if len(snap.Agents) == 0 {
		fmt.Println("(no agents with active instances)")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "AGENT\tPROJECT\tSTATE\tINSTANCES\tISSUE\tAGE")
	for _, agent := range snap.Agents {
		issue := ""
		age := ""
		if len(agent.Instances) > 0 {
			inst := agent.Instances[0]
			age = formatAge(time.Since(inst.Tmux.Created))
			if inst.Orca != nil && inst.Orca.CurrentRun != nil {
				issue = inst.Orca.CurrentRun.IssueID
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
			agent.Name, agent.Project, agent.State,
			len(agent.Instances), issue, age)
	}
	w.Flush()
	return nil
}

func singlePoll() (*poller.Result, error) {
	cfgPath, err := configPath()
	if err != nil {
		return nil, err
	}

	store := events.NewStore(50)
	p := poller.New(cfgPath, store)
	return p.Poll()
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
