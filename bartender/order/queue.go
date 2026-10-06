package order

import (
	"sync"
	"time"
)

const RecentDisplayDuration = 15 * time.Second

type Queue struct {
	mu      sync.Mutex
	notify  chan struct{}
	pending []Order
	current *Order
	recent  []Order
}

func NewQueue() *Queue {
	return &Queue{
		notify: make(chan struct{}, 1),
	}
}

func (q *Queue) Notify() <-chan struct{} {
	return q.notify
}

func (q *Queue) Enqueue(o Order) {
	q.mu.Lock()
	q.pending = append(q.pending, o)
	q.mu.Unlock()
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

func (q *Queue) Start() (Order, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) == 0 {
		return Order{}, false
	}
	o := q.pending[0]
	q.pending = q.pending[1:]
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

func (q *Queue) Complete() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.current == nil {
		return
	}
	done := *q.current
	done.CompletedAt = time.Now()
	q.recent = append(q.recent, done)
	q.current = nil
	q.pruneRecentLocked()
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
	IsBusy  bool    `json:"is_busy"`
	Count   int     `json:"count"`
	Current *Order  `json:"current,omitempty"`
	Queue   []Order `json:"queue"`
	Recent  []Order `json:"recent"`
}

func (q *Queue) State() State {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pruneRecentLocked()
	s := State{
		IsBusy: q.current != nil,
		Count:  len(q.pending),
		Queue:  append([]Order(nil), q.pending...),
		Recent: append([]Order(nil), q.recent...),
	}
	if q.current != nil {
		cur := *q.current
		s.Current = &cur
		s.Count++
	}
	return s
}
