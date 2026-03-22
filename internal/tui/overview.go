package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/soenderby/watch/internal/events"
	"github.com/soenderby/watch/internal/model"
)

// overviewView is the Level 0 view showing all agents.
type overviewView struct {
	cursor int
}

func newOverview() *overviewView {
	return &overviewView{}
}

func (v *overviewView) title() string { return "" }

func (v *overviewView) update(msg tea.Msg, snap *model.Snapshot, store *events.Store) (view, action) {
	if snap == nil {
		return v, actionNone
	}

	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return v, actionNone
	}

	agentCount := len(snap.Agents)
	if agentCount == 0 {
		return v, actionNone
	}

	switch km.String() {
	case "j", "down":
		if v.cursor < agentCount-1 {
			v.cursor++
		}
	case "k", "up":
		if v.cursor > 0 {
			v.cursor--
		}
	case "enter":
		if v.cursor < agentCount {
			agent := snap.Agents[v.cursor]
			return newAgentDetail(agent.Name), actionPush
		}
	case "g":
		if v.cursor < agentCount {
			agent := snap.Agents[v.cursor]
			if len(agent.Instances) == 1 {
				return &jumpTarget{sessionName: agent.Instances[0].SessionName}, actionJump
			}
			// Multiple instances: must drill in first.
		}
	}

	// Clamp cursor.
	if v.cursor >= agentCount {
		v.cursor = agentCount - 1
	}

	return v, actionNone
}

func (v *overviewView) render(snap *model.Snapshot, store *events.Store, width, height int) string {
	if snap == nil || len(snap.Agents) == 0 {
		return "(no agents with active instances)"
	}

	var lines []string
	for i, agent := range snap.Agents {
		cursor := "  "
		if i == v.cursor {
			cursor = "> "
		}

		// Agent header line.
		header := cursor + agent.Name
		if agent.Project != "" {
			header += " (" + agent.Project + ")"
		}

		// Right-aligned queue info for orca projects.
		right := ""
		if agent.Project != "" && snap != nil {
			proj := snap.ProjectByName(agent.Project)
			if proj != nil && proj.Queue.Available {
				right = fmt.Sprintf("%d ready", proj.Queue.Ready)
			}
		}
		if right != "" {
			pad := width - len(header) - len(right)
			if pad > 0 {
				header += spaces(pad) + right
			}
		}

		lines = append(lines, header)

		// Inline instance summaries.
		instanceLines := renderInstanceSummaries(agent, width)
		lines = append(lines, instanceLines...)

		// Blank line between agents.
		if i < len(snap.Agents)-1 {
			lines = append(lines, "")
		}
	}

	return strings.Join(lines, "\n")
}

func renderInstanceSummaries(agent *model.Agent, width int) []string {
	const maxInlineInstances = 4

	instances := agent.Instances
	overflow := 0
	if len(instances) > maxInlineInstances {
		overflow = len(instances) - maxInlineInstances
		instances = instances[:maxInlineInstances]
	}

	// Try to pack two instances per line for density.
	var parts []string
	for _, inst := range instances {
		parts = append(parts, formatInstanceShort(inst))
	}

	var lines []string
	// Pack two per line if they fit.
	for i := 0; i < len(parts); i += 2 {
		line := "    " + parts[i]
		if i+1 < len(parts) {
			combined := line + "     " + parts[i+1]
			if len(combined) <= width {
				line = combined
			} else {
				lines = append(lines, line)
				line = "    " + parts[i+1]
			}
		}
		lines = append(lines, line)
	}

	if overflow > 0 {
		lines = append(lines, fmt.Sprintf("    +%d more", overflow))
	}

	return lines
}

func formatInstanceShort(inst *model.Instance) string {
	// For orca instances: slot state issue duration
	if inst.Orca != nil {
		name := inst.Orca.AgentName
		if name == "" {
			name = inst.SessionName
		}
		issue := ""
		dur := ""
		if inst.Orca.CurrentRun != nil {
			issue = inst.Orca.CurrentRun.IssueID
			if inst.Orca.CurrentRun.Duration > 0 {
				dur = formatDuration(inst.Orca.CurrentRun.Duration)
			}
		}
		// Estimate duration from tmux created time if not available.
		if dur == "" {
			dur = formatDuration(time.Since(inst.Tmux.Created))
		}

		parts := []string{name, inst.State}
		if issue != "" {
			parts = append(parts, issue)
		}
		parts = append(parts, dur)
		return strings.Join(parts, "  ")
	}

	// For non-orca instances: name state age
	name := inst.SessionName
	age := formatDuration(time.Since(inst.Tmux.Created))
	return strings.Join([]string{name, inst.State, age}, "  ")
}

func formatDuration(d time.Duration) string {
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
