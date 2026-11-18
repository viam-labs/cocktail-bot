package order

import (
	"errors"
	"testing"
	"time"

	"go.viam.com/test"
)

func TestEnqueueStartComplete(t *testing.T) {
	q := NewQueue()
	o := NewOrder("negroni")
	ahead := q.Enqueue(o)
	test.That(t, ahead, test.ShouldEqual, 0)

	got := q.State()
	test.That(t, got.Count, test.ShouldEqual, 1)
	test.That(t, got.IsBusy, test.ShouldBeFalse)

	popped, ok := q.Start()
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, popped.ID, test.ShouldEqual, o.ID)
	test.That(t, popped.StartedAt.IsZero(), test.ShouldBeFalse)

	got = q.State()
	test.That(t, got.IsBusy, test.ShouldBeTrue)
	test.That(t, got.Current, test.ShouldNotBeNil)
	test.That(t, got.Current.ID, test.ShouldEqual, o.ID)

	q.Complete(nil)

	st := q.State()
	test.That(t, st.IsBusy, test.ShouldBeFalse)
	test.That(t, st.Count, test.ShouldEqual, 0)
	test.That(t, len(st.Recent), test.ShouldEqual, 1)
	test.That(t, st.Recent[0].ID, test.ShouldEqual, o.ID)
	test.That(t, st.Recent[0].Error, test.ShouldEqual, "")
	test.That(t, st.Recent[0].CompletedAt.IsZero(), test.ShouldBeFalse)
	test.That(t, st.AwaitingPickup, test.ShouldNotBeNil)
	test.That(t, st.AwaitingPickup.ID, test.ShouldEqual, o.ID)
}

func TestEnqueueReportsOrdersAhead(t *testing.T) {
	q := NewQueue()
	q.Enqueue(NewOrder("a"))
	q.Start()
	test.That(t, q.Enqueue(NewOrder("b")), test.ShouldEqual, 1)
	test.That(t, q.Enqueue(NewOrder("c")), test.ShouldEqual, 2)
}

func TestStartEmpty(t *testing.T) {
	q := NewQueue()
	_, ok := q.Start()
	test.That(t, ok, test.ShouldBeFalse)
}

func TestStartWaitsForRunningOrder(t *testing.T) {
	q := NewQueue()
	q.Enqueue(NewOrder("a"))
	q.Enqueue(NewOrder("b"))
	q.Start()
	_, ok := q.Start()
	test.That(t, ok, test.ShouldBeFalse)
}

func TestCompleteHoldsUntilRelease(t *testing.T) {
	q := NewQueue()
	first := NewOrder("a")
	second := NewOrder("b")
	q.Enqueue(first)
	q.Enqueue(second)
	q.Start()
	q.Complete(nil)

	_, ok := q.Start()
	test.That(t, ok, test.ShouldBeFalse)

	test.That(t, q.Release(), test.ShouldBeTrue)
	test.That(t, q.State().AwaitingPickup, test.ShouldBeNil)
	test.That(t, q.Release(), test.ShouldBeFalse)

	next, ok := q.Start()
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, next.ID, test.ShouldEqual, second.ID)
}

func TestReleaseNotifies(t *testing.T) {
	q := NewQueue()
	q.Enqueue(NewOrder("a"))
	<-q.Notify()
	q.Start()
	q.Complete(nil)
	q.Release()
	select {
	case <-q.Notify():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("release did not notify")
	}
}

func TestCompleteRecordsError(t *testing.T) {
	q := NewQueue()
	q.Enqueue(NewOrder("a"))
	q.Start()
	q.Complete(errors.New("bottle empty"))
	st := q.State()
	test.That(t, st.Recent[0].Error, test.ShouldEqual, "bottle empty")
	test.That(t, st.AwaitingPickup.Error, test.ShouldEqual, "bottle empty")
}

func TestRemovePending(t *testing.T) {
	q := NewQueue()
	running := NewOrder("a")
	waiting := NewOrder("b")
	q.Enqueue(running)
	q.Enqueue(waiting)
	q.Start()

	test.That(t, q.Remove(running.ID), test.ShouldBeFalse)
	test.That(t, q.Remove(waiting.ID), test.ShouldBeTrue)
	test.That(t, q.Remove(waiting.ID), test.ShouldBeFalse)
	test.That(t, len(q.State().Queue), test.ShouldEqual, 0)
}

func TestSetStep(t *testing.T) {
	q := NewQueue()
	q.Enqueue(NewOrder("martini"))
	q.Start()
	q.SetStep("pouring gin")
	got := q.State()
	test.That(t, got.Current, test.ShouldNotBeNil)
	test.That(t, got.Current.RawStep, test.ShouldEqual, "pouring gin")
}

func TestRecentPruning(t *testing.T) {
	q := NewQueue()
	q.Enqueue(NewOrder("old"))
	q.Start()
	q.Complete(nil)
	q.recent[0].CompletedAt = time.Now().Add(-RecentDisplayDuration - time.Second)
	got := q.State()
	test.That(t, len(got.Recent), test.ShouldEqual, 0)
}

func TestNotifyFiresOnEnqueue(t *testing.T) {
	q := NewQueue()
	q.Enqueue(NewOrder("daiquiri"))
	fired := false
	select {
	case <-q.Notify():
		fired = true
	case <-time.After(100 * time.Millisecond):
	}
	test.That(t, fired, test.ShouldBeTrue)
}
