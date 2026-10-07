package bartender

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/spatialmath"
	viz "go.viam.com/rdk/vision"
	"go.viam.com/test"
)

// A camera 400 mm above the table at (0, 0, 500), looking 45° down along +X; its axis hits z=100 at (400, 0, 100).
func testCamera() spatialmath.Pose {
	axis := r3.Vector{X: 1, Z: -1}.Normalize()
	return spatialmath.NewPose(r3.Vector{Z: 500}, &spatialmath.OrientationVector{OX: axis.X, OY: axis.Y, OZ: axis.Z})
}

func TestLookTarget(t *testing.T) {
	target, err := lookTarget(testCamera(), 100)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.R3VectorAlmostEqual(target, r3.Vector{X: 400, Z: 100}, 1e-6), test.ShouldBeTrue)
}

func TestLookTargetRejectsBadViews(t *testing.T) {
	up := spatialmath.NewPose(r3.Vector{Z: 500}, &spatialmath.OrientationVector{OZ: 1})
	_, err := lookTarget(up, 100)
	test.That(t, err, test.ShouldNotBeNil)

	_, err = lookTarget(testCamera(), 600)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestLoweredViewKeepsAim(t *testing.T) {
	cam := testCamera()
	target, err := lookTarget(cam, 100)
	test.That(t, err, test.ShouldBeNil)

	for _, lowerMM := range []float64{0, 5, 10} {
		got := loweredView(cam, target, lowerMM)
		want := cam.Point().Sub(r3.Vector{Z: lowerMM})
		test.That(t, spatialmath.R3VectorAlmostEqual(got.Point(), want, 1e-9), test.ShouldBeTrue)
		toTarget := target.Sub(got.Point()).Normalize()
		test.That(t, opticalAxis(got).Normalize().Dot(toTarget), test.ShouldAlmostEqual, 1, 1e-9)
	}
}

func TestLoweredViewIsASmallRotation(t *testing.T) {
	cam := testCamera()
	target, _ := lookTarget(cam, 100)
	got := loweredView(cam, target, 10)
	angle := spatialmath.QuatToR4AA(spatialmath.OrientationBetween(cam.Orientation(), got.Orientation()).Quaternion()).Theta
	test.That(t, angle*180/math.Pi, test.ShouldBeLessThan, 2)
	test.That(t, angle, test.ShouldBeGreaterThan, 0)
}

func TestPannedViewTurnsInPlace(t *testing.T) {
	cam := testCamera()
	axis := opticalAxis(cam)
	for _, panDeg := range []float64{5, -5} {
		got := pannedView(cam, panDeg)
		test.That(t, spatialmath.R3VectorAlmostEqual(got.Point(), cam.Point(), 1e-9), test.ShouldBeTrue)
		gotAxis := opticalAxis(got)
		test.That(t, gotAxis.Z, test.ShouldAlmostEqual, axis.Z, 1e-9)
		turned := math.Atan2(gotAxis.Y, gotAxis.X) - math.Atan2(axis.Y, axis.X)
		test.That(t, turned*180/math.Pi, test.ShouldAlmostEqual, panDeg, 1e-6)
	}
	// Looking along +X, the camera's left is +Y.
	test.That(t, opticalAxis(pannedView(cam, 5)).Y, test.ShouldBeGreaterThan, 0)
	test.That(t, pannedView(cam, 0), test.ShouldEqual, cam)
}

func TestBestGlassUsesFirstNonEmptyObject(t *testing.T) {
	empty, err := viz.NewObjectWithLabel(pointcloud.NewBasicEmpty(), "cup", nil)
	test.That(t, err, test.ShouldBeNil)
	pc := pointcloud.NewBasicEmpty()
	test.That(t, pc.Set(r3.Vector{X: 10, Y: 20, Z: 0}, nil), test.ShouldBeNil)
	test.That(t, pc.Set(r3.Vector{X: 30, Y: 40, Z: 100}, nil), test.ShouldBeNil)
	glass, err := viz.NewObjectWithLabel(pc, "wine glass", nil)
	test.That(t, err, test.ShouldBeNil)

	c, label, ok := bestGlass([]*viz.Object{empty, glass})
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, label, test.ShouldEqual, "wine glass")
	test.That(t, spatialmath.R3VectorAlmostEqual(c, r3.Vector{X: 20, Y: 30, Z: 50}, 1e-9), test.ShouldBeTrue)

	_, _, ok = bestGlass(nil)
	test.That(t, ok, test.ShouldBeFalse)
}
