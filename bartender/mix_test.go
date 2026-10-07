package bartender

import (
	"testing"

	"go.viam.com/test"
)

func TestParseMix(t *testing.T) {
	cases := []struct {
		name     string
		raw      any
		station  string
		dwellMs  int
		wantErr  bool
		errMatch string
	}{
		{"ok", map[string]any{"station": "mixer", "dwell_ms": 10000.0}, "mixer", 10000, false, ""},
		{"ok int", map[string]any{"station": "m", "dwell_ms": 100}, "m", 100, false, ""},
		{"zero dwell", map[string]any{"station": "m", "dwell_ms": 0.0}, "m", 0, false, ""},
		{"not object", "mixer", "", 0, true, "object"},
		{"missing station", map[string]any{"dwell_ms": 100}, "", 0, true, "station"},
		{"missing dwell", map[string]any{"station": "m"}, "", 0, true, "dwell_ms"},
		{"negative dwell", map[string]any{"station": "m", "dwell_ms": -1.0}, "", 0, true, ">= 0"},
		{"string dwell", map[string]any{"station": "m", "dwell_ms": "100"}, "", 0, true, "must be a number"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			station, dwellMs, err := parseMix(tc.raw)
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
