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

func opticalAxis(p spatialmath.Pose) r3.Vector {
	return spatialmath.Compose(p, spatialmath.NewPoseFromPoint(r3.Vector{Z: 1})).Point().Sub(p.Point())
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

func TestOrbitViewKeepsAimAndDistance(t *testing.T) {
	cam := testCamera()
	target, err := lookTarget(cam, 100)
	test.That(t, err, test.ShouldBeNil)
	dist := cam.Point().Distance(target)

	for _, v := range []searchView{{0, 0}, {15, 0}, {30, 20}, {0, -20}} {
		got := orbitView(cam, target, v.lowerDeg, v.rotateDeg)
		test.That(t, got.Point().Distance(target), test.ShouldAlmostEqual, dist, 1e-6)
		toTarget := target.Sub(got.Point()).Normalize()
		test.That(t, opticalAxis(got).Normalize().Dot(toTarget), test.ShouldAlmostEqual, 1, 1e-9)
		test.That(t, elevationDeg(got.Point(), target), test.ShouldAlmostEqual, 45-v.lowerDeg, 1e-6)

		start := cam.Point().Sub(target)
		end := got.Point().Sub(target)
		rotated := math.Atan2(end.Y, end.X) - math.Atan2(start.Y, start.X)
		test.That(t, wrapDeg(rotated*180/math.Pi), test.ShouldAlmostEqual, v.rotateDeg, 1e-6)
	}
}

func wrapDeg(d float64) float64 {
	return math.Atan2(math.Sin(d*math.Pi/180), math.Cos(d*math.Pi/180)) * 180 / math.Pi
}

func TestLoweredViewIsLowerAndFurther(t *testing.T) {
	cam := testCamera()
	target, _ := lookTarget(cam, 100)
	got := orbitView(cam, target, 20, 0)
	test.That(t, got.Point().Z, test.ShouldBeLessThan, cam.Point().Z)
	test.That(t, target.X-got.Point().X, test.ShouldBeGreaterThan, target.X-cam.Point().X)
}

func TestSearchViewsOrderAndMinElevation(t *testing.T) {
	cam := testCamera()
	target, _ := lookTarget(cam, 100)
	views := searchViews(cam, target, []float64{0, 15, 35}, []float64{0, 20, -20}, 15)
	test.That(t, views, test.ShouldResemble, []searchView{
		{0, 0}, {0, 20}, {0, -20},
		{15, 0}, {15, 20}, {15, -20},
	})
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
