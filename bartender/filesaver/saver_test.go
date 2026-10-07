package filesaver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"go.viam.com/rdk/logging"
	"go.viam.com/test"
)

func writeString(content string) SaveFunc {
	return func(path string) error {
		return os.WriteFile(path, []byte(content), 0o644)
	}
}

func TestNilSaverNoOp(t *testing.T) {
	var s *Saver
	s.SaveAsync(context.Background(), "o1", "f.txt", writeString("x"))
	test.That(t, s.Close(), test.ShouldBeNil)
}

func TestNewEmptyDirReturnsNil(t *testing.T) {
	s := New("", logging.NewTestLogger(t))
	test.That(t, s, test.ShouldBeNil)
}

func TestSaveAsyncWritesToTaggedDir(t *testing.T) {
	dir := t.TempDir()
	s := New(dir, logging.NewTestLogger(t))
	t.Cleanup(func() { _ = s.Close() })

	s.SaveAsync(context.Background(), "order-abc", "plan.json", writeString(`{"k":"v"}`))
	test.That(t, s.Close(), test.ShouldBeNil)

	tagDir := filepath.Join(dir, "tag=order-abc")
	entries, err := os.ReadDir(tagDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(entries), test.ShouldEqual, 1)
	test.That(t, strings.HasSuffix(entries[0].Name(), "_plan.json"), test.ShouldBeTrue)

	body, err := os.ReadFile(filepath.Join(tagDir, entries[0].Name()))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(body), test.ShouldEqual, `{"k":"v"}`)
}

func TestSaveAsyncSeparatesByOrderID(t *testing.T) {
	dir := t.TempDir()
	s := New(dir, logging.NewTestLogger(t))
	t.Cleanup(func() { _ = s.Close() })

	s.SaveAsync(context.Background(), "o1", "a.txt", writeString("1"))
	s.SaveAsync(context.Background(), "o2", "b.txt", writeString("2"))
	test.That(t, s.Close(), test.ShouldBeNil)

	for _, id := range []string{"o1", "o2"} {
		entries, err := os.ReadDir(filepath.Join(dir, "tag="+id))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(entries), test.ShouldEqual, 1)
	}
}

func TestSaveAsyncEmptyOrderIDDropped(t *testing.T) {
	dir := t.TempDir()
	s := New(dir, logging.NewTestLogger(t))
	t.Cleanup(func() { _ = s.Close() })

	var called atomic.Int32
	s.SaveAsync(context.Background(), "", "f.txt", func(string) error {
		called.Add(1)
		return nil
	})
	test.That(t, s.Close(), test.ShouldBeNil)
	test.That(t, int(called.Load()), test.ShouldEqual, 0)
}

func TestSaveAsyncCancelledCtxDropped(t *testing.T) {
	dir := t.TempDir()
	s := New(dir, logging.NewTestLogger(t))
	t.Cleanup(func() { _ = s.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var called atomic.Int32
	s.SaveAsync(ctx, "o1", "f.txt", func(string) error {
		called.Add(1)
		return nil
	})
	test.That(t, s.Close(), test.ShouldBeNil)
	test.That(t, int(called.Load()), test.ShouldEqual, 0)
}
