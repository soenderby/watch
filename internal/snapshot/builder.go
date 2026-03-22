// Package snapshot assembles a model.Snapshot from raw data sources.
package snapshot

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/soenderby/watch/internal/identity"
	"github.com/soenderby/watch/internal/model"
	"github.com/soenderby/watch/internal/orca"
)

// TmuxSession is the subset of tmux session data the builder needs.
// Using a struct rather than an interface so test data is easy to construct.
type TmuxSession struct {
	Name     string
	Windows  int
	Created  time.Time
	Attached bool
	Activity time.Time
	Path     string
}

// ProjectConfig describes a registered project.
type ProjectConfig struct {
	Name string
	Path string
}

// ProjectArtifacts holds the orca artifact data and queue state for one project.
type ProjectArtifacts struct {
	Sessions []orca.SessionInfo
	Queue    model.QueueState
}

// Input bundles all the raw data the builder needs to assemble a snapshot.
type Input struct {
	Timestamp    time.Time
	TmuxSessions []TmuxSession
	Registry     *identity.Registry
	Projects     []ProjectConfig
	Artifacts    map[string]ProjectArtifacts // keyed by project name
}

// Build assembles a Snapshot from the provided input.
func Build(in Input) *model.Snapshot {
	snap := &model.Snapshot{
		Timestamp: in.Timestamp,
	}

	if in.Registry == nil {
		return snap
	}

	// Build project list.
	for _, proj := range in.Projects {
		p := &model.Project{
			Name: proj.Name,
			Path: proj.Path,
		}
		if arts, ok := in.Artifacts[proj.Name]; ok {
			p.Queue = arts.Queue
		}
		snap.Projects = append(snap.Projects, p)
	}

	// Index artifact sessions by session ID for each project.
	type artifactIndex struct {
		projectName string
		session     orca.SessionInfo
	}
	artifactsBySessionID := make(map[string]artifactIndex)
	for _, proj := range in.Projects {
		if arts, ok := in.Artifacts[proj.Name]; ok {
			for _, s := range arts.Sessions {
				artifactsBySessionID[s.SessionID] = artifactIndex{
					projectName: proj.Name,
					session:     s,
				}
			}
		}
	}

	// Match tmux sessions to agent identities.
	agents := make(map[string]*agentAccum) // keyed by agent identity name

	for _, ts := range in.TmuxSessions {
		matched := false

		// Try orca matching first.
		if orca.IsOrcaSession(ts.Name, "") {
			sessionID := orca.ExtractSessionID(ts.Name, "")
			if ai, ok := artifactsBySessionID[sessionID]; ok {
				// Find the agent identity for this project.
				projAgents := in.Registry.ForProject(ai.projectName)
				if len(projAgents) > 0 {
					agentID := &projAgents[0] // first agent identity for this project
					inst := buildOrcaInstance(ts, ai.session, ai.projectName)
					ensureAgent(agents, agentID).instances = append(
						ensureAgent(agents, agentID).instances, inst,
					)
					matched = true
				}
			}
		}

		// Try non-orca matching by working directory.
		if !matched && ts.Path != "" {
			for _, proj := range in.Projects {
				if isPathWithin(ts.Path, proj.Path) {
					projAgents := in.Registry.ForProject(proj.Name)
					if len(projAgents) > 0 {
						agentID := &projAgents[0]
						// Don't double-match if this is actually an orca session
						// that just didn't have artifacts.
						if orca.IsOrcaSession(ts.Name, "") {
							continue
						}
						inst := buildNonOrcaInstance(ts)
						ensureAgent(agents, agentID).instances = append(
							ensureAgent(agents, agentID).instances, inst,
						)
						matched = true
						break
					}
				}
			}
		}
		// Unmatched sessions are ignored.
		_ = matched
	}

	// Build Agent objects from collected instances.
	for _, ai := range agents {
		agent := &model.Agent{
			Name:        ai.identity.Name,
			Project:     ai.identity.Project,
			Description: ai.identity.Description,
			Instances:   ai.instances,
		}

		// Order instances by most recently active.
		sortInstancesByActivity(agent.Instances)

		// Derive aggregate agent state.
		agent.State = deriveAgentState(agent.Instances)

		snap.Agents = append(snap.Agents, agent)
	}

	return snap
}

// agentAccum accumulates instances for one agent during snapshot building.
type agentAccum struct {
	identity  *identity.AgentIdentity
	instances []*model.Instance
}

