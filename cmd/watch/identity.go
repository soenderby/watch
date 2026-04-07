package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/soenderby/watch/internal/config"
	"github.com/soenderby/watch/internal/identity"
	"github.com/soenderby/watch/internal/identityflow"
	"github.com/soenderby/watch/internal/model"
	"github.com/soenderby/watch/internal/tmux"
)

type adoptOptions struct {
	SessionName    string
	Name           string
	Project        string
	Global         bool
	Description    string
	SessionPattern string
	PathPrefix     string
	File           string
	DryRun         bool
	Yes            bool
}

type resolvedAdopt struct {
	Identity   identity.AgentIdentity
	TargetFile string
	Project    *config.Project
}

var errHelpRequested = errors.New("help requested")

func runIdentity(args []string) error {
	if len(args) == 0 {
		printIdentityUsage()
		return nil
	}

	switch args[0] {
	case "discover":
		return runIdentityDiscover(args[1:])
	case "adopt":
		return runIdentityAdopt(args[1:])
	case "--help", "-h", "help":
		printIdentityUsage()
		return nil
	default:
		return fmt.Errorf("unknown identity subcommand: %s", args[0])
	}
}

func runIdentityDiscover(args []string) error {
	outputJSON := false
	includeAll := false
	projectFilter := ""

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			outputJSON = true
		case "--all":
			includeAll = true
		case "--project":
			if i+1 >= len(args) {
				return fmt.Errorf("--project requires a value")
			}
			i++
			projectFilter = args[i]
		case "--help", "-h":
			printIdentityDiscoverUsage()
			return nil
		default:
			return fmt.Errorf("unknown option: %s", arg)
		}
	}

	cfgPath, err := configPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	result, err := singlePoll()
	if err != nil {
		return err
	}

	sessions, err := tmux.ListSessions()
	if err != nil {
		return err
	}

	matched := matchedSessionNames(result.Snapshot)
	candidates := identityflow.DiscoverCandidates(sessions, matched, cfg.Projects, projectFilter, includeAll)

	if outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(candidates)
	}

	if len(candidates) == 0 {
		fmt.Println("(no unmatched candidate sessions)")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "SESSION\tPROJECT\tCONFIDENCE\tREASON\tAGE\tPATH")
	for _, c := range candidates {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			c.SessionName, c.InferredProject, c.Confidence, c.Reason, c.ActivityAge, c.Path)
	}
	w.Flush()
	fmt.Println()
	fmt.Println("adopt one with: watch identity adopt <session-name>")
	return nil
}

func runIdentityAdopt(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: watch identity adopt <session-name> [options]")
	}
	if args[0] == "--help" || args[0] == "-h" {
		printIdentityAdoptUsage()
		return nil
	}

	opts, err := parseAdoptOptions(args)
	if errors.Is(err, errHelpRequested) {
		return nil
	}
	if err != nil {
		return err
	}

	cfgPath, err := configPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	globalPath, err := identity.DefaultGlobalPath()
	if err != nil {
		return err
	}
	reg, err := identity.BuildRegistry(globalPath, identitySourcesFromConfig(cfg))
	if err != nil {
		return err
	}

	sessions, err := tmux.ListSessions()
	if err != nil {
		return err
	}
	session, ok := identityflow.FindSessionByName(sessions, opts.SessionName)
	if !ok {
		return fmt.Errorf("session %q not found", opts.SessionName)
	}

	inferredProject, _ := identityflow.InferProjectForPath(session.Path, cfg.Projects)
	reader := bufio.NewReader(os.Stdin)
	resolved, err := resolveAdoptIdentity(opts, reader, session, inferredProject, cfg)
	if err != nil {
		return err
	}

	if reg.ByName(resolved.Identity.Name) != nil {
		return fmt.Errorf("identity %q already exists", resolved.Identity.Name)
	}
	if err := identityflow.EnsureNameNotInFile(resolved.TargetFile, resolved.Identity.Name); err != nil {
		return err
	}

	matches := identityflow.PreviewMatches(resolved.Identity, sessions, resolved.Project)
	if err := identityflow.ValidateIdentityForAdopt(resolved.Identity, resolved.Project != nil); err != nil {
		return err
	}
	if len(matches) == 0 {
		fmt.Fprintln(os.Stderr, "warning: identity currently matches 0 sessions")
	}
	if len(matches) > 3 {
		fmt.Fprintf(os.Stderr, "warning: identity currently matches %d sessions\n", len(matches))
	}

	if err := printAdoptPreview(resolved.TargetFile, resolved.Identity, matches); err != nil {
		return err
	}

	if opts.DryRun {
		return nil
	}

	if !opts.Yes {
		ok, err := promptYesNo(reader, "Write changes?", false)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("aborted")
			return nil
		}
	}

	if err := identityflow.AppendIdentityToFile(resolved.TargetFile, resolved.Identity); err != nil {
		return err
	}

	fmt.Printf("added identity %q in %s\n", resolved.Identity.Name, resolved.TargetFile)
	return nil
}

