package order

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"
)

func openTestQueue(t *testing.T, path string) (*Queue, *Order) {
	t.Helper()
	q, interrupted, err := OpenQueue(NewFileStore(path), func(err error) { t.Errorf("save: %v", err) })
	test.That(t, err, test.ShouldBeNil)
	return q, interrupted
}

func TestFileStoreMissingFileIsEmptyQueue(t *testing.T) {
	q, interrupted := openTestQueue(t, filepath.Join(t.TempDir(), "queue.json"))
	test.That(t, interrupted, test.ShouldBeNil)
	test.That(t, q.State().Count, test.ShouldEqual, 0)
}

func TestQueueSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	q, _ := openTestQueue(t, path)

	done := NewOrder("done")
	waiting := NewOrder("waiting")
	waiting.Payload = json.RawMessage(`{"title":"waiting"}`)
	_, err := q.Enqueue(done)
	test.That(t, err, test.ShouldBeNil)
	_, err = q.Enqueue(waiting)
	test.That(t, err, test.ShouldBeNil)
	q.Start()
	q.Complete(nil)

	reopened, interrupted := openTestQueue(t, path)
	test.That(t, interrupted, test.ShouldBeNil)
	st := reopened.State()
	test.That(t, st.AwaitingPickup.ID, test.ShouldEqual, done.ID)
	test.That(t, len(st.Queue), test.ShouldEqual, 1)
	test.That(t, st.Queue[0].ID, test.ShouldEqual, waiting.ID)
	test.That(t, len(st.Recent), test.ShouldEqual, 1)

	reopened.Release()
	next, ok := reopened.Start()
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, string(next.Payload), test.ShouldEqual, `{"title":"waiting"}`)
}

func TestRestartMidDrinkMarksOrderInterruptedAndHolds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	q, _ := openTestQueue(t, path)
	running := NewOrder("running")
	next := NewOrder("next")
	mustEnqueue(t, q, running)
	mustEnqueue(t, q, next)
	q.Start()

	reopened, interrupted := openTestQueue(t, path)
	test.That(t, interrupted, test.ShouldNotBeNil)
	test.That(t, interrupted.ID, test.ShouldEqual, running.ID)
	test.That(t, interrupted.Error, test.ShouldEqual, ErrInterrupted)

	st := reopened.State()
	test.That(t, st.Current, test.ShouldBeNil)
	test.That(t, st.AwaitingPickup.ID, test.ShouldEqual, running.ID)
	_, ok := reopened.Start()
	test.That(t, ok, test.ShouldBeFalse)

	again, interruptedAgain := openTestQueue(t, path)
	test.That(t, interruptedAgain, test.ShouldBeNil)
	test.That(t, again.State().AwaitingPickup.ID, test.ShouldEqual, running.ID)
}

func TestReopenSignalsWorkerForPendingOrders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	q, _ := openTestQueue(t, path)
	mustEnqueue(t, q, NewOrder("a"))

	reopened, _ := openTestQueue(t, path)
	select {
	case <-reopened.Notify():
	default:
		t.Fatal("reopened queue with pending orders did not signal the worker")
	}
}

func TestCorruptQueueFileIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	test.That(t, os.WriteFile(path, []byte("{not json"), 0o644), test.ShouldBeNil)
	_, _, err := OpenQueue(NewFileStore(path), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "queue.json")
}

type failingStore struct{ fail bool }

func (f *failingStore) Load() (Snapshot, error) { return Snapshot{}, nil }
func (f *failingStore) Save(Snapshot) error {
	if f.fail {
		return errors.New("disk full")
	}
	return nil
}

func TestEnqueueAndRemoveRollBackWhenSaveFails(t *testing.T) {
	store := &failingStore{}
	q, _, err := OpenQueue(store, nil)
	test.That(t, err, test.ShouldBeNil)
	kept := NewOrder("kept")
	mustEnqueue(t, q, kept)

	store.fail = true
	_, err = q.Enqueue(NewOrder("rejected"))
	test.That(t, err, test.ShouldNotBeNil)
	removed, err := q.Remove(kept.ID)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, removed, test.ShouldBeFalse)

	st := q.State()
	test.That(t, len(st.Queue), test.ShouldEqual, 1)
	test.That(t, st.Queue[0].ID, test.ShouldEqual, kept.ID)
}

func TestPayloadIsHiddenFromState(t *testing.T) {
	q := NewQueue()
	o := NewOrder("a")
	o.Payload = json.RawMessage(`{"secret":true}`)
	mustEnqueue(t, q, o)
	raw, err := json.Marshal(q.State())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(raw), test.ShouldNotContainSubstring, "secret")
}
