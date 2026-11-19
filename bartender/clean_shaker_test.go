package bartender

import (
	"context"
	"errors"
	"testing"
	"time"

	toggleswitch "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
)

func TestParseCleanShaker(t *testing.T) {
	cases := []struct {
		name     string
		raw      any
		want     cleanShakerRequest
		errMatch string
	}{
		{"ok", map[string]any{"station": "shaker-cleaner", "dwell_ms": 3000.0}, cleanShakerRequest{station: "shaker-cleaner", dwellMs: 3000}, ""},
		{"ok with drain", map[string]any{"station": "c", "dwell_ms": 100, "drain_ms": 500.0}, cleanShakerRequest{station: "c", dwellMs: 100, drainMs: 500}, ""},
		{"ok with power-on delay", map[string]any{"station": "c", "dwell_ms": 100, "power_on_delay_ms": 750.0}, cleanShakerRequest{station: "c", powerOnDelayMs: 750, dwellMs: 100}, ""},
		{"ok int", map[string]any{"station": "c", "dwell_ms": 100}, cleanShakerRequest{station: "c", dwellMs: 100}, ""},
		{"zero dwell", map[string]any{"station": "c", "dwell_ms": 0.0}, cleanShakerRequest{station: "c"}, ""},
		{"not object", "shaker-cleaner", cleanShakerRequest{}, "object"},
		{"missing station", map[string]any{"dwell_ms": 100}, cleanShakerRequest{}, "station"},
		{"missing dwell", map[string]any{"station": "c"}, cleanShakerRequest{}, "dwell_ms"},
		{"negative dwell", map[string]any{"station": "c", "dwell_ms": -1.0}, cleanShakerRequest{}, ">= 0"},
		{"string dwell", map[string]any{"station": "c", "dwell_ms": "100"}, cleanShakerRequest{}, "must be a number"},
		{"negative drain", map[string]any{"station": "c", "dwell_ms": 100, "drain_ms": -1.0}, cleanShakerRequest{}, "drain_ms"},
		{"string drain", map[string]any{"station": "c", "dwell_ms": 100, "drain_ms": "x"}, cleanShakerRequest{}, "drain_ms"},
		{"negative power-on delay", map[string]any{"station": "c", "dwell_ms": 100, "power_on_delay_ms": -1.0}, cleanShakerRequest{}, "power_on_delay_ms"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := parseCleanShaker(tc.raw)
			if tc.errMatch != "" {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.errMatch)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, req, test.ShouldResemble, tc.want)
		})
	}
}

func TestConfigValidateShakerCleaning(t *testing.T) {
	base := func(sc *ShakerCleaning) *Config {
		return &Config{
			ArmName:           "arm",
			GripperName:       "gripper",
			PoseSwitcherNames: []string{"serving-glass-center", "shaker-cleaner"},
			ShakerCleaning:    sc,
		}
	}
	cases := []struct {
		name     string
		sc       *ShakerCleaning
		errMatch string
	}{
		{"unset", nil, ""},
		{"disabled without station", &ShakerCleaning{}, ""},
		{"enabled", &ShakerCleaning{Enabled: true, Station: "shaker-cleaner", DwellMs: 3000}, ""},
		{"enabled without station", &ShakerCleaning{Enabled: true}, "shaker_cleaning.station is required"},
		{"station not a switcher", &ShakerCleaning{Enabled: true, Station: "nope"}, "not in pose_switcher_names"},
		{"negative dwell", &ShakerCleaning{Enabled: true, Station: "shaker-cleaner", DwellMs: -1}, "dwell_ms must be >= 0"},
		{"negative drain", &ShakerCleaning{Enabled: true, Station: "shaker-cleaner", DrainMs: -1}, "drain_ms must be >= 0"},
		{"negative power-on delay", &ShakerCleaning{Enabled: true, Station: "shaker-cleaner", PowerOnDelayMs: -1}, "power_on_delay_ms must be >= 0"},
		{"bad held geometry while disabled", &ShakerCleaning{HeldShakerGeometry: &HeldObjectGeometry{Type: "box"}}, "shaker_cleaning.held_shaker_geometry.type"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := base(tc.sc).Validate("svc")
			if tc.errMatch == "" {
				test.That(t, err, test.ShouldBeNil)
				return
			}
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.errMatch)
		})
	}
}

