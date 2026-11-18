package order

import (
	"fmt"
	"sync"
	"time"
)

// RecentDisplayDuration is long enough for clients polling every few seconds to see an order's outcome.
const RecentDisplayDuration = 2 * time.Minute

// ErrInterrupted is recorded on an order that was being made when the module stopped.
const ErrInterrupted = "interrupted: the bartender restarted while this drink was being made"

// Queue runs orders one at a time. Every drink is served at the same station,
// so after each order the queue holds until Release confirms the drink was
// taken and a fresh glass is in place. An order that names a glass also holds
// until a Release made while it was next in line, since only then were staff
// shown which glass to place.
type Queue struct {
	mu             sync.Mutex
	store          Store
	onSaveErr      func(error)
	notify         chan struct{}
	pending        []Order
	current        *Order
	awaitingPickup *Order
	glassReadyFor  string
	recent         []Order
}

// NewQueue returns a queue that lives only in memory.
func NewQueue() *Queue {
	return &Queue{
		notify:    make(chan struct{}, 1),
		onSaveErr: func(error) {},
	}
}

// OpenQueue restores the queue from store and saves every change back to it.
// An order that was mid-drink is returned as interrupted: it is not re-made,
// and the queue holds so someone can clear the station first.
func OpenQueue(store Store, onSaveErr func(error)) (*Queue, *Order, error) {
	snap, err := store.Load()
	if err != nil {
		return nil, nil, err
	}
	q := NewQueue()
	q.store = store
	if onSaveErr != nil {
		q.onSaveErr = onSaveErr
	}
	q.pending = snap.Pending
	q.recent = snap.Recent
	q.awaitingPickup = snap.AwaitingPickup
	var interrupted *Order
	if snap.Current != nil {
		o := *snap.Current
		o.CompletedAt = time.Now()
		o.Error = ErrInterrupted
		o.Payload = nil
		q.recent = append(q.recent, o)
		q.awaitingPickup = &o
		interrupted = &o
	}
	q.pruneRecentLocked()
	if err := q.saveLocked(); err != nil {
		return nil, nil, err
	}
	if len(q.pending) > 0 {
		q.signal()
	}
	return q, interrupted, nil
}

func (q *Queue) saveLocked() error {
	if q.store == nil {
		return nil
	}
	return q.store.Save(Snapshot{
		Pending:        q.pending,
		Current:        q.current,
		AwaitingPickup: q.awaitingPickup,
		Recent:         q.recent,
	})
}

func (q *Queue) saveOrReportLocked() {
	if err := q.saveLocked(); err != nil {
		q.onSaveErr(err)
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
// The order is rejected if it cannot be saved, so a restart never loses an accepted order.
func (q *Queue) Enqueue(o Order) (int, error) {
	q.mu.Lock()
	ahead := len(q.pending)
	if q.current != nil {
		ahead++
	}
	q.pending = append(q.pending, o)
	if err := q.saveLocked(); err != nil {
		q.pending = q.pending[:len(q.pending)-1]
		q.mu.Unlock()
		return 0, fmt.Errorf("save order queue: %w", err)
	}
	q.mu.Unlock()
	q.signal()
	return ahead, nil
}

// Start pops the next pending order, unless one is running, a finished drink is still awaiting pickup,
// or the next order's glass has not been confirmed.
func (q *Queue) Start() (Order, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.current != nil || q.awaitingPickup != nil || len(q.pending) == 0 || q.awaitingGlassLocked() != nil {
		return Order{}, false
	}
	o := q.pending[0]
	q.pending = q.pending[1:]
	q.glassReadyFor = ""
	o.StartedAt = time.Now()
	q.current = &o
	q.saveOrReportLocked()
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
	q.saveOrReportLocked()
}

// awaitingGlassLocked is the next order when it names a glass nobody has confirmed is at the station.
func (q *Queue) awaitingGlassLocked() *Order {
	if q.current != nil || q.awaitingPickup != nil || len(q.pending) == 0 {
		return nil
	}
	next := &q.pending[0]
	if next.Glass == "" || next.ID == q.glassReadyFor {
		return nil
	}
	return next
}

// Release confirms the station is ready: the finished drink was taken, and the glass for the next order is in place.
func (q *Queue) Release() bool {
	q.mu.Lock()
	released := q.awaitingPickup != nil || q.awaitingGlassLocked() != nil
	if released {
		q.awaitingPickup = nil
		if len(q.pending) > 0 {
			q.glassReadyFor = q.pending[0].ID
		}
		q.saveOrReportLocked()
	}
	q.mu.Unlock()
	if released {
		q.signal()
	}
	return released
}

// Remove drops a pending order that has not started yet. It is kept if the
// removal cannot be saved, so a restart never brings back an order that was reported cancelled.
func (q *Queue) Remove(id string) (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, o := range q.pending {
		if o.ID == id {
			before := q.pending
			q.pending = append(q.pending[:i:i], q.pending[i+1:]...)
			if err := q.saveLocked(); err != nil {
				q.pending = before
				return false, fmt.Errorf("save order queue: %w", err)
			}
			return true, nil
		}
	}
	return false, nil
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
	AwaitingGlass  *Order  `json:"awaiting_glass,omitempty"`
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
	if next := q.awaitingGlassLocked(); next != nil {
		waiting := *next
		s.AwaitingGlass = &waiting
	}
	return s
}
