package events

import (
	"testing"
	"time"

	"github.com/soenderby/watch/internal/model"
)

var t0 = time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
var t1 = time.Date(2026, 3, 20, 10, 0, 3, 0, time.UTC)

func TestDiff_NilCurr(t *testing.T) {
	events := Diff(&model.Snapshot{}, nil)
	if len(events) != 0 {
		t.Fatalf("expected no events for nil curr, got %d", len(events))
	}
}

func TestDiff_NilPrev(t *testing.T) {
	curr := &model.Snapshot{
		Timestamp: t0,
		Agents: []*model.Agent{
			{
				Name:    "worker",
				Project: "orca",
				Instances: []*model.Instance{
					{SessionName: "orca-agent-1-20260320T100000Z"},
					{SessionName: "orca-agent-2-20260320T100000Z"},
				},
			},
		},
	}

	events := Diff(nil, curr)
	if len(events) != 2 {
		t.Fatalf("expected 2 instance_up events, got %d", len(events))
	}
	for _, e := range events {
		if e.Type != model.EventInstanceUp {
			t.Errorf("expected instance_up, got %q", e.Type)
		}
		if e.AgentName != "worker" {
			t.Errorf("expected agent 'worker', got %q", e.AgentName)
		}
		if e.ProjectName != "orca" {
			t.Errorf("expected project 'orca', got %q", e.ProjectName)
		}
	}
}

func TestDiff_NoChange(t *testing.T) {
	snap := &model.Snapshot{
		Timestamp: t0,
		Agents: []*model.Agent{
			{
				Name: "worker",
				Instances: []*model.Instance{
					{SessionName: "session-1"},
				},
			},
		},
	}

	// Same structure, different timestamp.
	curr := &model.Snapshot{
		Timestamp: t1,
		Agents: []*model.Agent{
			{
				Name: "worker",
				Instances: []*model.Instance{
					{SessionName: "session-1"},
				},
			},
		},
	}

	events := Diff(snap, curr)
	if len(events) != 0 {
		t.Fatalf("expected no events for unchanged snapshot, got %d", len(events))
	}
}

func TestDiff_InstanceAppears(t *testing.T) {
	prev := &model.Snapshot{
		Timestamp: t0,
		Agents:    []*model.Agent{},
	}
	curr := &model.Snapshot{
		Timestamp: t1,
		Agents: []*model.Agent{
			{
				Name:    "worker",
				Project: "proj",
				Instances: []*model.Instance{
					{SessionName: "session-1"},
				},
			},
		},
	}

	events := Diff(prev, curr)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]
	if e.Type != model.EventInstanceUp {
		t.Errorf("expected instance_up, got %q", e.Type)
	}
	if e.SessionName != "session-1" {
		t.Errorf("expected session 'session-1', got %q", e.SessionName)
	}
	if e.Timestamp != t1 {
		t.Errorf("expected timestamp %v, got %v", t1, e.Timestamp)
	}
}

func TestDiff_InstanceDisappears(t *testing.T) {
	prev := &model.Snapshot{
		Timestamp: t0,
		Agents: []*model.Agent{
			{
				Name:    "worker",
				Project: "proj",
				Instances: []*model.Instance{
					{SessionName: "session-1"},
				},
			},
		},
	}
	curr := &model.Snapshot{
		Timestamp: t1,
		Agents:    []*model.Agent{},
	}

	events := Diff(prev, curr)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]
	if e.Type != model.EventInstanceDown {
		t.Errorf("expected instance_down, got %q", e.Type)
	}
	if e.SessionName != "session-1" {
		t.Errorf("expected session 'session-1', got %q", e.SessionName)
	}
	if e.AgentName != "worker" {
		t.Errorf("expected agent 'worker', got %q", e.AgentName)
	}
}

