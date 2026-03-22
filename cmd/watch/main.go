package main

import (
	"fmt"
	"os"

	"github.com/soenderby/watch/internal/config"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) < 2 {
		// No subcommand: will be TUI mode in the future.
		fmt.Println("watch: TUI mode not yet implemented. Use a subcommand.")
		fmt.Println()
		printUsage()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "list":
		if err := runList(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "watch list: %v\n", err)
			os.Exit(1)
		}
	case "status":
		if err := runStatus(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "watch status: %v\n", err)
			os.Exit(1)
		}
	case "project":
		if err := runProject(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "watch project: %v\n", err)
			os.Exit(1)
		}
	case "version":
		fmt.Printf("watch %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "watch: unknown command %q\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Usage: watch [command]

Commands:
  list          List agents with active instances
  status        One-line summary of agent state
  project       Manage registered projects
  version       Print version
  help          Print this help

When run with no command, enters TUI mode (not yet implemented).

Options:
  --json        Machine-readable output (list, status, project list)`)
}

// configPath returns the path to the watch config file.
func configPath() (string, error) {
	return config.DefaultPath()
}
