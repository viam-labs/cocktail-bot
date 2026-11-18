package order

import (
	"sync"
	"time"
)

// RecentDisplayDuration is long enough for clients polling every few seconds to see an order's outcome.
const RecentDisplayDuration = 2 * time.Minute

// Queue runs orders one at a time. Every drink is served at the same station,
// so after each order the queue holds until Release confirms the drink was
// taken and a fresh glass is in place.
type Queue struct {
	mu             sync.Mutex
	notify         chan struct{}
	pending        []Order
	current        *Order
	awaitingPickup *Order
	recent         []Order
}

func NewQueue() *Queue {
	return &Queue{
		notify: make(chan struct{}, 1),
	}
}

func (q *Queue) Notify() <-chan struct{} {
	return q.notify
}

func (q *Queue) signal() {
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

// Enqueue adds o to the back of the line and returns how many orders are ahead of it.
func (q *Queue) Enqueue(o Order) int {
	q.mu.Lock()
	ahead := len(q.pending)
	if q.current != nil {
		ahead++
	}
	q.pending = append(q.pending, o)
	q.mu.Unlock()
	q.signal()
	return ahead
}

// Start pops the next pending order, unless one is running or a finished drink is still awaiting pickup.
func (q *Queue) Start() (Order, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.current != nil || q.awaitingPickup != nil || len(q.pending) == 0 {
		return Order{}, false
	}
	o := q.pending[0]
	q.pending = q.pending[1:]
	o.StartedAt = time.Now()
	q.current = &o
	return o, true
}

func (q *Queue) SetStep(step string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.current != nil {
		q.current.RawStep = step
	}
}

// Complete finishes the running order and holds the queue until Release.
func (q *Queue) Complete(err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.current == nil {
		return
	}
	done := *q.current
	done.CompletedAt = time.Now()
	if err != nil {
		done.Error = err.Error()
	}
	done.Payload = nil
	q.recent = append(q.recent, done)
	q.awaitingPickup = &done
	q.current = nil
	q.pruneRecentLocked()
}

// Release clears the pickup hold so the next pending order can start.
func (q *Queue) Release() bool {
	q.mu.Lock()
	released := q.awaitingPickup != nil
	q.awaitingPickup = nil
	q.mu.Unlock()
	if released {
		q.signal()
	}
	return released
}

// Remove drops a pending order that has not started yet.
func (q *Queue) Remove(id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, o := range q.pending {
		if o.ID == id {
			q.pending = append(q.pending[:i:i], q.pending[i+1:]...)
			return true
		}
	}
	return false
}

func (q *Queue) pruneRecentLocked() {
	cutoff := time.Now().Add(-RecentDisplayDuration)
	kept := q.recent[:0]
	for _, o := range q.recent {
		if o.CompletedAt.After(cutoff) {
			kept = append(kept, o)
		}
	}
	q.recent = kept
}

type State struct {
	IsBusy         bool    `json:"is_busy"`
	Count          int     `json:"count"`
	Current        *Order  `json:"current,omitempty"`
	AwaitingPickup *Order  `json:"awaiting_pickup,omitempty"`
	Queue          []Order `json:"queue"`
	Recent         []Order `json:"recent"`
}

func (q *Queue) State() State {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pruneRecentLocked()
	s := State{
		IsBusy: q.current != nil,
		Count:  len(q.pending),
		Queue:  append([]Order{}, q.pending...),
		Recent: append([]Order{}, q.recent...),
	}
	if q.current != nil {
		cur := *q.current
		s.Current = &cur
		s.Count++
	}
	if q.awaitingPickup != nil {
		held := *q.awaitingPickup
		s.AwaitingPickup = &held
	}
	return s
}
