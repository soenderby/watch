package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/soenderby/watch/internal/events"
	"github.com/soenderby/watch/internal/model"
)

// agentDetailView is the Level 1 view showing one agent's instances and events.
type agentDetailView struct {
	agentName string
	cursor    int
}

func newAgentDetail(agentName string) *agentDetailView {
	return &agentDetailView{agentName: agentName}
}

func (v *agentDetailView) title() string { return v.agentName }

func (v *agentDetailView) update(msg tea.Msg, snap *model.Snapshot, store *events.Store) (view, action) {
	if snap == nil {
		return v, actionNone
	}

	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return v, actionNone
	}

	agent := snap.AgentByName(v.agentName)
	if agent == nil {
		return v, actionNone
	}

	instCount := len(agent.Instances)
	if instCount == 0 {
		return v, actionNone
	}

	switch km.String() {
	case "j", "down":
		if v.cursor < instCount-1 {
			v.cursor++
		}
	case "k", "up":
		if v.cursor > 0 {
			v.cursor--
		}
	case "enter":
		if v.cursor < instCount {
			inst := agent.Instances[v.cursor]
			if inst.Orca != nil {
				slotName := ""
				if inst.Orca.AgentName != "" {
					slotName = inst.Orca.AgentName
				}
				return newInstanceDetail(v.agentName, inst.SessionName, slotName), actionPush
			}
		}
	case "g":
		if v.cursor < instCount {
			inst := agent.Instances[v.cursor]
			return &jumpTarget{sessionName: inst.SessionName}, actionJump
		}
	}

	if v.cursor >= instCount {
		v.cursor = instCount - 1
	}

	return v, actionNone
}

func (v *agentDetailView) render(snap *model.Snapshot, store *events.Store, width, height int) string {
	if snap == nil {
		return ""
	}

	agent := snap.AgentByName(v.agentName)
	if agent == nil {
		return fmt.Sprintf("agent %q not found", v.agentName)
	}

	var lines []string

	// Project info.
	if agent.Project != "" {
		proj := snap.ProjectByName(agent.Project)
		if proj != nil {
			info := "  " + proj.Path
			if proj.Queue.Available {
				info += fmt.Sprintf("                          %d ready  %d in progress", proj.Queue.Ready, proj.Queue.InProgress)
			}
			lines = append(lines, info)
			lines = append(lines, "")
		}
	}

	// Instance list.
	for i, inst := range agent.Instances {
		cursor := "  "
		if i == v.cursor {
			cursor = "> "
		}
		lines = append(lines, cursor+formatInstanceDetail(inst))
	}

	// Events section.
	if store != nil {
		agentEvents := store.ForAgent(v.agentName)
		if len(agentEvents) > 0 {
			lines = append(lines, "")
			lines = append(lines, "  events")

			limit := 10
			if len(agentEvents) < limit {
				limit = len(agentEvents)
			}
			for _, e := range agentEvents[:limit] {
				lines = append(lines, "    "+formatEvent(e))
			}
		}
	}

	return strings.Join(lines, "\n")
}

func formatInstanceDetail(inst *model.Instance) string {
	if inst.Orca != nil {
		name := inst.Orca.AgentName
		if name == "" {
			name = inst.SessionName
		}
		parts := []string{name, inst.State}

		if inst.Orca.CurrentRun != nil {
			run := inst.Orca.CurrentRun
			if run.IssueID != "" {
				parts = append(parts, run.IssueID)
			}
			if run.Notes != "" {
				// Truncate long notes.
				notes := run.Notes
				if len(notes) > 50 {
					notes = notes[:47] + "..."
				}
				parts = append(parts, `"`+notes+`"`)
			}
		}

		dur := formatDuration(time.Since(inst.Tmux.Created))
		parts = append(parts, dur)
		return strings.Join(parts, "  ")
	}

	// Non-orca.
	parts := []string{
		inst.SessionName,
		inst.State,
		formatDuration(time.Since(inst.Tmux.Created)),
	}
	if inst.Tmux.Windows > 1 {
		parts = append(parts, fmt.Sprintf("windows: %d", inst.Tmux.Windows))
	}
	return strings.Join(parts, "  ")
}

func formatEvent(e model.Event) string {
	ts := e.Timestamp.Format("15:04")
	parts := []string{ts}

	if e.AgentName != "" {
		parts = append(parts, e.AgentName)
	}

	switch e.Type {
	case model.EventInstanceUp:
		parts = append(parts, "instance up")
	case model.EventInstanceDown:
		parts = append(parts, "instance down")
	case model.EventRunStarted:
		parts = append(parts, "started")
		if e.IssueID != "" {
			parts = append(parts, e.IssueID)
		}
	case model.EventRunCompleted:
		parts = append(parts, e.Result)
		if e.IssueID != "" {
			parts = append(parts, e.IssueID)
		}
		if e.Merged {
			parts = append(parts, "merged")
		}
	default:
		parts = append(parts, e.Type)
	}

	return strings.Join(parts, "  ")
}
