package bartender

import (
	"testing"

	"go.viam.com/test"
)

func TestParseDispenseIce(t *testing.T) {
	cases := []struct {
		name     string
		raw      any
		lever    string
		dwellMs  int
		wantErr  bool
		errMatch string
	}{
		{"ok", map[string]any{"lever": "ice-lever", "dwell_ms": 3000.0}, "ice-lever", 3000, false, ""},
		{"ok int", map[string]any{"lever": "l", "dwell_ms": 100}, "l", 100, false, ""},
		{"zero dwell", map[string]any{"lever": "l", "dwell_ms": 0.0}, "l", 0, false, ""},
		{"not object", "ice-lever", "", 0, true, "object"},
		{"missing lever", map[string]any{"dwell_ms": 100}, "", 0, true, "lever"},
		{"missing dwell", map[string]any{"lever": "l"}, "", 0, true, "dwell_ms"},
		{"negative dwell", map[string]any{"lever": "l", "dwell_ms": -1.0}, "", 0, true, ">= 0"},
		{"string dwell", map[string]any{"lever": "l", "dwell_ms": "3000"}, "", 0, true, "must be a number"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lever, dwellMs, err := parseDispenseIce(tc.raw)
			if tc.wantErr {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.errMatch)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, lever, test.ShouldEqual, tc.lever)
			test.That(t, dwellMs, test.ShouldEqual, tc.dwellMs)
		})
	}
}
