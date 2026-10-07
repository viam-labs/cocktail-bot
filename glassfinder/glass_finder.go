package glassfinder

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/vision"
	viz "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/rdk/vision/viscapture"
)

var Model = resource.NewModel("viam", "cocktail-bot", "glass-finder")

func init() {
	resource.RegisterService(vision.API, Model,
		resource.Registration[vision.Service, *Config]{
			Constructor: newGlassFinder,
		},
	)
}

// Glass is one detected glass with its point cloud and centroid in the world frame.
type Glass struct {
	Detection objectdetection.Detection
	Cloud     pointcloud.PointCloud
	Center    r3.Vector
}

type GlassFinder struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable

	conf     *Config
	logger   logging.Logger
	cam      camera.Camera
	detector vision.Service
	fs       framesystem.Service
}

func newGlassFinder(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (vision.Service, error) {
	conf, err := resource.NativeConfig[*Config](rawConf)
	if err != nil {
		return nil, err
	}
	return New(rawConf.ResourceName(), conf, deps, logger)
}

// New builds a GlassFinder from any resource provider: module dependencies or a connected robot client.
func New(name resource.Name, conf *Config, provider resource.Provider, logger logging.Logger) (*GlassFinder, error) {
	cam, err := camera.FromProvider(provider, conf.CameraName)
	if err != nil {
		return nil, fmt.Errorf("camera %q: %w", conf.CameraName, err)
	}
	detector, err := vision.FromProvider(provider, conf.DetectorName)
	if err != nil {
		return nil, fmt.Errorf("detector %q: %w", conf.DetectorName, err)
	}
	fs, err := framesystem.FromProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("frame system: %w", err)
	}
	return &GlassFinder{
		Named:    name.AsNamed(),
		conf:     conf,
		logger:   logger,
		cam:      cam,
		detector: detector,
		fs:       fs,
	}, nil
}

// FindGlasses captures one image and one point cloud, detects glasses in the image, and
// lifts each detection into the world frame. Results are ordered by detection score, highest first.
func (g *GlassFinder) FindGlasses(ctx context.Context) ([]Glass, *camera.NamedImage, error) {
	imgs, _, err := g.cam.Images(ctx, nil, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("images: %w", err)
	}
	if len(imgs) == 0 {
		return nil, nil, errors.New("camera returned no images")
	}
	img := &imgs[0]

	pc, err := g.cam.NextPointCloud(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("point cloud: %w", err)
	}
	props, err := g.cam.Properties(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("camera properties: %w", err)
	}

	dets, err := g.detector.Detections(ctx, img, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("detections: %w", err)
	}
	dets = filterGlasses(dets, g.conf.labels(), g.conf.minConfidence())

	glasses := make([]Glass, 0, len(dets))
	for _, d := range dets {
		camCloud, err := cropToBox(pc, &props, *d.BoundingBox())
		if err != nil {
			return nil, nil, fmt.Errorf("crop %s %v: %w", d.Label(), *d.BoundingBox(), err)
		}
		if camCloud.Size() == 0 {
			g.logger.Warnw("no depth points inside glass box", "label", d.Label(), "box", *d.BoundingBox())
			continue
		}
		worldCloud, err := g.fs.TransformPointCloud(ctx, camCloud, g.conf.CameraName, referenceframe.World)
		if err != nil {
			return nil, nil, fmt.Errorf("transform to world: %w", err)
		}
		center, err := centroid(worldCloud)
		if err != nil {
			return nil, nil, err
		}
		glasses = append(glasses, Glass{Detection: d, Cloud: worldCloud, Center: center})
	}
	return glasses, img, nil
}

func (g *GlassFinder) CaptureAllFromCamera(
	ctx context.Context,
	_ string,
	opts viscapture.CaptureOptions,
	_ map[string]interface{},
) (viscapture.VisCapture, error) {
	glasses, img, err := g.FindGlasses(ctx)
	if err != nil {
		return viscapture.VisCapture{}, err
	}
	ret := viscapture.VisCapture{}
	if opts.ReturnImage {
		ret.Image = img
	}
	for _, gl := range glasses {
		if opts.ReturnDetections {
			ret.Detections = append(ret.Detections, gl.Detection)
		}
		if opts.ReturnObject {
			o, err := viz.NewObjectWithLabel(gl.Cloud, gl.Detection.Label(), nil)
			if err != nil {
				return viscapture.VisCapture{}, err
			}
			ret.Objects = append(ret.Objects, o)
		}
	}
	return ret, nil
}

func (g *GlassFinder) GetObjectPointClouds(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*viz.Object, error) {
	ret, err := g.CaptureAllFromCamera(ctx, cameraName, viscapture.CaptureOptions{ReturnObject: true}, extra)
	if err != nil {
		return nil, err
	}
	return ret.Objects, nil
}

func (g *GlassFinder) DetectionsFromCamera(ctx context.Context, cameraName string, extra map[string]interface{}) ([]objectdetection.Detection, error) {
	ret, err := g.CaptureAllFromCamera(ctx, cameraName, viscapture.CaptureOptions{ReturnDetections: true}, extra)
	if err != nil {
		return nil, err
	}
	return ret.Detections, nil
}

func (g *GlassFinder) Detections(_ context.Context, _ *camera.NamedImage, _ map[string]interface{}) ([]objectdetection.Detection, error) {
	return nil, errors.New("Detections not supported: glass-finder needs its own depth capture, use DetectionsFromCamera")
}

func (g *GlassFinder) ClassificationsFromCamera(_ context.Context, _ string, _ int, _ map[string]interface{}) (classification.Classifications, error) {
	return nil, errors.New("classifications not supported")
}

func (g *GlassFinder) Classifications(_ context.Context, _ *camera.NamedImage, _ int, _ map[string]interface{}) (classification.Classifications, error) {
	return nil, errors.New("classifications not supported")
}

func (g *GlassFinder) GetProperties(_ context.Context, _ map[string]interface{}) (*vision.Properties, error) {
	return &vision.Properties{
		DetectionSupported:  true,
		ObjectPCDsSupported: true,
		DefaultCamera:       &g.conf.CameraName,
	}, nil
}
