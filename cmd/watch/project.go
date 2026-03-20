package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/soenderby/watch/internal/config"
)

func runProject(args []string) error {
	if len(args) == 0 {
		return runProjectList(nil)
	}

	switch args[0] {
	case "add":
		return runProjectAdd(args[1:])
	case "remove":
		return runProjectRemove(args[1:])
	case "list":
		return runProjectList(args[1:])
	case "--help", "-h":
		printProjectUsage()
		return nil
	default:
		return fmt.Errorf("unknown project subcommand: %s", args[0])
	}
}

func runProjectAdd(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: watch project add <name> <path>")
	}
	name := args[0]
	path := args[1]

	cfgPath, err := config.DefaultPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	if err := cfg.AddProject(name, path); err != nil {
		return err
	}

	if err := config.Save(cfg, cfgPath); err != nil {
		return err
	}

	fmt.Printf("added project %q at %s\n", name, path)
	return nil
}

func runProjectRemove(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: watch project remove <name>")
	}
	name := args[0]

	cfgPath, err := config.DefaultPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	if err := cfg.RemoveProject(name); err != nil {
		return err
	}

	if err := config.Save(cfg, cfgPath); err != nil {
		return err
	}

	fmt.Printf("removed project %q\n", name)
	return nil
}

func runProjectList(args []string) error {
	outputJSON := false
	for _, arg := range args {
		switch arg {
		case "--json":
			outputJSON = true
		default:
			return fmt.Errorf("unknown option: %s", arg)
		}
	}

	cfgPath, err := config.DefaultPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	if outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cfg.Projects)
	}

	if len(cfg.Projects) == 0 {
		fmt.Println("(no projects registered)")
		fmt.Println("Use: watch project add <name> <path>")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tPATH")
	for _, p := range cfg.Projects {
		fmt.Fprintf(w, "%s\t%s\n", p.Name, p.Path)
	}
	w.Flush()
	return nil
}

func printProjectUsage() {
	fmt.Println(`Usage: watch project <subcommand>

Subcommands:
  add <name> <path>    Register a project
  remove <name>        Unregister a project
  list [--json]        List registered projects`)
}
