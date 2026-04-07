// Package tui implements the watch terminal user interface.
package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/soenderby/watch/internal/events"
	"github.com/soenderby/watch/internal/model"
	"github.com/soenderby/watch/internal/poller"
)

// pollInterval is how often the TUI refreshes data.
const pollInterval = 3 * time.Second

// pollMsg carries the result of a poll tick.
type pollMsg struct {
	result *poller.Result
	err    error
}

// App is the top-level bubbletea model.
type App struct {
	poller   *poller.Poller
	store    *events.Store
	snapshot *model.Snapshot
	stack    []view // navigation stack
	width    int
	height   int
	showHelp bool
	err      error
}

// view is implemented by each navigation level.
type view interface {
	// update handles a key or data message and returns the updated view
	// plus an optional action for the app to perform.
	update(msg tea.Msg, snap *model.Snapshot, store *events.Store) (view, action)

	// render returns the view's content as a string.
	render(snap *model.Snapshot, store *events.Store, width, height int) string

	// title returns the breadcrumb segment for this view.
	title() string
}

// action tells the app what to do after a view handles a message.
type action int

const (
	actionNone action = iota
	actionPush        // push a new view (returned view is the new one)
	actionPop         // pop this view
	actionJump        // jump to tmux session
	actionQuit        // quit the app
)

// New creates a new TUI app.
func New(p *poller.Poller, store *events.Store) *App {
	return &App{
		poller: p,
		store:  store,
		stack:  []view{newOverview()},
	}
}

// Init performs the first poll.
func (a *App) Init() tea.Cmd {
	return a.doPoll()
}

// Update handles messages.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case pollMsg:
		if msg.err == nil && msg.result != nil {
			a.snapshot = msg.result.Snapshot
		}
		return a, a.schedulePoll()

	case tea.KeyMsg:
		return a.handleKey(msg)
	}

	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// When help is showing, only ? and q are active.
	if a.showHelp {
		switch msg.String() {
		case "?", "esc":
			a.showHelp = false
			return a, nil
		case "q":
			return a, tea.Quit
		}
		return a, nil
	}

	switch msg.String() {
	case "q":
		return a, tea.Quit
	case "?":
		a.showHelp = !a.showHelp
		return a, nil
	case "r":
		return a, a.doPoll()
	case "esc":
		if len(a.stack) > 1 {
			a.stack = a.stack[:len(a.stack)-1]
		}
		return a, nil
	}

	// Delegate to current view.
	if len(a.stack) == 0 {
		return a, nil
	}

	current := a.stack[len(a.stack)-1]
	newView, act := current.update(msg, a.snapshot, a.store)

	switch act {
	case actionPush:
		a.stack = append(a.stack, newView)
	case actionPop:
		if len(a.stack) > 1 {
			a.stack = a.stack[:len(a.stack)-1]
		}
	case actionJump:
		return a, a.doJump(newView)
	case actionQuit:
		return a, tea.Quit
	default:
		a.stack[len(a.stack)-1] = newView
	}

	return a, nil
}

// View renders the current screen.
func (a *App) View() string {
	if a.snapshot == nil {
		return "loading..."
	}
	if len(a.stack) == 0 {
		return ""
	}

	// Build breadcrumb.
	breadcrumb := "watch"
	for _, v := range a.stack {
		t := v.title()
		if t != "" {
			breadcrumb += " / " + t
		}
	}

	// Reserve lines for header, footer, and padding.
	headerHeight := 2
	footerHeight := 2
	contentHeight := a.height - headerHeight - footerHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	current := a.stack[len(a.stack)-1]
	content := current.render(a.snapshot, a.store, a.width, contentHeight)

	// Build header.
	header := breadcrumb
	agentCount := len(a.snapshot.Agents)
	right := fmt.Sprintf("%d agents  %s", agentCount, time.Now().Format("15:04"))
	pad := a.width - len(breadcrumb) - len(right)
	if pad > 0 {
		header = breadcrumb + spaces(pad) + right
	}

	// Build footer.
	footer := a.footerForView(current)

	// Back indicator for non-root views.
	if len(a.stack) > 1 {
		backLabel := "esc back"
		headerPad := a.width - len(breadcrumb) - len(backLabel)
		if headerPad > 0 {
			header = breadcrumb + spaces(headerPad) + backLabel
		}
	}

	if a.showHelp {
		content = renderHelp(a.width, contentHeight)
		footer = "? close help  q quit"
	}

	return header + "\n\n" + content + "\n\n" + footer
}

func renderHelp(width, height int) string {
	help := `Keybindings

  j / ↓          Move cursor down
  k / ↑          Move cursor up
  enter          Drill into selected item
  esc            Go back one level
  g              Jump to tmux session
  l              View run log (Level 2)
  r              Force refresh
  ?              Toggle this help
  q              Quit

Navigation

  Level 0        Overview — all agents
  Level 1        Agent detail — instances + events
  Level 2        Instance detail — runs + events

  enter          pushes a level
  esc            pops a level
  g              jumps to tmux (watch stays running)`

	return help
}

func (a *App) footerForView(v view) string {
	switch v.(type) {
	case *overviewView:
		return "j/k move  enter expand  g jump  q quit  ? help"
	case *agentDetailView:
		return "j/k move  enter detail  g jump  esc back  ? help"
	case *instanceDetailView:
		return "j/k move  g jump  l log  esc back  ? help"
	default:
		return "q quit  ? help"
	}
}

func (a *App) doPoll() tea.Cmd {
	return func() tea.Msg {
		result, err := a.poller.Poll()
		return pollMsg{result: result, err: err}
	}
}

func (a *App) schedulePoll() tea.Cmd {
	return tea.Tick(pollInterval, func(time.Time) tea.Msg {
		result, err := a.poller.Poll()
		return pollMsg{result: result, err: err}
	})
}

func (a *App) doJump(v view) tea.Cmd {
	// The view returned on actionJump carries the target session name.
	// We extract it from the jump target view.
	if jt, ok := v.(*jumpTarget); ok {
		return jumpCmd(jt.sessionName)
	}
	return nil
}

func spaces(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
