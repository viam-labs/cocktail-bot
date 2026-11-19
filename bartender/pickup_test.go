package bartender

import (
	"context"
	"testing"
	"time"

	toggleswitch "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/testutils/inject"
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

// returnTestBartender holds a bottle for slot "back", whose hover names "front" as its
// blocker. Every "back" pose is the arm's current joints, so putting the bottle back is trivial.
func returnTestBartender(t *testing.T, asked map[string][]any) (*bartender, toggleswitch.Switch) {
	t.Helper()
	b, _, _ := serveTestBartender(t, nil)
	b.cfg.ParkingSwitchName = "parking"
	g := inject.NewGripper("gripper")
	g.OpenFunc = func(context.Context, map[string]any) error { return nil }
	b.gripper = g
	back := inject.NewSwitch("back")
	back.DoFunc = func(_ context.Context, cmd map[string]any) (map[string]any, error) {
		resp := map[string]any{
			"joint_positions_degs": []any{0.0, -90.0, 90.0, -90.0, -90.0, 0.0},
			"component_name":       "arm",
			"reference_frame":      "world",
		}
		if cmd["get_pose_by_name"] == poseHover {
			resp["blocked_by"] = "front"
		}
		return resp, nil
	}
	b.poseSwitches = []toggleswitch.Switch{back, recordingPoseSwitch("front", asked), recordingPoseSwitch("parking", asked)}
	return b, back
}

func TestReturnBottleRestoresOnlyAParkedBlocker(t *testing.T) {
	t.Run("blocker was never parked", func(t *testing.T) {
		asked := map[string][]any{}
		b, back := returnTestBartender(t, asked)
		test.That(t, b.returnBottle(context.Background(), back, ""), test.ShouldBeNil)
		test.That(t, asked, test.ShouldBeEmpty)
	})

	t.Run("blocker was parked", func(t *testing.T) {
		asked := map[string][]any{}
		b, back := returnTestBartender(t, asked)
		err := b.returnBottle(context.Background(), back, "front")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "retrieve blocker from parking")
		test.That(t, asked["parking"], test.ShouldResemble, []any{poseHover})
	})
}
