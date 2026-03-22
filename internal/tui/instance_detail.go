package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/soenderby/watch/internal/events"
	"github.com/soenderby/watch/internal/model"
)

// instanceDetailView is the Level 2 view showing one orca instance's runs and events.
type instanceDetailView struct {
	agentName   string
	sessionName string
	cursor      int
}

func newInstanceDetail(agentName, sessionName string) *instanceDetailView {
	return &instanceDetailView{
		agentName:   agentName,
		sessionName: sessionName,
	}
}

func (v *instanceDetailView) title() string {
	// Extract a short name from the session.
	agent := v.agentName
	// Try to show the orca slot name.
	return agent
}

func (v *instanceDetailView) update(msg tea.Msg, snap *model.Snapshot, store *events.Store) (view, action) {
	if snap == nil {
		return v, actionNone
	}

	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return v, actionNone
	}

	inst := v.findInstance(snap)
	if inst == nil || inst.Orca == nil {
		return v, actionNone
	}

	runCount := len(inst.Orca.Runs)

	switch km.String() {
	case "j", "down":
		if v.cursor < runCount-1 {
			v.cursor++
		}
	case "k", "up":
		if v.cursor > 0 {
			v.cursor--
		}
	case "g":
		return &jumpTarget{sessionName: v.sessionName}, actionJump
	case "l":
		// Log pager — show the selected run's log.
		if v.cursor < runCount {
			run := inst.Orca.Runs[v.cursor]
			if run.LogPath != "" {
				return newLogPager(run.LogPath), actionPush
			}
		}
	}

	if v.cursor >= runCount {
		v.cursor = runCount - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}

	return v, actionNone
}

func (v *instanceDetailView) render(snap *model.Snapshot, store *events.Store, width, height int) string {
	if snap == nil {
		return ""
	}

	inst := v.findInstance(snap)
	if inst == nil {
		return fmt.Sprintf("instance %q not found", v.sessionName)
	}
	if inst.Orca == nil {
		return "not an orca instance"
	}

	var lines []string

	// Instance info.
	lines = append(lines, fmt.Sprintf("  state:     %s", inst.State))
	if inst.Orca.CurrentRun != nil {
		run := inst.Orca.CurrentRun
		issue := run.IssueID
		if issue == "" {
			issue = "(none)"
		}
		lines = append(lines, fmt.Sprintf("  issue:     %s", issue))
	}
	lines = append(lines, fmt.Sprintf("  duration:  %s", formatDuration(time.Since(inst.Tmux.Created))))
	lines = append(lines, fmt.Sprintf("  session:   %s", v.sessionName))

	// Run list.
	if len(inst.Orca.Runs) > 0 {
		lines = append(lines, "")
		lines = append(lines, "  runs")
		for i, run := range inst.Orca.Runs {
			cursor := "  "
			if i == v.cursor {
				cursor = "> "
			}
			lines = append(lines, cursor+formatRunLine(run))
		}
	}

	// Events.
	if store != nil {
		agentEvents := store.ForAgent(v.agentName)
		// Filter to events for this session.
		var sessionEvents []model.Event
		for _, e := range agentEvents {
			if e.SessionName == v.sessionName {
				sessionEvents = append(sessionEvents, e)
			}
		}
		if len(sessionEvents) > 0 {
			lines = append(lines, "")
			lines = append(lines, "  events")
			limit := 10
			if len(sessionEvents) < limit {
				limit = len(sessionEvents)
			}
			for _, e := range sessionEvents[:limit] {
				lines = append(lines, "    "+formatEvent(e))
			}
		}
	}

	return strings.Join(lines, "\n")
}

func (v *instanceDetailView) findInstance(snap *model.Snapshot) *model.Instance {
	agent := snap.AgentByName(v.agentName)
	if agent == nil {
		return nil
	}
	for _, inst := range agent.Instances {
		if inst.SessionName == v.sessionName {
			return inst
		}
	}
	return nil
}

func formatRunLine(run model.Run) string {
	num := run.RunID
	// Shorten run ID to just the sequence number if it starts with digits.
	if len(num) >= 4 && num[0] >= '0' && num[0] <= '9' {
		num = "#" + strings.TrimLeft(num[:4], "0")
		if num == "#" {
			num = "#0"
		}
	}

	parts := []string{num, run.Result}
	if run.Result == "" {
		parts = []string{num, "running"}
	}

	if run.IssueID != "" {
		parts = append(parts, run.IssueID)
	}
	if run.Merged {
		parts = append(parts, "merged")
	}
	if run.Duration > 0 {
		parts = append(parts, formatDuration(run.Duration))
	}
	if run.Tokens > 0 {
		parts = append(parts, fmt.Sprintf("%dk tokens", run.Tokens/1000))
	}

	return strings.Join(parts, "  ")
}
