package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/soenderby/watch/internal/events"
	"github.com/soenderby/watch/internal/model"
)

var t0 = time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)

func testSnapshot(agents ...*model.Agent) *model.Snapshot {
	return &model.Snapshot{
		Timestamp: t0,
		Agents:    agents,
	}
}

func testAgent(name, project, state string, instances ...*model.Instance) *model.Agent {
	return &model.Agent{
		Name:      name,
		Project:   project,
		State:     state,
		Instances: instances,
	}
}

func testInstance(sessionName, state string) *model.Instance {
	return &model.Instance{
		SessionName: sessionName,
		State:       state,
		Tmux:        model.TmuxState{Created: t0, Activity: t0},
	}
}

func testOrcaInstance(sessionName, state, slotName string) *model.Instance {
	return &model.Instance{
		SessionName: sessionName,
		State:       state,
		Tmux:        model.TmuxState{Created: t0, Activity: t0},
		Orca: &model.OrcaRunState{
			AgentName: slotName,
			SessionID: sessionName,
		},
	}
}

func key(k string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

// --- Overview tests ---

func TestOverview_RenderEmpty(t *testing.T) {
	v := newOverview()
	snap := testSnapshot()
	out := v.render(snap, nil, 80, 24)
	if !strings.Contains(out, "no agents") {
		t.Fatalf("expected empty message, got %q", out)
	}
}

func TestOverview_RenderAgents(t *testing.T) {
	v := newOverview()
	snap := testSnapshot(
		testAgent("worker", "orca", "running", testInstance("s1", "running")),
		testAgent("reviewer", "", "active", testInstance("s2", "active")),
	)

	out := v.render(snap, nil, 80, 24)
	if !strings.Contains(out, "> worker") {
		t.Fatalf("expected cursor on worker, got:\n%s", out)
	}
	if !strings.Contains(out, "  reviewer") {
		t.Fatalf("expected reviewer without cursor, got:\n%s", out)
	}
}

func TestOverview_CursorMovement(t *testing.T) {
	v := newOverview()
	snap := testSnapshot(
		testAgent("a", "", "running", testInstance("s1", "running")),
		testAgent("b", "", "running", testInstance("s2", "running")),
	)

	// Move down.
	v2, act := v.update(key("j"), snap, nil)
	if act != actionNone {
		t.Fatalf("expected actionNone, got %d", act)
	}
	ov := v2.(*overviewView)
	if ov.cursor != 1 {
		t.Fatalf("expected cursor 1 after j, got %d", ov.cursor)
	}

	// Move up.
	v3, _ := ov.update(key("k"), snap, nil)
	ov2 := v3.(*overviewView)
	if ov2.cursor != 0 {
		t.Fatalf("expected cursor 0 after k, got %d", ov2.cursor)
	}
}

func TestOverview_EnterPushesAgentDetail(t *testing.T) {
	v := newOverview()
	snap := testSnapshot(
		testAgent("worker", "orca", "running", testInstance("s1", "running")),
	)

	newV, act := v.update(key("enter"), snap, nil)
	if act != actionPush {
		t.Fatalf("expected actionPush, got %d", act)
	}
	ad, ok := newV.(*agentDetailView)
	if !ok {
		t.Fatalf("expected agentDetailView, got %T", newV)
	}
	if ad.agentName != "worker" {
		t.Fatalf("expected worker, got %q", ad.agentName)
	}
}

func TestOverview_JumpSingleInstance(t *testing.T) {
	v := newOverview()
	snap := testSnapshot(
		testAgent("worker", "", "running", testInstance("s1", "running")),
	)

	newV, act := v.update(key("g"), snap, nil)
	if act != actionJump {
		t.Fatalf("expected actionJump, got %d", act)
	}
	jt, ok := newV.(*jumpTarget)
	if !ok {
		t.Fatalf("expected jumpTarget, got %T", newV)
	}
	if jt.sessionName != "s1" {
		t.Fatalf("expected s1, got %q", jt.sessionName)
	}
}

func TestOverview_NoJumpMultipleInstances(t *testing.T) {
	v := newOverview()
	snap := testSnapshot(
		testAgent("worker", "", "running",
			testInstance("s1", "running"),
			testInstance("s2", "running"),
		),
	)

	_, act := v.update(key("g"), snap, nil)
	if act != actionNone {
		t.Fatalf("expected actionNone for multi-instance, got %d", act)
	}
}

// --- Agent detail tests ---

func TestAgentDetail_RenderInstances(t *testing.T) {
	v := newAgentDetail("worker")
	snap := testSnapshot(
		testAgent("worker", "orca", "running",
			testOrcaInstance("s1", "running", "1"),
			testOrcaInstance("s2", "done", "2"),
		),
	)
	store := events.NewStore(50)

	out := v.render(snap, store, 80, 24)
	if !strings.Contains(out, "> 1") {
		t.Fatalf("expected cursor on slot 1, got:\n%s", out)
	}
	if !strings.Contains(out, "  2") {
		t.Fatalf("expected slot 2 without cursor, got:\n%s", out)
	}
}

func TestAgentDetail_EnterPushesInstanceDetail(t *testing.T) {
	v := newAgentDetail("worker")
	snap := testSnapshot(
		testAgent("worker", "orca", "running",
			testOrcaInstance("s1", "running", "1"),
		),
	)

	newV, act := v.update(key("enter"), snap, nil)
	if act != actionPush {
		t.Fatalf("expected actionPush, got %d", act)
	}
	id, ok := newV.(*instanceDetailView)
	if !ok {
		t.Fatalf("expected instanceDetailView, got %T", newV)
	}
	if id.sessionName != "s1" {
		t.Fatalf("expected s1, got %q", id.sessionName)
	}
	if id.slotName != "1" {
		t.Fatalf("expected slot '1', got %q", id.slotName)
	}
}

func TestAgentDetail_EnterNoopNonOrca(t *testing.T) {
	v := newAgentDetail("reviewer")
	snap := testSnapshot(
		testAgent("reviewer", "", "active", testInstance("s1", "active")),
	)

	_, act := v.update(key("enter"), snap, nil)
	if act != actionNone {
		t.Fatalf("expected no push for non-orca, got %d", act)
	}
}

func TestAgentDetail_JumpToInstance(t *testing.T) {
	v := newAgentDetail("worker")
	snap := testSnapshot(
		testAgent("worker", "", "running", testInstance("s1", "running")),
	)

	newV, act := v.update(key("g"), snap, nil)
	if act != actionJump {
		t.Fatalf("expected actionJump, got %d", act)
	}
	jt := newV.(*jumpTarget)
	if jt.sessionName != "s1" {
		t.Fatalf("expected s1, got %q", jt.sessionName)
	}
}

func TestAgentDetail_ShowsEvents(t *testing.T) {
	v := newAgentDetail("worker")
	snap := testSnapshot(
		testAgent("worker", "orca", "running",
			testOrcaInstance("s1", "running", "1"),
		),
	)
	store := events.NewStore(50)
	store.Add([]model.Event{
		{Timestamp: t0, Type: model.EventInstanceUp, AgentName: "worker", SessionName: "s1"},
	})

	out := v.render(snap, store, 80, 24)
	if !strings.Contains(out, "events") {
		t.Fatalf("expected events section, got:\n%s", out)
	}
	if !strings.Contains(out, "instance up") {
		t.Fatalf("expected instance up event, got:\n%s", out)
	}
}

// --- Instance detail tests ---

func TestInstanceDetail_Title(t *testing.T) {
	v := newInstanceDetail("worker", "s1", "1")
	if v.title() != "1" {
		t.Fatalf("expected title '1', got %q", v.title())
	}

	v2 := newInstanceDetail("worker", "s1", "")
	if v2.title() != "s1" {
		t.Fatalf("expected title 's1', got %q", v2.title())
	}
}

func TestInstanceDetail_RenderRuns(t *testing.T) {
	v := newInstanceDetail("worker", "s1", "1")
	snap := testSnapshot(
		testAgent("worker", "orca", "running", &model.Instance{
			SessionName: "s1",
			State:       "running",
			Tmux:        model.TmuxState{Created: t0, Activity: t0},
			Orca: &model.OrcaRunState{
				AgentName:  "1",
				SessionID:  "s1",
				CurrentRun: &model.Run{RunID: "0002", IssueID: "orca-42"},
				Runs: []model.Run{
					{RunID: "0002", IssueID: "orca-42"},
					{RunID: "0001", Result: "completed", HasSummary: true, Merged: true},
				},
			},
		}),
	)

	out := v.render(snap, nil, 80, 24)
	if !strings.Contains(out, "runs") {
		t.Fatalf("expected runs section, got:\n%s", out)
	}
	if !strings.Contains(out, "orca-42") {
		t.Fatalf("expected issue orca-42, got:\n%s", out)
	}
	if !strings.Contains(out, "#2") {
		t.Fatalf("expected run #2, got:\n%s", out)
	}
}

func TestInstanceDetail_JumpAction(t *testing.T) {
	v := newInstanceDetail("worker", "s1", "1")
	snap := testSnapshot(
		testAgent("worker", "orca", "running",
			testOrcaInstance("s1", "running", "1"),
		),
	)

	newV, act := v.update(key("g"), snap, nil)
	if act != actionJump {
		t.Fatalf("expected actionJump, got %d", act)
	}
	jt := newV.(*jumpTarget)
	if jt.sessionName != "s1" {
		t.Fatalf("expected s1, got %q", jt.sessionName)
	}
}

// --- Help overlay tests ---

func TestHelp_RenderContainsKeybindings(t *testing.T) {
	out := renderHelp(80, 24)
	for _, expected := range []string{"j / ", "enter", "esc", "q"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected help to contain %q, got:\n%s", expected, out)
		}
	}
}