func TestCleanShakerRequiresEmptyGripper(t *testing.T) {
	b := &bartender{cfg: &Config{}, heldGeomFrame: referenceframe.NewZeroStaticFrame(heldObjectFrameName)}
	err := b.cleanShaker(context.Background(), cleanShakerRequest{station: "shaker-cleaner"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "gripper must be empty")
}

func recordingPoseSwitch(name string, asked map[string][]any) *inject.Switch {
	sw := inject.NewSwitch(name)
	sw.DoFunc = func(_ context.Context, cmd map[string]any) (map[string]any, error) {
		asked[name] = append(asked[name], cmd["get_pose_by_name"])
		return nil, errors.New("stop before motion")
	}
	return sw
}

func TestCleanShakerStartsWithCleanerGrab(t *testing.T) {
	for _, tc := range []struct {
		name     string
		prestage string
		want     string
	}{
		{"no prestage", "", poseCleanerGrabHover},
		{"prestage", "cleaner-prestage", "cleaner-prestage"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			asked := map[string][]any{}
			b := stationTestBartender(t, "glass-center")
			b.cfg.ShakerCleaning = &ShakerCleaning{Station: "shaker-cleaner", PrestagePose: tc.prestage}
			b.poseSwitches = []toggleswitch.Switch{recordingPoseSwitch("shaker-cleaner", asked)}

			err := b.cleanShaker(context.Background(), cleanShakerRequest{station: "shaker-cleaner"})
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, asked["shaker-cleaner"], test.ShouldResemble, []any{tc.want})
		})
	}
}

func TestPutShakerAwayReturnsHomeBeforeCleaning(t *testing.T) {
	asked := map[string][]any{}
	b := stationTestBartender(t, "glass-center")
	b.cfg.ShakerCleaning = &ShakerCleaning{Enabled: true, Station: "shaker-cleaner"}
	b.poseSwitches = []toggleswitch.Switch{
		recordingPoseSwitch("serving-glass-center", asked),
		recordingPoseSwitch("shaker-cleaner", asked),
	}

	err := b.putShakerAway(context.Background(), &RecipeDefaults{ServeStation: "serving-glass-center"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "return shaker home")
	test.That(t, asked["serving-glass-center"], test.ShouldResemble, []any{poseShakerHomeCarryHover})
	test.That(t, asked["shaker-cleaner"], test.ShouldBeNil)
}

func TestCleanerHeldShakerGeometryFallsBack(t *testing.T) {
	main := &HeldObjectGeometry{Type: "cylinder", RadiusMM: 38, LengthMM: 166}
	low := &HeldObjectGeometry{Type: "cylinder", RadiusMM: 38, LengthMM: 166, ZOffsetMM: 90}
	b := &bartender{cfg: &Config{HeldShakerGeometry: main}}
	test.That(t, b.cleanerHeldShakerGeometry(), test.ShouldEqual, main)

	b.cfg.ShakerCleaning = &ShakerCleaning{}
	test.That(t, b.cleanerHeldShakerGeometry(), test.ShouldEqual, main)

	b.cfg.ShakerCleaning.HeldShakerGeometry = low
	test.That(t, b.cleanerHeldShakerGeometry(), test.ShouldEqual, low)
}

func TestCleanerAllowedUsesConfiguredObstacle(t *testing.T) {
	b := &bartender{cfg: &Config{ArmName: "arm", GripperName: "gripper"}}
	test.That(t, b.cleanerAllowed(), test.ShouldBeNil)

	b.cfg.ShakerCleaning = &ShakerCleaning{Enabled: true, Station: "shaker-cleaner"}
	test.That(t, b.cleanerAllowed(), test.ShouldBeNil)

	b.cfg.ShakerCleaning.Obstacle = "cleaner-obstacle"
	allowed := b.cleanerAllowed()
	test.That(t, allowed, test.ShouldContain, AllowedCollision{Frame1: heldObjectFrameName, Frame2: "cleaner-obstacle"})
	test.That(t, allowed, test.ShouldContain, AllowedCollision{Frame1: "gripper:claws", Frame2: "cleaner-obstacle"})
}

func TestRinseTurnsCleanerOnOnlyAfterPowerOnDelay(t *testing.T) {
	sw := newRecordingSwitch(nil)
	b := newCleanerPowerBartender(t, "cleaner-power", sw, "")
	start := time.Now()
	var onAfter time.Duration
	record := sw.SetPositionFunc
	sw.SetPositionFunc = func(ctx context.Context, position uint32, extra map[string]any) error {
		if position == 1 {
			onAfter = time.Since(start)
		}
		return record(ctx, position, extra)
	}

	err := b.rinse(context.Background(), cleanShakerRequest{powerOnDelayMs: 50})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sw.calls(), test.ShouldResemble, []uint32{1, 0})
	test.That(t, onAfter, test.ShouldBeGreaterThanOrEqualTo, 50*time.Millisecond)
}

func TestRinseCancelledDuringPowerOnDelayNeverTurnsCleanerOn(t *testing.T) {
	sw := newRecordingSwitch(nil)
	b := newCleanerPowerBartender(t, "cleaner-power", sw, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := b.rinse(ctx, cleanShakerRequest{powerOnDelayMs: 1000})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "power-on delay")
	test.That(t, sw.calls(), test.ShouldBeEmpty)
}
