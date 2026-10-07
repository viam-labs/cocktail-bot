package main

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"
)

func spillGlassJSON() map[string]interface{} {
	return map[string]interface{}{
		"label":    "wine glass",
		"score":    0.91,
		"bbox":     []interface{}{100, 120, 220, 400},
		"frame":    "world",
		"tilt_deg": 3,
		"keypoints_px": map[string]interface{}{
			"bottom_front": []interface{}{160.5, 398.0},
		},
		"rim_center_mm": []interface{}{12.5, -40, 610.25},
		"radius_mm":     38.2,
		"height_mm":     181,
	}
}

// roundTrip mimics DoCommand's wire format, where every JSON number arrives as float64.
func roundTrip(t *testing.T, m map[string]interface{}) map[string]interface{} {
	t.Helper()
	s, err := structpb.NewStruct(m)
	test.That(t, err, test.ShouldBeNil)
	return s.AsMap()
}

func TestParseFindGlassesResponse(t *testing.T) {
	resp := roundTrip(t, map[string]interface{}{
		"glasses": []interface{}{spillGlassJSON()},
		"table":   map[string]interface{}{"height_mm": 745, "frame": "world"},
	})

	glasses, err := parseFindGlassesResponse(resp)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, glasses, test.ShouldResemble, []spillGlass{{
		Label:       "wine glass",
		Score:       0.91,
		RimCenterMM: r3.Vector{X: 12.5, Y: -40, Z: 610.25},
		RadiusMM:    38.2,
		HeightMM:    181,
		TiltDeg:     3,
		Frame:       "world",
	}})
}

func TestParseFindGlassesResponseEmpty(t *testing.T) {
	glasses, err := parseFindGlassesResponse(roundTrip(t, map[string]interface{}{"glasses": []interface{}{}}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, glasses, test.ShouldBeEmpty)
}

func TestParseFindGlassesResponseErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(g map[string]interface{})
		resp   map[string]interface{}
		errMsg string
	}{
		"missing glasses":    {resp: map[string]interface{}{"table": map[string]interface{}{}}, errMsg: `no "glasses" key`},
		"glasses not a list": {resp: map[string]interface{}{"glasses": "none"}, errMsg: "want a list"},
		"glass not object":   {resp: map[string]interface{}{"glasses": []interface{}{1}}, errMsg: "want an object"},
		"short rim center":   {mutate: func(g map[string]interface{}) { g["rim_center_mm"] = []interface{}{1, 2} }, errMsg: "rim_center_mm"},
		"non-numeric rim":    {mutate: func(g map[string]interface{}) { g["rim_center_mm"] = []interface{}{1, "y", 2} }, errMsg: "rim_center_mm"},
		"missing radius":     {mutate: func(g map[string]interface{}) { delete(g, "radius_mm") }, errMsg: "radius_mm"},
		"string height":      {mutate: func(g map[string]interface{}) { g["height_mm"] = "tall" }, errMsg: "height_mm"},
		"missing frame":      {mutate: func(g map[string]interface{}) { delete(g, "frame") }, errMsg: "frame"},
	} {
		t.Run(name, func(t *testing.T) {
			resp := tc.resp
			if tc.mutate != nil {
				g := spillGlassJSON()
				tc.mutate(g)
				resp = map[string]interface{}{"glasses": []interface{}{g}}
			}
			_, err := parseFindGlassesResponse(roundTrip(t, resp))
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.errMsg)
		})
	}
}
