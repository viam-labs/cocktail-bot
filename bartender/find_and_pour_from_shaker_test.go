package bartender

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/test"
)

// The serving station's saved serve-tilt.
func savedServeTilt() *poseData {
	return &poseData{
		pose: spatialmath.NewPose(
			r3.Vector{X: 622.2023366150262, Y: -245.16777240248757, Z: 339.17742002198474},
			&spatialmath.OrientationVectorDegrees{OX: 0.01794448973280975, OY: 0.999704570697147, OZ: -0.016394103069772292, Theta: -87.15222732895946},
		),
		refFrame:      referenceframe.World,
		componentName: "gripper",
	}
}

func TestWithXYKeepsZAndOrientation(t *testing.T) {
	saved := savedServeTilt()
	got := withXY(saved, 580, -87)
	test.That(t, spatialmath.R3VectorAlmostEqual(got.pose.Point(), r3.Vector{X: 580, Y: -87, Z: saved.pose.Point().Z}, 1e-9), test.ShouldBeTrue)
	test.That(t, spatialmath.OrientationAlmostEqual(got.pose.Orientation(), saved.pose.Orientation()), test.ShouldBeTrue)
	test.That(t, got.refFrame, test.ShouldEqual, referenceframe.World)
	test.That(t, got.componentName, test.ShouldEqual, "gripper")
	test.That(t, saved.pose.Point().X, test.ShouldAlmostEqual, 622.2023366150262, 1e-9)
}

func TestCheckServeShift(t *testing.T) {
	saved := savedServeTilt().pose.Point()
	test.That(t, checkServeShift(r3.Vector{X: 580, Y: -87, Z: 70}, saved, 300), test.ShouldBeNil)
	err := checkServeShift(r3.Vector{X: 100, Y: 300}, saved, 300)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "serve-tilt")
}

func TestParseFindAndPourFromShaker(t *testing.T) {
	pourMs, err := parseFindAndPourFromShaker(map[string]any{"pour_ms": 5000.0})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pourMs, test.ShouldEqual, 5000)

	for _, raw := range []any{"x", map[string]any{}, map[string]any{"pour_ms": "a"}, map[string]any{"pour_ms": -1.0}} {
		_, err := parseFindAndPourFromShaker(raw)
		test.That(t, err, test.ShouldNotBeNil)
	}
}

func TestServingStationDefault(t *testing.T) {
	test.That(t, (&Config{}).servingStation(), test.ShouldEqual, "serving-glass-center")
	test.That(t, (&Config{ServingStation: "serving"}).servingStation(), test.ShouldEqual, "serving")
}
