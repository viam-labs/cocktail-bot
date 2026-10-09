package bartender

import (
	"sync"
	"time"
)

type statusTracker struct {
	mu          sync.Mutex
	busy        bool
	drink       string
	started     time.Time
	history     []stepEntry
	lastOutcome *outcome
}

type stepEntry struct {
	Step      string
	StartedAt time.Time
}

type outcome struct {
	Drink   string
	Err     error
	EndedAt time.Time
}

func (s *statusTracker) begin(drink string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.busy = true
	s.drink = drink
	s.started = time.Now()
	s.history = s.history[:0]
	s.lastOutcome = nil
}

func (s *statusTracker) setPhase(phase string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = append(s.history, stepEntry{Step: phase, StartedAt: time.Now()})
}

func (s *statusTracker) finish(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastOutcome = &outcome{Drink: s.drink, Err: err, EndedAt: time.Now()}
	s.busy = false
	s.drink = ""
}

type statusSnapshot struct {
	busy        bool
	drink       string
	started     time.Time
	history     []stepEntry
	lastOutcome *outcome
}

func (s *statusTracker) snapshot() statusSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]stepEntry, len(s.history))
	copy(cp, s.history)
	return statusSnapshot{busy: s.busy, drink: s.drink, started: s.started, history: cp, lastOutcome: s.lastOutcome}
}
