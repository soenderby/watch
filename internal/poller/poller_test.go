package poller

import (
	"testing"
)

func TestNew(t *testing.T) {
	// Basic construction test — no real tmux or filesystem needed.
	// The poller's Poll() method requires real tmux, so we test
	// that it can be constructed without error.
	p := New("/nonexistent/config.json", nil)
	if p == nil {
		t.Fatal("expected non-nil poller")
	}
}

// Integration testing of Poll() is deferred because it requires
// real tmux sessions. The builder and differ are thoroughly tested
// in their own packages with synthetic data.
