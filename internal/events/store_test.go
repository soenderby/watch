package events

import (
	"testing"
	"time"

	"github.com/soenderby/watch/internal/model"
)

func TestStore_AddAndQuery(t *testing.T) {
	s := NewStore(50)

	s.Add([]model.Event{
		{Timestamp: t0, Type: model.EventInstanceUp, AgentName: "worker"},
		{Timestamp: t0, Type: model.EventInstanceUp, AgentName: "reviewer"},
	})

	worker := s.ForAgent("worker")
	if len(worker) != 1 {
		t.Fatalf("expected 1 worker event, got %d", len(worker))
	}

	reviewer := s.ForAgent("reviewer")
	if len(reviewer) != 1 {
		t.Fatalf("expected 1 reviewer event, got %d", len(reviewer))
	}
}

func TestStore_ForAgent_NewestFirst(t *testing.T) {
	s := NewStore(50)

	s.Add([]model.Event{
		{Timestamp: t0, Type: model.EventInstanceUp, AgentName: "worker"},
		{Timestamp: t1, Type: model.EventRunStarted, AgentName: "worker"},
	})

	events := s.ForAgent("worker")
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != model.EventRunStarted {
		t.Errorf("expected newest first (run_started), got %q", events[0].Type)
	}
	if events[1].Type != model.EventInstanceUp {
		t.Errorf("expected oldest last (instance_up), got %q", events[1].Type)
	}
}

func TestStore_ForAgent_Unknown(t *testing.T) {
	s := NewStore(50)
	events := s.ForAgent("nonexistent")
	if len(events) != 0 {
		t.Fatalf("expected 0 events for unknown agent, got %d", len(events))
	}
}

func TestStore_Cap(t *testing.T) {
	s := NewStore(3)

	for i := 0; i < 5; i++ {
		s.Add([]model.Event{
			{
				Timestamp: t0.Add(time.Duration(i) * time.Second),
				Type:      model.EventRunStarted,
				AgentName: "worker",
				RunID:     string(rune('a' + i)),
			},
		})
	}

	events := s.ForAgent("worker")
	if len(events) != 3 {
		t.Fatalf("expected 3 events (capped), got %d", len(events))
	}
	// Oldest should have been trimmed.
	// Events are stored chronologically, returned newest first.
	// We added a, b, c, d, e. After cap=3, stored should be c, d, e.
	if events[0].RunID != string('e') {
		t.Errorf("expected newest event 'e', got %q", events[0].RunID)
	}
	if events[2].RunID != string('c') {
		t.Errorf("expected oldest retained event 'c', got %q", events[2].RunID)
	}
}

func TestStore_ScopingIsolation(t *testing.T) {
	s := NewStore(3)

	// Fill worker to cap.
	for i := 0; i < 5; i++ {
		s.Add([]model.Event{
			{Timestamp: t0.Add(time.Duration(i) * time.Second), AgentName: "worker"},
		})
	}

	// Add one reviewer event.
	s.Add([]model.Event{
		{Timestamp: t0, AgentName: "reviewer"},
	})

	// Worker should be capped at 3.
	if len(s.ForAgent("worker")) != 3 {
		t.Fatalf("expected 3 worker events, got %d", len(s.ForAgent("worker")))
	}
	// Reviewer should be unaffected.
	if len(s.ForAgent("reviewer")) != 1 {
		t.Fatalf("expected 1 reviewer event, got %d", len(s.ForAgent("reviewer")))
	}
}

func TestStore_All(t *testing.T) {
	s := NewStore(50)

	s.Add([]model.Event{
		{Timestamp: t0, AgentName: "worker", Type: model.EventInstanceUp},
		{Timestamp: t1, AgentName: "reviewer", Type: model.EventInstanceUp},
		{Timestamp: t0.Add(1 * time.Second), AgentName: "worker", Type: model.EventRunStarted},
	})

	all := s.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 total events, got %d", len(all))
	}
	// Should be sorted newest first.
	if all[0].Timestamp != t1 {
		t.Errorf("expected newest first, got timestamp %v", all[0].Timestamp)
	}
}

func TestStore_Clear(t *testing.T) {
	s := NewStore(50)
	s.Add([]model.Event{
		{Timestamp: t0, AgentName: "worker"},
	})

	s.Clear()

	if len(s.ForAgent("worker")) != 0 {
		t.Fatal("expected empty after Clear")
	}
	if len(s.All()) != 0 {
		t.Fatal("expected All empty after Clear")
	}
}
