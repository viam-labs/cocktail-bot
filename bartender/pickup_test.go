package bartender

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"
)

func TestSleepCtxCompletes(t *testing.T) {
	err := sleepCtx(context.Background(), 5*time.Millisecond)
	test.That(t, err, test.ShouldBeNil)
}

func TestSleepCtxCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := sleepCtx(ctx, time.Hour)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestParsePickupPourReturn(t *testing.T) {
	cases := []struct {
		name     string
		raw      any
		bottle   string
		pourMs   int
		wantErr  bool
		errMatch string
	}{
		{"ok", map[string]any{"bottle": "bottle-gin", "pour_ms": 2000.0}, "bottle-gin", 2000, false, ""},
		{"ok int", map[string]any{"bottle": "b", "pour_ms": 100}, "b", 100, false, ""},
		{"not object", "bottle-gin", "", 0, true, "object"},
		{"missing bottle", map[string]any{"pour_ms": 100}, "", 0, true, "bottle"},
		{"missing pour_ms", map[string]any{"bottle": "b"}, "", 0, true, "pour_ms"},
		{"negative pour_ms", map[string]any{"bottle": "b", "pour_ms": -1.0}, "", 0, true, ">= 0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bottle, pourMs, err := parsePickupPourReturn(tc.raw)
			if tc.wantErr {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.errMatch)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, bottle, test.ShouldEqual, tc.bottle)
			test.That(t, pourMs, test.ShouldEqual, tc.pourMs)
		})
	}
}
