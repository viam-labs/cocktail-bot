package bartender

import (
	"sync"
	"testing"
	"time"

	"go.viam.com/rdk/logging"
	"go.viam.com/test"

	"github.com/viam-labs/cocktail-bot/bartender/order"
)

type mockSink struct {
	mu       sync.Mutex
	readings []order.Reading
}

func (m *mockSink) PushOrderReading(r order.Reading) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readings = append(m.readings, r)
}

func (m *mockSink) snapshot() []order.Reading {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]order.Reading(nil), m.readings...)
}

func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", msg)
}

func newTestBartender(t *testing.T, sink orderSensorSink) *bartender {
	return &bartender{
		logger:    logging.NewTestLogger(t),
		queue:     order.NewQueue(),
		orderSink: sink,
		queueStop: make(chan struct{}),
	}
}

func withFastPhases(t *testing.T) {
	t.Helper()
	prev := phaseDuration
	phaseDuration = time.Millisecond
	t.Cleanup(func() { phaseDuration = prev })
}

func TestProcessQueueRunsOrderAndPushesReading(t *testing.T) {
	withFastPhases(t)

	sink := &mockSink{}
	b := newTestBartender(t, sink)
	go b.processQueue()
	defer close(b.queueStop)

	o := order.NewOrder("negroni")
	b.queue.Enqueue(o)

	waitFor(t, func() bool { return len(sink.snapshot()) == 1 }, "one reading pushed")

	got := sink.snapshot()[0]
	test.That(t, got.OrderID, test.ShouldEqual, o.ID)
	test.That(t, got.Drink, test.ShouldEqual, "negroni")
	test.That(t, got.Status, test.ShouldEqual, order.StatusSucceeded)
	test.That(t, got.ErrMessage, test.ShouldEqual, "")
	test.That(t, got.FailedStep, test.ShouldEqual, "")
	test.That(t, got.StartedAt.IsZero(), test.ShouldBeFalse)
	test.That(t, got.EndedAt.Before(got.StartedAt), test.ShouldBeFalse)
}

func TestProcessQueueRunsOrdersInOrder(t *testing.T) {
	withFastPhases(t)

	sink := &mockSink{}
	b := newTestBartender(t, sink)
	go b.processQueue()
	defer close(b.queueStop)

	first := order.NewOrder("negroni")
	second := order.NewOrder("martini")
	b.queue.Enqueue(first)
	b.queue.Enqueue(second)

	waitFor(t, func() bool { return len(sink.snapshot()) == 2 }, "two readings pushed")

	got := sink.snapshot()
	test.That(t, got[0].OrderID, test.ShouldEqual, first.ID)
	test.That(t, got[1].OrderID, test.ShouldEqual, second.ID)
}

func TestProcessQueueSkipsPushWhenSinkNil(t *testing.T) {
	withFastPhases(t)

	b := newTestBartender(t, nil)
	go b.processQueue()
	defer close(b.queueStop)

	o := order.NewOrder("daiquiri")
	b.queue.Enqueue(o)

	waitFor(t, func() bool {
		st := b.queue.State()
		return !st.IsBusy && len(st.Recent) == 1
	}, "order completed without panic")
}
