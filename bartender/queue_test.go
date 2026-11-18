package bartender

import (
	"context"
	"errors"
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
	b := &bartender{
		logger:    logging.NewTestLogger(t),
		queue:     order.NewQueue(),
		orderSink: sink,
		queueStop: make(chan struct{}),
		queueDone: make(chan struct{}),
	}
	b.brew = func(context.Context, drinkPlan) error { return nil }
	return b
}

func newCocktailOrder(drink string) order.Order {
	o := order.NewOrder(drink)
	o.Payload = cocktailJob{plan: drinkPlan{title: drink}, quantity: 1}
	return o
}

func TestProcessQueueRunsOrderAndPushesReading(t *testing.T) {
	sink := &mockSink{}
	b := newTestBartender(t, sink)
	go b.processQueue()
	defer close(b.queueStop)

	o := newCocktailOrder("negroni")
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

	snap := b.status.snapshot()
	test.That(t, snap.busy, test.ShouldBeFalse)
	test.That(t, snap.lastOutcome.Drink, test.ShouldEqual, "negroni")
}

func TestProcessQueueRunsQuantityTimes(t *testing.T) {
	b := newTestBartender(t, nil)
	var mu sync.Mutex
	brews := 0
	b.brew = func(context.Context, drinkPlan) error {
		mu.Lock()
		brews++
		mu.Unlock()
		return nil
	}
	go b.processQueue()
	defer close(b.queueStop)

	o := newCocktailOrder("negroni")
	o.Payload = cocktailJob{quantity: 3}
	b.queue.Enqueue(o)

	waitFor(t, func() bool { return b.queue.State().AwaitingPickup != nil }, "order finished")
	mu.Lock()
	defer mu.Unlock()
	test.That(t, brews, test.ShouldEqual, 3)
}

func TestProcessQueueHoldsNextOrderUntilStartNext(t *testing.T) {
	sink := &mockSink{}
	b := newTestBartender(t, sink)
	go b.processQueue()
	defer close(b.queueStop)

	first := newCocktailOrder("negroni")
	second := newCocktailOrder("martini")
	b.queue.Enqueue(first)
	b.queue.Enqueue(second)

	waitFor(t, func() bool { return len(sink.snapshot()) == 1 }, "first order made")
	time.Sleep(50 * time.Millisecond)
	test.That(t, len(sink.snapshot()), test.ShouldEqual, 1)
	st := b.queue.State()
	test.That(t, st.AwaitingPickup.ID, test.ShouldEqual, first.ID)
	test.That(t, len(st.Queue), test.ShouldEqual, 1)

	resp, err := b.DoCommand(context.Background(), map[string]any{"start_next": true})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["released"], test.ShouldEqual, true)

	waitFor(t, func() bool { return len(sink.snapshot()) == 2 }, "second order made")
	test.That(t, sink.snapshot()[1].OrderID, test.ShouldEqual, second.ID)
}

func TestFailedOrderRecordsErrorOnQueue(t *testing.T) {
	sink := &mockSink{}
	b := newTestBartender(t, sink)
	b.brew = func(ctx context.Context, _ drinkPlan) error {
		b.setPhase("Pouring vodka")
		return errors.New("bottle empty")
	}
	go b.processQueue()
	defer close(b.queueStop)

	o := newCocktailOrder("martini")
	b.queue.Enqueue(o)

	waitFor(t, func() bool { return len(sink.snapshot()) == 1 }, "reading pushed")
	got := sink.snapshot()[0]
	test.That(t, got.Status, test.ShouldEqual, order.StatusFailed)
	test.That(t, got.FailedStep, test.ShouldEqual, "Pouring vodka")
	test.That(t, b.queue.State().AwaitingPickup.Error, test.ShouldEqual, "bottle empty")
}

func TestManualCommandRefusedWhileDrinkRuns(t *testing.T) {
	b := newTestBartender(t, nil)
	started := make(chan struct{})
	b.brew = func(ctx context.Context, _ drinkPlan) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}
	go b.processQueue()
	defer close(b.queueStop)

	b.queue.Enqueue(newCocktailOrder("negroni"))
	<-started

	_, err := b.DoCommand(context.Background(), map[string]any{"execute_action": "home"})
	test.That(t, errors.Is(err, errBusy), test.ShouldBeTrue)

	b.cancelRunning()
	waitFor(t, func() bool { return b.queue.State().AwaitingPickup != nil }, "cancelled order completed")
	test.That(t, b.queue.State().AwaitingPickup.Error, test.ShouldContainSubstring, "canceled")
}

