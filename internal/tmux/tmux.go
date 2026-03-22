// Package tmux provides tmux session discovery and interaction.
package tmux

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Session represents a tmux session.
type Session struct {
	Name     string    `json:"name"`
	Windows  int       `json:"windows"`
	Created  time.Time `json:"created"`
	Attached bool      `json:"attached"`
	Activity time.Time `json:"activity"` // last activity timestamp
	Path     string    `json:"path"`     // working directory of the active pane
}

// Installed reports whether the tmux binary exists on PATH.
func Installed() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// ListSessions returns all active tmux sessions.
func ListSessions() ([]Session, error) {
	if !Installed() {
		return nil, fmt.Errorf("tmux is not installed")
	}

	// Format: name|windows|created_epoch|attached_flag|activity_epoch|pane_path
	format := "#{session_name}|#{session_windows}|#{session_created}|#{session_attached}|#{session_activity}|#{pane_current_path}"
	cmd := exec.Command("tmux", "list-sessions", "-F", format)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// No sessions
			return nil, nil
		}
		return nil, fmt.Errorf("tmux list-sessions: %w", err)
	}

	var sessions []Session
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		s, err := ParseSessionLine(line)
		if err != nil {
			continue // skip malformed lines
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// SwitchClient switches the current tmux client to the given session.
func SwitchClient(sessionName string) error {
	cmd := exec.Command("tmux", "switch-client", "-t", sessionName)
	return cmd.Run()
}

// AttachSession attaches to the given tmux session (for use outside tmux).
func AttachSession(sessionName string) error {
	cmd := exec.Command("tmux", "attach-session", "-t", sessionName)
	cmd.Stdin = nil
	return cmd.Run()
}

// InsideTmux reports whether the current process is running inside a tmux session.
func InsideTmux() bool {
	cmd := exec.Command("tmux", "display-message", "-p", "#{client_tty}")
	err := cmd.Run()
	return err == nil
}

// ParseSessionLine parses a single line of tmux list-sessions output.
// Exported for testing.
func ParseSessionLine(line string) (Session, error) {
	parts := strings.SplitN(line, "|", 6)
	if len(parts) != 6 {
		return Session{}, fmt.Errorf("expected 6 fields, got %d", len(parts))
	}

	windows, err := strconv.Atoi(parts[1])
	if err != nil {
		windows = 0
	}

	var created time.Time
	if epoch, err := strconv.ParseInt(parts[2], 10, 64); err == nil {
		created = time.Unix(epoch, 0)
	}

	attached := parts[3] == "1"

	var activity time.Time
	if epoch, err := strconv.ParseInt(parts[4], 10, 64); err == nil {
		activity = time.Unix(epoch, 0)
	}

	path := parts[5]

	return Session{
		Name:     parts[0],
		Windows:  windows,
		Created:  created,
		Attached: attached,
		Activity: activity,
		Path:     path,
	}, nil
}
