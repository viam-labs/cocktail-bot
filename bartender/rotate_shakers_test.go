package bartender

import (
	"testing"

	"go.viam.com/test"
)

func TestParseRotateShakers(t *testing.T) {
	cases := []struct {
		name     string
		raw      any
		want     string
		wantErr  bool
		errMatch string
	}{
		{"ok", map[string]any{"strain_flow": "strain-flow"}, "strain-flow", false, ""},
		{"not object", "nope", "", true, "object"},
		{"missing", map[string]any{}, "", true, "strain_flow"},
		{"empty", map[string]any{"strain_flow": ""}, "", true, "strain_flow"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRotateShakers(tc.raw)
			if tc.wantErr {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.errMatch)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, got, test.ShouldEqual, tc.want)
		})
	}
}