func resolveAdoptIdentity(opts adoptOptions, reader *bufio.Reader, session tmux.Session, inferredProject string, cfg *config.Config) (*resolvedAdopt, error) {
	if opts.Global && opts.Project != "" {
		return nil, fmt.Errorf("--global and --project cannot be used together")
	}

	selectedProject := opts.Project
	if opts.Global {
		selectedProject = ""
	}
	if selectedProject == "" && !opts.Global {
		if inferredProject != "" {
			selectedProject = inferredProject
		} else if !opts.Yes {
			scope, err := prompt(reader, "Scope [project/global]", "global")
			if err != nil {
				return nil, err
			}
			scope = strings.ToLower(strings.TrimSpace(scope))
			if scope == "project" {
				selectedProject, err = prompt(reader, "Project name", "")
				if err != nil {
					return nil, err
				}
			}
		}
	}

	var selectedProjectConfig *config.Project
	if selectedProject != "" {
		selectedProjectConfig = cfg.FindProject(selectedProject)
		if selectedProjectConfig == nil {
			return nil, fmt.Errorf("project %q is not registered", selectedProject)
		}
	}

	name := opts.Name
	if name == "" {
		if opts.Yes {
			return nil, fmt.Errorf("--name is required with --yes")
		}
		var err error
		name, err = prompt(reader, "Identity name", "")
		if err != nil {
			return nil, err
		}
		if name == "" {
			return nil, fmt.Errorf("identity name cannot be empty")
		}
	}

	description := opts.Description
	if description == "" && !opts.Yes {
		var err error
		description, err = prompt(reader, "Description (optional)", "")
		if err != nil {
			return nil, err
		}
	}

	suggestedPattern := identityflow.SuggestSessionPattern(session.Name)
	suggestedPrefix := identityflow.SuggestPathPrefix(session.Path, selectedProjectConfig)

	sessionPattern := opts.SessionPattern
	if sessionPattern == "" {
		if opts.Yes {
			sessionPattern = suggestedPattern
		} else {
			var err error
			sessionPattern, err = prompt(reader, "Session pattern", suggestedPattern)
			if err != nil {
				return nil, err
			}
		}
	}

	pathPrefix := opts.PathPrefix
	if pathPrefix == "" {
		if opts.Yes {
			pathPrefix = suggestedPrefix
		} else {
			var err error
			pathPrefix, err = prompt(reader, "Path prefix", suggestedPrefix)
			if err != nil {
				return nil, err
			}
		}
	}

	sessionPattern = strings.TrimSpace(sessionPattern)
	pathPrefix = strings.TrimSpace(pathPrefix)
	if pathPrefix != "" {
		pathPrefix = filepath.Clean(pathPrefix)
	}

	targetFile := opts.File
	isProjectLocalTarget := false
	if targetFile == "" {
		if selectedProjectConfig != nil {
			targetFile = identity.ProjectLocalPath(selectedProjectConfig.Path)
			isProjectLocalTarget = true
		} else {
			globalPath, err := identity.DefaultGlobalPath()
			if err != nil {
				return nil, err
			}
			targetFile = globalPath
		}
	}

	agentProject := selectedProject
	if isProjectLocalTarget {
		agentProject = ""
	}

	agent := identity.AgentIdentity{
		Name:        name,
		Project:     agentProject,
		Description: description,
	}
	if sessionPattern != "" || pathPrefix != "" {
		agent.Match = &identity.MatchRules{SessionPattern: sessionPattern, PathPrefix: pathPrefix}
	}

	return &resolvedAdopt{Identity: agent, TargetFile: targetFile, Project: selectedProjectConfig}, nil
}

