package bartender

import (
	"sync"
	"time"
)

// statusTracker holds the live "what is the bartender doing right now?" state
// plus a transition log for the current run. begin brackets a run; setPhase
// records a transition. snapshot returns both the current state and the full
// history so a polling client (kiosk) can render step + elapsed and an audit
// view can replay what happened.
//
// Independent of the FIFO order queue — the kiosk's make_cocktail path doesn't
// go through that queue, but still wants observability.
type statusTracker struct {
	mu      sync.Mutex
	busy    bool
	drink   string
	started time.Time
	history []stepEntry
}

type stepEntry struct {
	Step      string
	StartedAt time.Time
}

func (s *statusTracker) begin(drink string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.busy = true
	s.drink = drink
	s.started = time.Now()
	s.history = s.history[:0]
}

func (s *statusTracker) setPhase(phase string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = append(s.history, stepEntry{Step: phase, StartedAt: time.Now()})
}

func (s *statusTracker) end() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.busy = false
	s.drink = ""
}

type statusSnapshot struct {
	busy    bool
	drink   string
	started time.Time
	history []stepEntry
}

func (s *statusTracker) snapshot() statusSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]stepEntry, len(s.history))
	copy(cp, s.history)
	return statusSnapshot{busy: s.busy, drink: s.drink, started: s.started, history: cp}
}
