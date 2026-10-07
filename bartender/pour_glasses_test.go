package bartender

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/test"
)

func TestPourOffsets(t *testing.T) {
	ref := GlassXY{X: 500, Y: 100}
	offsets, err := pourOffsets(ref, []GlassXY{{X: 500, Y: 100}, {X: 600, Y: 50}}, 300)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, offsets[0], test.ShouldResemble, r3.Vector{})
	test.That(t, offsets[1], test.ShouldResemble, r3.Vector{X: 100, Y: -50})
}

func TestPourOffsetsRejectsFarGlass(t *testing.T) {
	_, err := pourOffsets(GlassXY{}, []GlassXY{{X: 10, Y: 0}, {X: 300, Y: 300}}, 300)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "glass 1")
}

func TestShiftedPoseKeepsZAndOrientation(t *testing.T) {
	o := &spatialmath.OrientationVectorDegrees{OX: 0.3, OY: -0.2, OZ: -1, Theta: 40}
	pd := &poseData{
		pose:          spatialmath.NewPose(r3.Vector{X: 1, Y: 2, Z: 350}, o),
		refFrame:      referenceframe.World,
		componentName: "gripper",
	}
	got := shiftedPose(pd, r3.Vector{X: 100, Y: -50})
	test.That(t, spatialmath.R3VectorAlmostEqual(got.pose.Point(), r3.Vector{X: 101, Y: -48, Z: 350}, 1e-9), test.ShouldBeTrue)
	test.That(t, spatialmath.OrientationAlmostEqual(got.pose.Orientation(), o), test.ShouldBeTrue)
	test.That(t, got.refFrame, test.ShouldEqual, referenceframe.World)
	test.That(t, got.componentName, test.ShouldEqual, "gripper")
	test.That(t, spatialmath.R3VectorAlmostEqual(pd.pose.Point(), r3.Vector{X: 1, Y: 2, Z: 350}, 1e-9), test.ShouldBeTrue)
}

func TestParsePourIntoGlasses(t *testing.T) {
	req, err := parsePourIntoGlasses(map[string]any{
		"bottle":  "bottle-gin",
		"pour_ms": 1500.0,
		"glasses": []any{
			map[string]any{"x": 500.0, "y": 100.0},
			map[string]any{"x": 600.0, "y": -20.5},
		},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, req.bottle, test.ShouldEqual, "bottle-gin")
	test.That(t, req.pourMs, test.ShouldEqual, 1500)
	test.That(t, req.glasses, test.ShouldResemble, []GlassXY{{X: 500, Y: 100}, {X: 600, Y: -20.5}})
}

func TestParsePourIntoGlassesErrors(t *testing.T) {
	cases := []struct {
		name     string
		raw      any
		errMatch string
	}{
		{"not object", "x", "object"},
		{"missing bottle", map[string]any{"pour_ms": 1.0, "glasses": []any{map[string]any{"x": 1.0, "y": 1.0}}}, "bottle"},
		{"missing glasses", map[string]any{"bottle": "b", "pour_ms": 1.0}, "glasses"},
		{"empty glasses", map[string]any{"bottle": "b", "pour_ms": 1.0, "glasses": []any{}}, "glasses"},
		{"glass not object", map[string]any{"bottle": "b", "pour_ms": 1.0, "glasses": []any{1.0}}, "glasses[0]"},
		{"glass missing y", map[string]any{"bottle": "b", "pour_ms": 1.0, "glasses": []any{map[string]any{"x": 1.0}}}, "glasses[0]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parsePourIntoGlasses(tc.raw)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.errMatch)
		})
	}
}
