package bartender

import (
	"context"
	"testing"

	"go.viam.com/test"
)

func TestParseCleanShaker(t *testing.T) {
	cases := []struct {
		name     string
		raw      any
		station  string
		dwellMs  int
		drainMs  int
		wantErr  bool
		errMatch string
	}{
		{"ok", map[string]any{"station": "shaker-cleaner", "dwell_ms": 3000.0}, "shaker-cleaner", 3000, 0, false, ""},
		{"ok with drain", map[string]any{"station": "c", "dwell_ms": 100, "drain_ms": 500.0}, "c", 100, 500, false, ""},
		{"ok int", map[string]any{"station": "c", "dwell_ms": 100}, "c", 100, 0, false, ""},
		{"zero dwell", map[string]any{"station": "c", "dwell_ms": 0.0}, "c", 0, 0, false, ""},
		{"not object", "shaker-cleaner", "", 0, 0, true, "object"},
		{"missing station", map[string]any{"dwell_ms": 100}, "", 0, 0, true, "station"},
		{"missing dwell", map[string]any{"station": "c"}, "", 0, 0, true, "dwell_ms"},
		{"negative dwell", map[string]any{"station": "c", "dwell_ms": -1.0}, "", 0, 0, true, ">= 0"},
		{"string dwell", map[string]any{"station": "c", "dwell_ms": "100"}, "", 0, 0, true, "must be a number"},
		{"negative drain", map[string]any{"station": "c", "dwell_ms": 100, "drain_ms": -1.0}, "", 0, 0, true, "drain_ms"},
		{"string drain", map[string]any{"station": "c", "dwell_ms": 100, "drain_ms": "x"}, "", 0, 0, true, "drain_ms"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			station, dwellMs, drainMs, err := parseCleanShaker(tc.raw)
			if tc.wantErr {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.errMatch)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, station, test.ShouldEqual, tc.station)
			test.That(t, dwellMs, test.ShouldEqual, tc.dwellMs)
			test.That(t, drainMs, test.ShouldEqual, tc.drainMs)
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

func TestCleanShakerRequiresHeldShaker(t *testing.T) {
	b := &bartender{cfg: &Config{}}
	err := b.cleanShaker(context.Background(), "shaker-cleaner", 0, 0)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "shaker must already be in the gripper")
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
