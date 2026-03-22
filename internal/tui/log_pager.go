package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/soenderby/watch/internal/events"
	"github.com/soenderby/watch/internal/model"
)

// logPagerView shows a run.log file in a scrollable pager.
type logPagerView struct {
	path   string
	lines  []string
	offset int
	loaded bool
	err    error
}

func newLogPager(path string) *logPagerView {
	return &logPagerView{path: path}
}

func (v *logPagerView) title() string { return "log" }

func (v *logPagerView) update(msg tea.Msg, snap *model.Snapshot, store *events.Store) (view, action) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return v, actionNone
	}

	switch km.String() {
	case "q", "esc":
		return v, actionPop
	case "j", "down":
		if v.offset < len(v.lines)-1 {
			v.offset++
		}
	case "k", "up":
		if v.offset > 0 {
			v.offset--
		}
	case "d": // half page down
		v.offset += 10
		if v.offset >= len(v.lines) {
			v.offset = len(v.lines) - 1
		}
		if v.offset < 0 {
			v.offset = 0
		}
	case "u": // half page up
		v.offset -= 10
		if v.offset < 0 {
			v.offset = 0
		}
	case "G": // bottom
		if len(v.lines) > 0 {
			v.offset = len(v.lines) - 1
		}
	case "g": // top (single g in this context)
		v.offset = 0
	}

	return v, actionNone
}

func (v *logPagerView) render(snap *model.Snapshot, store *events.Store, width, height int) string {
	if !v.loaded {
		v.load()
	}

	if v.err != nil {
		return fmt.Sprintf("  error reading log: %v", v.err)
	}

	if len(v.lines) == 0 {
		return "  (empty log)"
	}

	// Show a window of lines from offset.
	end := v.offset + height
	if end > len(v.lines) {
		end = len(v.lines)
	}
	start := v.offset
	if start < 0 {
		start = 0
	}

	visible := v.lines[start:end]
	// Truncate lines to width.
	var truncated []string
	for _, line := range visible {
		if len(line) > width {
			line = line[:width]
		}
		truncated = append(truncated, line)
	}

	pos := fmt.Sprintf("  [%d/%d]  %s", v.offset+1, len(v.lines), v.path)
	return strings.Join(truncated, "\n") + "\n\n" + pos
}

func (v *logPagerView) load() {
	v.loaded = true
	data, err := os.ReadFile(v.path)
	if err != nil {
		v.err = err
		return
	}
	v.lines = strings.Split(string(data), "\n")
}
