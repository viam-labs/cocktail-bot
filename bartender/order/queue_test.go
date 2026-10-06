package order

import (
	"testing"
	"time"

	"go.viam.com/test"
)

func TestEnqueueStartComplete(t *testing.T) {
	q := NewQueue()
	o := NewOrder("negroni")
	q.Enqueue(o)

	got := q.State()
	test.That(t, got.Count, test.ShouldEqual, 1)
	test.That(t, got.IsBusy, test.ShouldBeFalse)

	popped, ok := q.Start()
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, popped.ID, test.ShouldEqual, o.ID)

	got = q.State()
	test.That(t, got.IsBusy, test.ShouldBeTrue)
	test.That(t, got.Current, test.ShouldNotBeNil)
	test.That(t, got.Current.ID, test.ShouldEqual, o.ID)

	q.Complete()

	st := q.State()
	test.That(t, st.IsBusy, test.ShouldBeFalse)
	test.That(t, st.Count, test.ShouldEqual, 0)
	test.That(t, len(st.Recent), test.ShouldEqual, 1)
	test.That(t, st.Recent[0].ID, test.ShouldEqual, o.ID)
	test.That(t, st.Recent[0].CompletedAt.IsZero(), test.ShouldBeFalse)
}

func TestStartEmpty(t *testing.T) {
	q := NewQueue()
	_, ok := q.Start()
	test.That(t, ok, test.ShouldBeFalse)
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
	q.Complete()
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
