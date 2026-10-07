package bartender

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/test"
)

// Gripper pointing down (z axis -Z) at pour-approach; at pour-tilt its z axis swings toward +X.
func testPourPoses() (spatialmath.Pose, spatialmath.Pose) {
	approach := spatialmath.NewPose(r3.Vector{X: 400, Y: 0, Z: 300}, &spatialmath.OrientationVector{OZ: -1})
	tilted := r3.Vector{X: 1, Z: -1}.Normalize()
	tilt := spatialmath.NewPose(r3.Vector{X: 420, Y: 0, Z: 280}, &spatialmath.OrientationVector{OX: tilted.X, OY: tilted.Y, OZ: tilted.Z})
	return approach, tilt
}

func TestLeanDirection(t *testing.T) {
	approach, tilt := testPourPoses()
	lean, err := leanDirection(approach, tilt)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.R3VectorAlmostEqual(lean, r3.Vector{X: 1}, 1e-9), test.ShouldBeTrue)

	_, err = leanDirection(approach, approach)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestPourShiftsPutMouthOverGlass(t *testing.T) {
	_, tilt := testPourPoses()
	lean := r3.Vector{X: 1}
	shifts, err := pourShifts([]GlassXY{{X: 600, Y: 50}, {X: 520, Y: 0}}, tilt.Point(), lean, 100, 300)
	test.That(t, err, test.ShouldBeNil)
	for i, g := range []GlassXY{{X: 600, Y: 50}, {X: 520, Y: 0}} {
		gripper := tilt.Point().Add(shifts[i])
		mouth := gripper.Add(lean.Mul(100))
		test.That(t, mouth.X, test.ShouldAlmostEqual, g.X, 1e-9)
		test.That(t, mouth.Y, test.ShouldAlmostEqual, g.Y, 1e-9)
		test.That(t, shifts[i].Z, test.ShouldEqual, 0.0)
	}
	test.That(t, spatialmath.R3VectorAlmostEqual(shifts[1], r3.Vector{}, 1e-9), test.ShouldBeTrue)
}

func TestPourShiftsNegativeOffsetFlipsSide(t *testing.T) {
	_, tilt := testPourPoses()
	shifts, err := pourShifts([]GlassXY{{X: 420, Y: 0}}, tilt.Point(), r3.Vector{X: 1}, -100, 300)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.R3VectorAlmostEqual(shifts[0], r3.Vector{X: 100}, 1e-9), test.ShouldBeTrue)
}

func TestPourShiftsRejectsFarGlass(t *testing.T) {
	_, tilt := testPourPoses()
	_, err := pourShifts([]GlassXY{{X: 520, Y: 0}, {X: 520, Y: 400}}, tilt.Point(), r3.Vector{X: 1}, 100, 300)
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
	test.That(t, req.mouthOffsetMM, test.ShouldBeNil)
}

func TestParsePourIntoGlassesMouthOffset(t *testing.T) {
	req, err := parsePourIntoGlasses(map[string]any{
		"bottle": "b", "pour_ms": 1.0, "mouth_offset_mm": -80.0,
		"glasses": []any{map[string]any{"x": 1.0, "y": 2.0}},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, *req.mouthOffsetMM, test.ShouldEqual, -80.0)

	_, err = parsePourIntoGlasses(map[string]any{
		"bottle": "b", "pour_ms": 1.0, "mouth_offset_mm": "far",
		"glasses": []any{map[string]any{"x": 1.0, "y": 2.0}},
	})
	test.That(t, err, test.ShouldNotBeNil)
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