func TestDiff_RunStarted(t *testing.T) {
	prev := &model.Snapshot{
		Timestamp: t0,
		Agents: []*model.Agent{
			{
				Name: "worker",
				Instances: []*model.Instance{
					{
						SessionName: "session-1",
						Orca:        &model.OrcaRunState{Runs: []model.Run{}},
					},
				},
			},
		},
	}
	curr := &model.Snapshot{
		Timestamp: t1,
		Agents: []*model.Agent{
			{
				Name: "worker",
				Instances: []*model.Instance{
					{
						SessionName: "session-1",
						Orca: &model.OrcaRunState{
							Runs: []model.Run{
								{RunID: "0001", IssueID: "orca-123"},
							},
						},
					},
				},
			},
		},
	}

	events := Diff(prev, curr)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]
	if e.Type != model.EventRunStarted {
		t.Errorf("expected run_started, got %q", e.Type)
	}
	if e.RunID != "0001" {
		t.Errorf("expected run ID '0001', got %q", e.RunID)
	}
	if e.IssueID != "orca-123" {
		t.Errorf("expected issue 'orca-123', got %q", e.IssueID)
	}
}

func TestDiff_RunCompleted(t *testing.T) {
	prev := &model.Snapshot{
		Timestamp: t0,
		Agents: []*model.Agent{
			{
				Name: "worker",
				Instances: []*model.Instance{
					{
						SessionName: "session-1",
						Orca: &model.OrcaRunState{
							Runs: []model.Run{
								{RunID: "0001", HasSummary: false},
							},
						},
					},
				},
			},
		},
	}
	curr := &model.Snapshot{
		Timestamp: t1,
		Agents: []*model.Agent{
			{
				Name: "worker",
				Instances: []*model.Instance{
					{
						SessionName: "session-1",
						Orca: &model.OrcaRunState{
							Runs: []model.Run{
								{RunID: "0001", HasSummary: true, Result: "completed", IssueID: "orca-123", Merged: true},
							},
						},
					},
				},
			},
		},
	}

	events := Diff(prev, curr)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]
	if e.Type != model.EventRunCompleted {
		t.Errorf("expected run_completed, got %q", e.Type)
	}
	if e.Result != "completed" {
		t.Errorf("expected result 'completed', got %q", e.Result)
	}
	if !e.Merged {
		t.Error("expected merged=true")
	}
}

func TestDiff_NewRunWithSummary(t *testing.T) {
	// A run that appears already completed (e.g., fast run between polls).
	prev := &model.Snapshot{
		Timestamp: t0,
		Agents: []*model.Agent{
			{
				Name: "worker",
				Instances: []*model.Instance{
					{
						SessionName: "session-1",
						Orca:        &model.OrcaRunState{Runs: []model.Run{}},
					},
				},
			},
		},
	}
	curr := &model.Snapshot{
		Timestamp: t1,
		Agents: []*model.Agent{
			{
				Name: "worker",
				Instances: []*model.Instance{
					{
						SessionName: "session-1",
						Orca: &model.OrcaRunState{
							Runs: []model.Run{
								{RunID: "0001", HasSummary: true, Result: "completed", IssueID: "orca-42"},
							},
						},
					},
				},
			},
		},
	}

	events := Diff(prev, curr)
	// Should emit both run_started and run_completed.
	if len(events) != 2 {
		t.Fatalf("expected 2 events (started + completed), got %d", len(events))
	}
	if events[0].Type != model.EventRunStarted {
		t.Errorf("expected first event run_started, got %q", events[0].Type)
	}
	if events[1].Type != model.EventRunCompleted {
		t.Errorf("expected second event run_completed, got %q", events[1].Type)
	}
}

func TestDiff_MultipleEvents(t *testing.T) {
	prev := &model.Snapshot{
		Timestamp: t0,
		Agents: []*model.Agent{
			{
				Name: "worker",
				Instances: []*model.Instance{
					{SessionName: "old-session"},
				},
			},
		},
	}
	curr := &model.Snapshot{
		Timestamp: t1,
		Agents: []*model.Agent{
			{
				Name:    "worker",
				Project: "proj",
				Instances: []*model.Instance{
					{SessionName: "new-session"},
				},
			},
		},
	}

	events := Diff(prev, curr)
	// old-session disappeared, new-session appeared.
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	var gotUp, gotDown bool
	for _, e := range events {
		switch e.Type {
		case model.EventInstanceUp:
			gotUp = true
			if e.SessionName != "new-session" {
				t.Errorf("expected new-session for up, got %q", e.SessionName)
			}
		case model.EventInstanceDown:
			gotDown = true
			if e.SessionName != "old-session" {
				t.Errorf("expected old-session for down, got %q", e.SessionName)
			}
		}
	}
	if !gotUp {
		t.Error("missing instance_up event")
	}
	if !gotDown {
		t.Error("missing instance_down event")
	}
}
