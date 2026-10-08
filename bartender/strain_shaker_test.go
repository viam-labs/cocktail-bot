package bartender

import (
	"testing"

	"go.viam.com/test"
)

func TestParseStrainShaker(t *testing.T) {
	ok := map[string]any{
		"source":         "ice-station",
		"strain_flow":    "strain-flow",
		"drain_dwell_ms": 4000.0,
		"dump_dwell_ms":  2000.0,
	}

	cases := []struct {
		name     string
		raw      any
		wantErr  bool
		errMatch string
	}{
		{"ok", ok, false, ""},
		{"not object", "not-a-map", true, "object"},
		{"missing source", omit(ok, "source"), true, "source"},
		{"missing strain_flow", omit(ok, "strain_flow"), true, "strain_flow"},
		{"missing drain dwell", omit(ok, "drain_dwell_ms"), true, "drain_dwell_ms"},
		{"missing dump dwell", omit(ok, "dump_dwell_ms"), true, "dump_dwell_ms"},
		{"negative drain", override(ok, "drain_dwell_ms", -1.0), true, ">= 0"},
		{"negative dump", override(ok, "dump_dwell_ms", -1.0), true, ">= 0"},
		{"string dwell", override(ok, "drain_dwell_ms", "nope"), true, "must be a number"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := parseStrainShaker(tc.raw)
			if tc.wantErr {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.errMatch)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, req.source, test.ShouldEqual, "ice-station")
			test.That(t, req.strainFlow, test.ShouldEqual, "strain-flow")
			test.That(t, req.drainDwellMs, test.ShouldEqual, 4000)
			test.That(t, req.dumpDwellMs, test.ShouldEqual, 2000)
		})
	}
}

func omit(m map[string]any, key string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if k != key {
			out[k] = v
		}
	}
	return out
}

func override(m map[string]any, key string, value any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	out[key] = value
	return out
}
