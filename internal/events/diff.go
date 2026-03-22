// Package events provides snapshot diffing and event accumulation.
package events

import (
	"time"

	"github.com/soenderby/watch/internal/model"
)

// Diff compares two snapshots and returns the events that represent
// the transition from prev to curr. When prev is nil, all instances
// in curr generate instance_up events.
func Diff(prev, curr *model.Snapshot) []model.Event {
	if curr == nil {
		return nil
	}

	var events []model.Event
	ts := curr.Timestamp

	// Index previous instances by session name.
	prevInstances := make(map[string]prevEntry)
	if prev != nil {
		for _, agent := range prev.Agents {
			for _, inst := range agent.Instances {
				prevInstances[inst.SessionName] = prevEntry{
					agent:    agent,
					instance: inst,
				}
			}
		}
	}

	// Index current instances by session name.
	currInstances := make(map[string]bool)
	for _, agent := range curr.Agents {
		for _, inst := range agent.Instances {
			currInstances[inst.SessionName] = true

			pe, existed := prevInstances[inst.SessionName]
			if !existed {
				// New instance.
				events = append(events, model.Event{
					Timestamp:   ts,
					Type:        model.EventInstanceUp,
					AgentName:   agent.Name,
					ProjectName: agent.Project,
					SessionName: inst.SessionName,
				})
			}

			// Check for orca run changes.
			if inst.Orca != nil {
				events = append(events, diffOrcaRuns(ts, agent, inst, pe, existed)...)
			}
		}
	}

	// Instances that disappeared.
	for sessionName, pe := range prevInstances {
		if !currInstances[sessionName] {
			events = append(events, model.Event{
				Timestamp:   ts,
				Type:        model.EventInstanceDown,
				AgentName:   pe.agent.Name,
				ProjectName: pe.agent.Project,
				SessionName: sessionName,
			})
		}
	}

	return events
}

type prevEntry struct {
	agent    *model.Agent
	instance *model.Instance
}

func diffOrcaRuns(ts time.Time, agent *model.Agent, curr *model.Instance, pe prevEntry, existed bool) []model.Event {
	var events []model.Event

	// Build index of previously known runs.
	prevRuns := make(map[string]*model.Run)
	if existed && pe.instance.Orca != nil {
		for i := range pe.instance.Orca.Runs {
			r := &pe.instance.Orca.Runs[i]
			prevRuns[r.RunID] = r
		}
	}

	for _, run := range curr.Orca.Runs {
		prevRun, wasSeen := prevRuns[run.RunID]

		if !wasSeen {
			// New run.
			events = append(events, model.Event{
				Timestamp:   ts,
				Type:        model.EventRunStarted,
				AgentName:   agent.Name,
				ProjectName: agent.Project,
				SessionName: curr.SessionName,
				RunID:       run.RunID,
				IssueID:     run.IssueID,
			})
			// If the new run already has a summary, also emit completed.
			if run.HasSummary {
				events = append(events, model.Event{
					Timestamp:   ts,
					Type:        model.EventRunCompleted,
					AgentName:   agent.Name,
					ProjectName: agent.Project,
					SessionName: curr.SessionName,
					RunID:       run.RunID,
					IssueID:     run.IssueID,
					Result:      run.Result,
					Merged:      run.Merged,
				})
			}
		} else if !prevRun.HasSummary && run.HasSummary {
			// Run gained a summary.
			events = append(events, model.Event{
				Timestamp:   ts,
				Type:        model.EventRunCompleted,
				AgentName:   agent.Name,
				ProjectName: agent.Project,
				SessionName: curr.SessionName,
				RunID:       run.RunID,
				IssueID:     run.IssueID,
				Result:      run.Result,
				Merged:      run.Merged,
			})
		}
	}

	return events
}
