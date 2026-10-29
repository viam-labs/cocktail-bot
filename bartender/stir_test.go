package bartender

import (
	"testing"

	"go.viam.com/test"
)

func TestParseStir(t *testing.T) {
	cases := []struct {
		name     string
		raw      any
		station  string
		dwellMs  int
		wantErr  bool
		errMatch string
	}{
		{"ok", map[string]any{"station": "stir-station", "dwell_ms": 20000.0}, "stir-station", 20000, false, ""},
		{"ok int", map[string]any{"station": "s", "dwell_ms": 100}, "s", 100, false, ""},
		{"zero dwell", map[string]any{"station": "s", "dwell_ms": 0.0}, "s", 0, false, ""},
		{"not object", "stir-station", "", 0, true, "object"},
		{"missing station", map[string]any{"dwell_ms": 100}, "", 0, true, "station"},
		{"missing dwell", map[string]any{"station": "s"}, "", 0, true, "dwell_ms"},
		{"negative dwell", map[string]any{"station": "s", "dwell_ms": -1.0}, "", 0, true, ">= 0"},
		{"string dwell", map[string]any{"station": "s", "dwell_ms": "100"}, "", 0, true, "must be a number"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			station, dwellMs, err := parseStir(tc.raw)
			if tc.wantErr {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.errMatch)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, station, test.ShouldEqual, tc.station)
			test.That(t, dwellMs, test.ShouldEqual, tc.dwellMs)
		})
	}
}

func TestConfigValidateHeldStirrerGeometry(t *testing.T) {
	cfg := &Config{ArmName: "arm", GripperName: "gripper", PoseSwitcherNames: []string{"stir"}}
	cfg.HeldStirrerGeometry = &HeldObjectGeometry{Type: "cylinder", RadiusMM: 30, LengthMM: 267, ZOffsetMM: 111.5}
	_, _, err := cfg.Validate("svc")
	test.That(t, err, test.ShouldBeNil)

	cfg.HeldStirrerGeometry = &HeldObjectGeometry{Type: "box", RadiusMM: 30, LengthMM: 267}
	_, _, err = cfg.Validate("svc")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "held_stirrer_geometry")
}

func TestRecipeDefaultsStir(t *testing.T) {
	d := &RecipeDefaults{MixDwellMs: 6000}
	test.That(t, d.stirStation(), test.ShouldEqual, "stir")
	test.That(t, d.stirDwellMs(), test.ShouldEqual, 6000)

	d.StirStation = "stir-2"
	d.StirDwellMs = 20000
	test.That(t, d.stirStation(), test.ShouldEqual, "stir-2")
	test.That(t, d.stirDwellMs(), test.ShouldEqual, 20000)
}
