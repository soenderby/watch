// Package model defines the core types for watch's agent-centric data model.
//
// A Snapshot is a point-in-time view of all known agents and their instances.
// Agents are persistent identities. Instances are tmux sessions associated
// with an agent. This package has no dependencies on any other internal package.
package model

import "time"

// Snapshot is a point-in-time view of everything watch knows about.
// It contains only agents that have at least one active instance.
type Snapshot struct {
	Timestamp time.Time  `json:"timestamp"`
	Agents    []*Agent   `json:"agents"`
	Projects  []*Project `json:"projects"`
}

// AgentByName returns the agent with the given identity name, or nil.
func (s *Snapshot) AgentByName(name string) *Agent {
	for _, a := range s.Agents {
		if a.Name == name {
			return a
		}
	}
	return nil
}

// AgentsForProject returns all agents associated with the given project.
func (s *Snapshot) AgentsForProject(project string) []*Agent {
	var result []*Agent
	for _, a := range s.Agents {
		if a.Project == project {
			result = append(result, a)
		}
	}
	return result
}

// ProjectByName returns the project with the given name, or nil.
func (s *Snapshot) ProjectByName(name string) *Project {
	for _, p := range s.Projects {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// Agent is an agent as observed at snapshot time. It combines identity
// (who it is) with operational state (what it is doing).
type Agent struct {
	// Identity fields.
	Name        string `json:"name"`        // unique identifier
	Project     string `json:"project"`     // optional project association
	Description string `json:"description"` // short human-readable description

	// Operational state.
	Instances []*Instance `json:"instances"` // active tmux sessions, most recently active first
	State     string      `json:"state"`     // derived aggregate state
}

// Instance is one tmux session associated with an agent.
type Instance struct {
	SessionName string        `json:"session_name"` // tmux session name
	Tmux        TmuxState     `json:"tmux"`
	State       string        `json:"state"` // instance-level state
	Orca        *OrcaRunState `json:"orca,omitempty"`
}

// TmuxState holds raw tmux data for a session.
type TmuxState struct {
	Windows  int       `json:"windows"`
	Created  time.Time `json:"created"`
	Attached bool      `json:"attached"`
	Activity time.Time `json:"activity"`
}

// OrcaRunState holds orca-specific enrichment for an instance.
type OrcaRunState struct {
	AgentName  string `json:"agent_name"`  // orca slot name, e.g. "agent-1"
	SessionID  string `json:"session_id"`  // orca session ID from artifact directory
	CurrentRun *Run   `json:"current_run"` // latest/active run
	Runs       []Run  `json:"runs"`        // all runs, newest first
}

// Run is one agent run within an orca session.
type Run struct {
	RunID       string        `json:"run_id"`
	Result      string        `json:"result"`       // "completed", "blocked", "no_work", "failed", ""
	IssueID     string        `json:"issue_id"`     // from summary.json
	Merged      bool          `json:"merged"`       // from summary.json
	Duration    time.Duration `json:"duration"`     // run duration
	Tokens      int           `json:"tokens"`       // token count, 0 if unavailable
	HasSummary  bool          `json:"has_summary"`  // whether summary.json was parseable
	Notes       string        `json:"notes"`        // from summary.json
	LogPath     string        `json:"log_path"`     // absolute path to run.log
	SummaryPath string        `json:"summary_path"` // absolute path to summary.json
}

// Project is a registered orca project.
type Project struct {
	Name  string     `json:"name"`
	Path  string     `json:"path"`
	Queue QueueState `json:"queue"`
}

// QueueState summarizes a project's br queue.
type QueueState struct {
	Ready      int  `json:"ready"`
	InProgress int  `json:"in_progress"`
	Available  bool `json:"available"` // false when queue state could not be read
}

// Event represents something that happened to an agent or instance.
type Event struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`         // event type
	AgentName   string    `json:"agent_name"`   // agent identity name
	ProjectName string    `json:"project_name"` // agent's project, empty for global agents
	SessionName string    `json:"session_name"` // tmux session name
	RunID       string    `json:"run_id"`       // empty for non-run events
	IssueID     string    `json:"issue_id"`     // empty when not applicable
	Result      string    `json:"result"`       // empty for non-completion events
	Merged      bool      `json:"merged"`
}

// Event type constants.
const (
	EventInstanceUp   = "instance_up"
	EventInstanceDown = "instance_down"
	EventRunStarted   = "run_started"
	EventRunCompleted = "run_completed"
)

// Agent state constants.
const (
	AgentStateRunning = "running"
	AgentStateDone    = "done"
	AgentStateFailed  = "failed"
	AgentStateBlocked = "blocked"
	AgentStateIdle    = "idle"
)

// Instance state constants (orca).
const (
	InstanceStateRunning = "running"
	InstanceStateDone    = "done"
	InstanceStateFailed  = "failed"
	InstanceStateBlocked = "blocked"
	InstanceStateIdle    = "idle"
)

// Instance state constants (non-orca).
const (
	InstanceStateActive = "active"
)

// Run result constants.
const (
	RunResultCompleted = "completed"
	RunResultBlocked   = "blocked"
	RunResultNoWork    = "no_work"
	RunResultFailed    = "failed"
)
