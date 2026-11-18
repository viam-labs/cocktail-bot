package bartender

import (
	"testing"

	"go.viam.com/test"
)

func TestParseStockBottle(t *testing.T) {
	ok := map[string]any{"slot": "slot-g-bottom-front", "ingredient": "aged-rum"}
	cases := []struct {
		name     string
		raw      any
		wantErr  bool
		errMatch string
	}{
		{"ok", ok, false, ""},
		{"not object", "nope", true, "object"},
		{"missing slot", omit(ok, "slot"), true, "slot"},
		{"missing ingredient", omit(ok, "ingredient"), true, "ingredient"},
		{"empty slot", override(ok, "slot", ""), true, "slot"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			slot, ing, err := parseStockBottle(tc.raw)
			if tc.wantErr {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.errMatch)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, slot, test.ShouldEqual, "slot-g-bottom-front")
			test.That(t, ing, test.ShouldEqual, "aged-rum")
		})
	}
}

func TestParseRetrieveBottle(t *testing.T) {
	ok := map[string]any{"slot": "slot-g-bottom-front"}
	cases := []struct {
		name     string
		raw      any
		wantErr  bool
		errMatch string
	}{
		{"ok", ok, false, ""},
		{"not object", 42, true, "object"},
		{"missing slot", omit(ok, "slot"), true, "slot"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			slot, err := parseRetrieveBottle(tc.raw)
			if tc.wantErr {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.errMatch)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, slot, test.ShouldEqual, "slot-g-bottom-front")
		})
	}
}

func TestParseMoveBottle(t *testing.T) {
	ok := map[string]any{"from": "slot-a-bottom-front", "to": "slot-g-bottom-front"}
	cases := []struct {
		name     string
		raw      any
		wantErr  bool
		errMatch string
	}{
		{"ok", ok, false, ""},
		{"not object", nil, true, "object"},
		{"missing from", omit(ok, "from"), true, "from"},
		{"missing to", omit(ok, "to"), true, "to"},
		{"same slot", override(ok, "to", "slot-a-bottom-front"), true, "must differ"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			from, to, err := parseMoveBottle(tc.raw)
			if tc.wantErr {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.errMatch)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, from, test.ShouldEqual, "slot-a-bottom-front")
			test.That(t, to, test.ShouldEqual, "slot-g-bottom-front")
		})
	}
}
