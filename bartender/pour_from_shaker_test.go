package bartender

import (
	"testing"

	"go.viam.com/test"
)

func TestParsePourFromShaker(t *testing.T) {
	cases := []struct {
		name     string
		raw      any
		station  string
		pourMs   int
		wantErr  bool
		errMatch string
	}{
		{"ok", map[string]any{"station": "serving", "pour_ms": 5000.0}, "serving", 5000, false, ""},
		{"ok int", map[string]any{"station": "s", "pour_ms": 100}, "s", 100, false, ""},
		{"zero pour", map[string]any{"station": "s", "pour_ms": 0.0}, "s", 0, false, ""},
		{"not object", "serving", "", 0, true, "object"},
		{"missing station", map[string]any{"pour_ms": 100}, "", 0, true, "station"},
		{"missing pour", map[string]any{"station": "s"}, "", 0, true, "pour_ms"},
		{"negative pour", map[string]any{"station": "s", "pour_ms": -1.0}, "", 0, true, ">= 0"},
		{"string pour", map[string]any{"station": "s", "pour_ms": "100"}, "", 0, true, "must be a number"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			station, pourMs, err := parsePourFromShaker(tc.raw)
			if tc.wantErr {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.errMatch)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, station, test.ShouldEqual, tc.station)
			test.That(t, pourMs, test.ShouldEqual, tc.pourMs)
		})
	}
}
