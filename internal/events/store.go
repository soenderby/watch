package events

import (
	"sort"

	"github.com/soenderby/watch/internal/model"
)

// Store accumulates events over time, scoped by agent name and capped
// to prevent unbounded memory growth.
type Store struct {
	cap    int
	events map[string][]model.Event // keyed by agent name
}

// NewStore creates a new event store with the given per-agent cap.
func NewStore(cap int) *Store {
	if cap <= 0 {
		cap = 50
	}
	return &Store{
		cap:    cap,
		events: make(map[string][]model.Event),
	}
}

// Add appends events to the store, scoped by each event's AgentName.
// Oldest events are trimmed when the cap is exceeded.
func (s *Store) Add(events []model.Event) {
	for _, e := range events {
		key := e.AgentName
		s.events[key] = append(s.events[key], e)
		if len(s.events[key]) > s.cap {
			excess := len(s.events[key]) - s.cap
			s.events[key] = s.events[key][excess:]
		}
	}
}

// ForAgent returns events for the named agent, newest first.
func (s *Store) ForAgent(name string) []model.Event {
	events := s.events[name]
	if len(events) == 0 {
		return nil
	}
	result := make([]model.Event, len(events))
	copy(result, events)
	// Reverse to get newest first.
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

// All returns all events across all agents, newest first.
func (s *Store) All() []model.Event {
	var all []model.Event
	for _, events := range s.events {
		all = append(all, events...)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].Timestamp.After(all[j].Timestamp)
	})
	return all
}

// Clear removes all events.
func (s *Store) Clear() {
	s.events = make(map[string][]model.Event)
}
