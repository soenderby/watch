package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/soenderby/watch/internal/events"
	"github.com/soenderby/watch/internal/model"
	"github.com/soenderby/watch/internal/tmux"
)

// jumpTarget is a pseudo-view used to carry the session name for a jump action.
type jumpTarget struct {
	sessionName string
}

func (j *jumpTarget) update(tea.Msg, *model.Snapshot, *events.Store) (view, action) {
	return j, actionNone
}

func (j *jumpTarget) render(*model.Snapshot, *events.Store, int, int) string {
	return ""
}

func (j *jumpTarget) title() string { return "" }

// jumpCmd returns a tea.Cmd that performs the tmux jump.
func jumpCmd(sessionName string) tea.Cmd {
	return func() tea.Msg {
		if tmux.InsideTmux() {
			_ = tmux.SwitchClient(sessionName)
		} else {
			_ = tmux.AttachSession(sessionName)
		}
		return nil
	}
}