func parseAdoptOptions(args []string) (adoptOptions, error) {
	var opts adoptOptions
	if len(args) == 0 {
		return opts, nil
	}

	opts.SessionName = args[0]
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--name":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--name requires a value")
			}
			i++
			opts.Name = args[i]
		case "--project":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--project requires a value")
			}
			i++
			opts.Project = args[i]
		case "--global":
			opts.Global = true
		case "--description":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--description requires a value")
			}
			i++
			opts.Description = args[i]
		case "--session-pattern":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--session-pattern requires a value")
			}
			i++
			opts.SessionPattern = args[i]
		case "--path-prefix":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--path-prefix requires a value")
			}
			i++
			opts.PathPrefix = args[i]
		case "--file":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--file requires a value")
			}
			i++
			opts.File = args[i]
		case "--dry-run":
			opts.DryRun = true
		case "--yes":
			opts.Yes = true
		case "--help", "-h":
			printIdentityAdoptUsage()
			return adoptOptions{}, errHelpRequested
		default:
			return opts, fmt.Errorf("unknown option: %s", arg)
		}
	}

	return opts, nil
}

func matchedSessionNames(snap *model.Snapshot) map[string]bool {
	matched := map[string]bool{}
	if snap == nil {
		return matched
	}
	for _, a := range snap.Agents {
		for _, inst := range a.Instances {
			matched[inst.SessionName] = true
		}
	}
	return matched
}

func identitySourcesFromConfig(cfg *config.Config) []identity.ProjectSource {
	var sources []identity.ProjectSource
	for _, p := range cfg.Projects {
		sources = append(sources, identity.ProjectSource{Name: p.Name, Path: p.Path})
	}
	return sources
}

func printAdoptPreview(path string, id identity.AgentIdentity, matched []string) error {
	fmt.Printf("Target file: %s\n", path)
	data, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println("Identity:")
	fmt.Println(string(data))

	fmt.Printf("Would currently match %d session(s)", len(matched))
	if len(matched) > 0 {
		fmt.Printf(": %s", strings.Join(matched, ", "))
	}
	fmt.Println()
	return nil
}

func prompt(reader *bufio.Reader, label, def string) (string, error) {
	if def != "" {
		fmt.Printf("%s [%s]: ", label, def)
	} else {
		fmt.Printf("%s: ", label)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def, nil
	}
	return line, nil
}

func promptYesNo(reader *bufio.Reader, label string, def bool) (bool, error) {
	defLabel := "y/N"
	if def {
		defLabel = "Y/n"
	}
	answer, err := prompt(reader, label+" ["+defLabel+"]", "")
	if err != nil {
		return false, err
	}
	if answer == "" {
		return def, nil
	}
	answer = strings.ToLower(answer)
	return answer == "y" || answer == "yes", nil
}

func printIdentityUsage() {
	fmt.Println(`Usage: watch identity <subcommand>

Subcommands:
  discover [--json] [--project <name>] [--all]
      Show unmatched tmux sessions that could be agent sessions.

  adopt <session-name> [options]
      Create an identity from a session and append it to agents.json.

  help
      Print this help`)
}

func printIdentityDiscoverUsage() {
	fmt.Println(`Usage: watch identity discover [--json] [--project <name>] [--all]

Options:
  --json             Machine-readable output
  --project <name>   Filter by inferred project
  --all              Include low-confidence candidates`)
}

func printIdentityAdoptUsage() {
	fmt.Println(`Usage: watch identity adopt <session-name> [options]

Options:
  --name <name>                  Identity name
  --project <name>               Associate with project
  --global                       Create a global identity
  --description <text>           Optional description
  --session-pattern <glob>       Match rule on tmux session name
  --path-prefix <path>           Match rule on working directory
  --file <path>                  Override target agents.json path
  --dry-run                      Show preview only, do not write
  --yes                          Skip confirmation prompts`)
}
