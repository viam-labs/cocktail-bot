package bartender

import (
	"testing"

	"go.viam.com/test"
)

func TestParseDispenseIce(t *testing.T) {
	cases := []struct {
		name     string
		raw      any
		station  string
		dwellMs  int
		wantErr  bool
		errMatch string
	}{
		{"ok", map[string]any{"station": "ice-station", "dwell_ms": 3000.0}, "ice-station", 3000, false, ""},
		{"ok int", map[string]any{"station": "s", "dwell_ms": 100}, "s", 100, false, ""},
		{"zero dwell", map[string]any{"station": "s", "dwell_ms": 0.0}, "s", 0, false, ""},
		{"not object", "ice-station", "", 0, true, "object"},
		{"missing station", map[string]any{"dwell_ms": 100}, "", 0, true, "station"},
		{"missing dwell", map[string]any{"station": "s"}, "", 0, true, "dwell_ms"},
		{"negative dwell", map[string]any{"station": "s", "dwell_ms": -1.0}, "", 0, true, ">= 0"},
		{"string dwell", map[string]any{"station": "s", "dwell_ms": "3000"}, "", 0, true, "must be a number"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			station, dwellMs, err := parseDispenseIce(tc.raw)
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
