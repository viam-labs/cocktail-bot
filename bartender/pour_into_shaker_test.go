package bartender

import (
	"testing"

	"go.viam.com/test"
)

func TestPourCount(t *testing.T) {
	cases := []struct {
		name     string
		oz       float64
		pourerOz float64
		want     int
		wantErr  bool
		errMatch string
	}{
		{"exact 1x", 1.0, 1.0, 1, false, ""},
		{"exact 2x", 2.0, 1.0, 2, false, ""},
		{"exact 3x 1.5", 4.5, 1.5, 3, false, ""},
		{"not a multiple", 2.5, 1.0, 0, true, "exact multiple"},
		{"half short of pourer", 0.5, 1.0, 0, true, "exact multiple"},
		{"zero oz", 0, 1.0, 0, true, "> 0"},
		{"negative oz", -1, 1.0, 0, true, "> 0"},
		{"zero pourer", 1.0, 0, 0, true, "> 0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, err := pourCount(tc.oz, tc.pourerOz)
			if tc.wantErr {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.errMatch)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, n, test.ShouldEqual, tc.want)
		})
	}
}

func TestParsePourIntoShaker(t *testing.T) {
	cases := []struct {
		name     string
		raw      any
		bottle   string
		oz       float64
		wantErr  bool
		errMatch string
	}{
		{"ok", map[string]any{"bottle": "vodka", "oz": 2.0}, "vodka", 2.0, false, ""},
		{"ok int", map[string]any{"bottle": "v", "oz": 1}, "v", 1.0, false, ""},
		{"not object", "vodka", "", 0, true, "object"},
		{"missing bottle", map[string]any{"oz": 2.0}, "", 0, true, "bottle"},
		{"missing oz", map[string]any{"bottle": "v"}, "", 0, true, "oz"},
		{"zero oz", map[string]any{"bottle": "v", "oz": 0.0}, "", 0, true, "> 0"},
		{"negative oz", map[string]any{"bottle": "v", "oz": -1.0}, "", 0, true, "> 0"},
		{"string oz", map[string]any{"bottle": "v", "oz": "2"}, "", 0, true, "must be a number"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bottle, oz, err := parsePourIntoShaker(tc.raw)
			if tc.wantErr {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.errMatch)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, bottle, test.ShouldEqual, tc.bottle)
			test.That(t, oz, test.ShouldEqual, tc.oz)
		})
	}
}