func ensureAgent(agents map[string]*agentAccum, id *identity.AgentIdentity) *agentAccum {
	ai, ok := agents[id.Name]
	if !ok {
		ai = &agentAccum{identity: id}
		agents[id.Name] = ai
	}
	return ai
}

func buildOrcaInstance(ts TmuxSession, si orca.SessionInfo, projectName string) *model.Instance {
	inst := &model.Instance{
		SessionName: ts.Name,
		Tmux: model.TmuxState{
			Windows:  ts.Windows,
			Created:  ts.Created,
			Attached: ts.Attached,
			Activity: ts.Activity,
		},
		Orca: &model.OrcaRunState{
			AgentName: orca.ExtractAgentSlot(ts.Name, ""),
			SessionID: si.SessionID,
		},
	}

	// Convert orca runs to model runs (newest first).
	for i := len(si.Runs) - 1; i >= 0; i-- {
		r := si.Runs[i]
		mr := model.Run{
			RunID:       r.RunID,
			HasSummary:  r.HasSummary,
			LogPath:     r.LogPath,
			SummaryPath: r.SummaryPath,
		}
		if r.Summary != nil {
			mr.Result = r.Summary.Result
			mr.IssueID = r.Summary.IssueID
			mr.Merged = r.Summary.Merged
			mr.Notes = r.Summary.Notes
		}
		inst.Orca.Runs = append(inst.Orca.Runs, mr)
	}

	if len(inst.Orca.Runs) > 0 {
		first := inst.Orca.Runs[0]
		inst.Orca.CurrentRun = &first
	}

	// Derive instance state for orca.
	tmuxAlive := true // it's in the tmux session list, so it's alive
	inst.State = deriveOrcaInstanceState(tmuxAlive, inst.Orca.CurrentRun)

	return inst
}

func buildNonOrcaInstance(ts TmuxSession) *model.Instance {
	inst := &model.Instance{
		SessionName: ts.Name,
		Tmux: model.TmuxState{
			Windows:  ts.Windows,
			Created:  ts.Created,
			Attached: ts.Attached,
			Activity: ts.Activity,
		},
	}
	inst.State = model.InstanceStateActive
	return inst
}

func deriveOrcaInstanceState(tmuxAlive bool, currentRun *model.Run) string {
	if currentRun == nil {
		if tmuxAlive {
			return model.InstanceStateRunning
		}
		return model.InstanceStateIdle
	}

	if !currentRun.HasSummary {
		if tmuxAlive {
			return model.InstanceStateRunning
		}
		return model.InstanceStateIdle
	}

	switch currentRun.Result {
	case model.RunResultFailed:
		return model.InstanceStateFailed
	case model.RunResultBlocked:
		return model.InstanceStateBlocked
	case model.RunResultCompleted, model.RunResultNoWork:
		if tmuxAlive {
			return model.InstanceStateRunning
		}
		return model.InstanceStateDone
	default:
		if tmuxAlive {
			return model.InstanceStateRunning
		}
		return model.InstanceStateIdle
	}
}

func deriveAgentState(instances []*model.Instance) string {
	if len(instances) == 0 {
		return model.AgentStateIdle
	}

	hasRunning := false
	hasFailed := false
	hasBlocked := false
	allDone := true

	for _, inst := range instances {
		switch inst.State {
		case model.InstanceStateRunning, model.InstanceStateActive:
			hasRunning = true
			allDone = false
		case model.InstanceStateFailed:
			hasFailed = true
			allDone = false
		case model.InstanceStateBlocked:
			hasBlocked = true
			allDone = false
		case model.InstanceStateDone:
			// allDone stays true
		case model.InstanceStateIdle:
			allDone = false
		default:
			allDone = false
		}
	}

	if hasRunning {
		return model.AgentStateRunning
	}
	if allDone {
		return model.AgentStateDone
	}
	if hasFailed {
		return model.AgentStateFailed
	}
	if hasBlocked {
		return model.AgentStateBlocked
	}
	return model.AgentStateIdle
}

func sortInstancesByActivity(instances []*model.Instance) {
	for i := 0; i < len(instances); i++ {
		for j := i + 1; j < len(instances); j++ {
			if instances[j].Tmux.Activity.After(instances[i].Tmux.Activity) {
				instances[i], instances[j] = instances[j], instances[i]
			}
		}
	}
}

func isPathWithin(child, parent string) bool {
	child = filepath.Clean(child)
	parent = filepath.Clean(parent)
	if child == parent {
		return true
	}
	return strings.HasPrefix(child, parent+string(filepath.Separator))
}
