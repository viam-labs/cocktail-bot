package glassfinder

import (
	"image"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/test"
)

func testProps() *camera.Properties {
	return &camera.Properties{
		IntrinsicParams: &transform.PinholeCameraIntrinsics{
			Width: 100, Height: 100, Fx: 100, Fy: 100, Ppx: 50, Ppy: 50,
		},
	}
}

func cloudOf(t *testing.T, pts ...r3.Vector) pointcloud.PointCloud {
	t.Helper()
	pc := pointcloud.NewBasicEmpty()
	for _, p := range pts {
		test.That(t, pc.Set(p, nil), test.ShouldBeNil)
	}
	return pc
}

func det(label string, score float64) objectdetection.Detection {
	return objectdetection.NewDetectionWithoutImgBounds(image.Rect(0, 0, 10, 10), score, label)
}

func TestFilterGlasses(t *testing.T) {
	dets := []objectdetection.Detection{
		det("cup", 0.6),
		det("person", 0.99),
		det("wine glass", 0.9),
		det("wine glass", 0.3),
	}
	got := filterGlasses(dets, defaultLabels, 0.5)
	test.That(t, len(got), test.ShouldEqual, 2)
	test.That(t, got[0].Label(), test.ShouldEqual, "wine glass")
	test.That(t, got[0].Score(), test.ShouldAlmostEqual, 0.9)
	test.That(t, got[1].Label(), test.ShouldEqual, "cup")
}

func TestCropToBox(t *testing.T) {
	// At z=1000mm with f=100px, 10mm lateral offset is 1px; (0,0) projects to pixel (50,50).
	inside := r3.Vector{X: 0, Y: 0, Z: 1000}
	insideEdge := r3.Vector{X: 50, Y: -50, Z: 1000}
	outside := r3.Vector{X: 200, Y: 0, Z: 1000}
	behind := r3.Vector{X: 0, Y: 0, Z: -1000}
	pc := cloudOf(t, inside, insideEdge, outside, behind)

	got, err := cropToBox(pc, testProps(), image.Rect(40, 40, 60, 60))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.Size(), test.ShouldEqual, 2)
	_, ok := got.At(inside.X, inside.Y, inside.Z)
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = got.At(insideEdge.X, insideEdge.Y, insideEdge.Z)
	test.That(t, ok, test.ShouldBeTrue)
}

func TestCropToBoxNoIntrinsics(t *testing.T) {
	pc := cloudOf(t, r3.Vector{Z: 1000})
	_, err := cropToBox(pc, &camera.Properties{}, image.Rect(0, 0, 10, 10))
	test.That(t, err, test.ShouldNotBeNil)
}

func TestCentroid(t *testing.T) {
	pc := cloudOf(t,
		r3.Vector{X: 0, Y: 0, Z: 0},
		r3.Vector{X: 10, Y: 0, Z: 0},
		r3.Vector{X: 0, Y: 30, Z: 0},
		r3.Vector{X: 10, Y: 30, Z: 100},
	)
	c, err := centroid(pc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, c.X, test.ShouldAlmostEqual, 5)
	test.That(t, c.Y, test.ShouldAlmostEqual, 15)
	test.That(t, c.Z, test.ShouldAlmostEqual, 25)

	_, err = centroid(pointcloud.NewBasicEmpty())
	test.That(t, err, test.ShouldNotBeNil)
}

func TestConfigValidate(t *testing.T) {
	_, _, err := (&Config{DetectorName: "yolov8"}).Validate("")
	test.That(t, err, test.ShouldNotBeNil)
	_, _, err = (&Config{CameraName: "cam"}).Validate("")
	test.That(t, err, test.ShouldNotBeNil)

	conf := &Config{CameraName: "cam", DetectorName: "yolov8"}
	deps, _, err := conf.Validate("")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(deps), test.ShouldEqual, 2)
	test.That(t, conf.labels(), test.ShouldResemble, defaultLabels)
	test.That(t, conf.minConfidence(), test.ShouldEqual, defaultMinConfidence)
}
