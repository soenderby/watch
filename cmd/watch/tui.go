package main

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/soenderby/watch/internal/events"
	"github.com/soenderby/watch/internal/poller"
	"github.com/soenderby/watch/internal/tui"
)

func runTUI() error {
	cfgPath, err := configPath()
	if err != nil {
		return err
	}

	store := events.NewStore(50)
	p := poller.New(cfgPath, store)
	app := tui.New(p, store)

	prog := tea.NewProgram(app, tea.WithAltScreen())
	_, err = prog.Run()
	return err
}