func TestQueuedOrderWaitsForManualCommand(t *testing.T) {
	sink := &mockSink{}
	b := newTestBartender(t, sink)
	go b.processQueue()
	defer close(b.queueStop)

	_, done, err := b.claim(context.Background())
	test.That(t, err, test.ShouldBeNil)
	b.queue.Enqueue(newCocktailOrder("negroni"))
	time.Sleep(50 * time.Millisecond)
	test.That(t, len(sink.snapshot()), test.ShouldEqual, 0)

	done()
	waitFor(t, func() bool { return len(sink.snapshot()) == 1 }, "order ran after manual command")
}

func TestCancelOrderRemovesPending(t *testing.T) {
	b := newTestBartender(t, nil)
	o := newCocktailOrder("negroni")
	b.queue.Enqueue(o)

	resp, err := b.DoCommand(context.Background(), map[string]any{"cancel_order": map[string]any{"order_id": o.ID}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["removed"], test.ShouldEqual, true)
	test.That(t, b.queue.State().Count, test.ShouldEqual, 0)
}

func TestGetQueueShape(t *testing.T) {
	b := newTestBartender(t, nil)
	o := newCocktailOrder("negroni")
	o.Double = true
	b.queue.Enqueue(o)

	resp, err := b.DoCommand(context.Background(), map[string]any{"get_queue": true})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["count"], test.ShouldEqual, 1.0)
	test.That(t, resp["is_busy"], test.ShouldEqual, false)
	queue, ok := resp["queue"].([]any)
	test.That(t, ok, test.ShouldBeTrue)
	entry := queue[0].(map[string]any)
	test.That(t, entry["id"], test.ShouldEqual, o.ID)
	test.That(t, entry["double"], test.ShouldEqual, true)
	_, hasPayload := entry["Payload"]
	test.That(t, hasPayload, test.ShouldBeFalse)
	_, ok = entry["enqueued_at"].(string)
	test.That(t, ok, test.ShouldBeTrue)
}

type brewCounter struct {
	mu    sync.Mutex
	count int
}

func (c *brewCounter) brew(ctx context.Context, _ drinkPlan) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
	return nil
}

func (c *brewCounter) n() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

func TestOrderWaitingOnArmIsNotCurrentAndCancelHitsManualCommand(t *testing.T) {
	b := newTestBartender(t, nil)
	brews := &brewCounter{}
	b.brew = brews.brew
	go b.processQueue()
	defer close(b.queueStop)

	manualCtx, done, err := b.claim(context.Background())
	test.That(t, err, test.ShouldBeNil)
	o := newCocktailOrder("negroni")
	b.queue.Enqueue(o)
	time.Sleep(50 * time.Millisecond)

	st := b.queue.State()
	test.That(t, st.Current, test.ShouldBeNil)
	test.That(t, len(st.Queue), test.ShouldEqual, 1)

	test.That(t, b.cancelRunning(), test.ShouldBeTrue)
	test.That(t, manualCtx.Err(), test.ShouldNotBeNil)
	done()

	waitFor(t, func() bool { return b.queue.State().AwaitingPickup != nil }, "order ran after manual command")
	test.That(t, b.queue.State().AwaitingPickup.Error, test.ShouldEqual, "")
	test.That(t, brews.n(), test.ShouldEqual, 1)
}

func TestCloseDropsOrderWaitingOnArm(t *testing.T) {
	b := newTestBartender(t, nil)
	brews := &brewCounter{}
	b.brew = brews.brew
	go b.processQueue()

	manualCtx, done, err := b.claim(context.Background())
	test.That(t, err, test.ShouldBeNil)
	b.queue.Enqueue(newCocktailOrder("negroni"))
	time.Sleep(20 * time.Millisecond)

	closed := make(chan error, 1)
	go func() { closed <- b.Close(context.Background()) }()
	waitFor(t, func() bool { return manualCtx.Err() != nil }, "close cancelled the manual command")
	done()

	select {
	case err := <-closed:
		test.That(t, err, test.ShouldBeNil)
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return")
	}
	time.Sleep(20 * time.Millisecond)
	test.That(t, brews.n(), test.ShouldEqual, 0)
}

func TestCloseCancelsRunningOrderAndWaitsForWorker(t *testing.T) {
	b := newTestBartender(t, nil)
	started := make(chan struct{})
	var brewErr error
	b.brew = func(ctx context.Context, _ drinkPlan) error {
		close(started)
		<-ctx.Done()
		time.Sleep(20 * time.Millisecond)
		brewErr = ctx.Err()
		return brewErr
	}
	go b.processQueue()
	b.queue.Enqueue(newCocktailOrder("negroni"))
	<-started

	test.That(t, b.Close(context.Background()), test.ShouldBeNil)
	select {
	case <-b.queueDone:
	default:
		t.Fatal("Close returned before the queue worker exited")
	}
	test.That(t, errors.Is(brewErr, context.Canceled), test.ShouldBeTrue)
}
