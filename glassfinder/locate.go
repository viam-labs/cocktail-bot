package glassfinder

import (
	"errors"
	"image"
	"slices"
	"sort"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/vision/objectdetection"
)

// filterGlasses keeps detections with an allowed label and score, highest score first.
func filterGlasses(dets []objectdetection.Detection, labels []string, minConfidence float64) []objectdetection.Detection {
	var out []objectdetection.Detection
	for _, d := range dets {
		if d.BoundingBox() == nil || d.Score() < minConfidence || !slices.Contains(labels, d.Label()) {
			continue
		}
		out = append(out, d)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score() > out[j].Score() })
	return out
}

// cropToBox returns the points of pc (camera frame) that project inside box.
func cropToBox(pc pointcloud.PointCloud, props *camera.Properties, box image.Rectangle) (pointcloud.PointCloud, error) {
	out := pointcloud.NewBasicEmpty()
	var iterErr error
	pc.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
		// Points at or behind the image plane can't project into the image.
		if p.Z <= 0 {
			return true
		}
		x, y, err := props.PointToPixel(p)
		if err != nil {
			iterErr = err
			return false
		}
		if !(image.Point{X: int(x), Y: int(y)}).In(box) {
			return true
		}
		if err := out.Set(p, d); err != nil {
			iterErr = err
			return false
		}
		return true
	})
	if iterErr != nil {
		return nil, iterErr
	}
	return out, nil
}

func centroid(pc pointcloud.PointCloud) (r3.Vector, error) {
	if pc.Size() == 0 {
		return r3.Vector{}, errors.New("empty point cloud")
	}
	var sum r3.Vector
	pc.Iterate(0, 0, func(p r3.Vector, _ pointcloud.Data) bool {
		sum = sum.Add(p)
		return true
	})
	return sum.Mul(1 / float64(pc.Size())), nil
}
