package bartender

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	viz "go.viam.com/rdk/vision"
)

const (
	poseGlassLook = "glass-look"

	defaultGlassSearchSettleMs = 1000
)

var (
	defaultGlassSearchLowerMM = []float64{0, 5, 10}
	defaultGlassSearchPanDeg  = []float64{0, 5, -5}
)

type foundGlass struct {
	center  r3.Vector
	label   string
	lowerMM float64
	panDeg  float64
}

func opticalAxis(cam spatialmath.Pose) r3.Vector {
	return spatialmath.Compose(cam, spatialmath.NewPoseFromPoint(r3.Vector{Z: 1})).Point().Sub(cam.Point())
}

// lookTarget is where the camera's optical axis (+Z of the camera frame) meets the table plane.
func lookTarget(cam spatialmath.Pose, tableZ float64) (r3.Vector, error) {
	axis := opticalAxis(cam)
	if axis.Z > -0.1 {
		return r3.Vector{}, errors.New("camera at glass-look does not point down at the table")
	}
	t := (tableZ - cam.Point().Z) / axis.Z
	if t <= 0 {
		return r3.Vector{}, fmt.Errorf("camera at glass-look is below glass_table_z_mm %.0f", tableZ)
	}
	return cam.Point().Add(axis.Mul(t)), nil
}

// loweredView moves the camera straight down by lowerMM and re-aims it at target with the smallest
// rotation, so the image roll stays as saved.
func loweredView(cam spatialmath.Pose, target r3.Vector, lowerMM float64) spatialmath.Pose {
	if lowerMM == 0 {
		return cam
	}
	pos := cam.Point().Sub(r3.Vector{Z: lowerMM})
	from := opticalAxis(cam).Normalize()
	to := target.Sub(pos).Normalize()
	axis := from.Cross(to)
	angle := math.Atan2(axis.Norm(), from.Dot(to))
	orientation := cam.Orientation()
	if axis.Norm() > 1e-12 {
		a := axis.Normalize()
		reaim := spatialmath.NewPoseFromOrientation(&spatialmath.R4AA{Theta: angle, RX: a.X, RY: a.Y, RZ: a.Z})
		orientation = spatialmath.Compose(reaim, spatialmath.NewPoseFromOrientation(cam.Orientation())).Orientation()
	}
	return spatialmath.NewPose(pos, orientation)
}

// pannedView turns the camera about the vertical through its own position, so it looks a little left
// (positive) or right (negative) of where it was aimed without moving.
func pannedView(cam spatialmath.Pose, panDeg float64) spatialmath.Pose {
	if panDeg == 0 {
		return cam
	}
	pan := spatialmath.NewPoseFromOrientation(&spatialmath.R4AA{Theta: panDeg * math.Pi / 180, RZ: 1})
	orientation := spatialmath.Compose(pan, spatialmath.NewPoseFromOrientation(cam.Orientation())).Orientation()
	return spatialmath.NewPose(cam.Point(), orientation)
}

func cloudCentroid(pc pointcloud.PointCloud) (r3.Vector, bool) {
	if pc == nil || pc.Size() == 0 {
		return r3.Vector{}, false
	}
	var sum r3.Vector
	pc.Iterate(0, 0, func(p r3.Vector, _ pointcloud.Data) bool {
		sum = sum.Add(p)
		return true
	})
	return sum.Mul(1 / float64(pc.Size())), true
}

// The glass finder returns objects highest score first, in the world frame.
func bestGlass(objs []*viz.Object) (r3.Vector, string, bool) {
	for _, o := range objs {
		if c, ok := cloudCentroid(o.PointCloud); ok {
			label := ""
			if o.Geometry != nil {
				label = o.Geometry.Label()
			}
			return c, label, true
		}
	}
	return r3.Vector{}, "", false
}

func (b *bartender) glassCameraName(ctx context.Context) (string, error) {
	props, err := b.glassFinder.GetProperties(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("glass finder properties: %w", err)
	}
	if props.DefaultCamera == nil || *props.DefaultCamera == "" {
		return "", errors.New("glass finder does not report its camera")
	}
	return *props.DefaultCamera, nil
}

func (b *bartender) framePoseInWorld(ctx context.Context, frame string) (spatialmath.Pose, error) {
	fs, inputs, err := b.currentInputs(ctx)
	if err != nil {
		return nil, err
	}
	tf, err := fs.Transform(inputs.ToLinearInputs(), referenceframe.NewPoseInFrame(frame, spatialmath.NewZeroPose()), referenceframe.World)
	if err != nil {
		return nil, fmt.Errorf("pose of %s in world: %w", frame, err)
	}
	return tf.(*referenceframe.PoseInFrame).Pose(), nil
}

// findGlass goes to the saved glass-look pose and, until the glass finder sees a glass, tries nearby views:
// at each height (lowered a few millimetres, re-aimed at the spot it looked at) it also pans a few degrees
// left and right. The arm is left at the view where the glass was found.
func (b *bartender) findGlass(ctx context.Context) (foundGlass, error) {
	if b.glassFinder == nil {
		return foundGlass{}, errors.New("glass_finder_name is not configured")
	}
	camName, err := b.glassCameraName(ctx)
	if err != nil {
		return foundGlass{}, err
	}
	if _, err := b.moveArmToPose(ctx, poseGlassLook); err != nil {
		return foundGlass{}, fmt.Errorf("move to %s: %w", poseGlassLook, err)
	}
	cam, err := b.framePoseInWorld(ctx, camName)
	if err != nil {
		return foundGlass{}, err
	}
	target, err := lookTarget(cam, b.cfg.GlassTableZMM)
	if err != nil {
		return foundGlass{}, err
	}
	b.logger.Infow("searching for glass", "look_target", target, "camera", camName)

	for _, lowerMM := range b.cfg.glassSearchLowerMM() {
		for _, panDeg := range b.cfg.glassSearchPanDeg() {
			label := fmt.Sprintf("glass-search-lower%.0fmm-pan%+.0fdeg", lowerMM, panDeg)
			if lowerMM != 0 || panDeg != 0 {
				view := &poseData{pose: pannedView(loweredView(cam, target, lowerMM), panDeg), refFrame: referenceframe.World, componentName: camName}
				if _, err := b.moveToResolvedPose(ctx, view, label, nil, nil); err != nil {
					b.logger.Warnw("skipping unreachable search view", "view", label, "err", err)
					continue
				}
			}
			// Let the arm settle so the camera delivers a frame taken at this view.
			if err := sleepCtx(ctx, time.Duration(b.cfg.glassSearchSettleMs())*time.Millisecond); err != nil {
				return foundGlass{}, err
			}
			objs, err := b.glassFinder.GetObjectPointClouds(ctx, camName, nil)
			if err != nil {
				return foundGlass{}, fmt.Errorf("%s: glass finder: %w", label, err)
			}
			if center, glassLabel, ok := bestGlass(objs); ok {
				b.logger.Infow("found glass", "label", glassLabel, "x", center.X, "y", center.Y, "z", center.Z, "view", label)
				return foundGlass{center: center, label: glassLabel, lowerMM: lowerMM, panDeg: panDeg}, nil
			}
			b.logger.Infow("no glass in view", "view", label)
		}
	}
	if _, err := b.moveArmToPose(ctx, poseGlassLook); err != nil {
		return foundGlass{}, fmt.Errorf("no glass found, and return to %s failed: %w", poseGlassLook, err)
	}
	return foundGlass{}, errors.New("no glass found in any search view")
}

func (b *bartender) findAndPour(ctx context.Context, bottle string, pourMs int, mouthOffsetMM *float64) (foundGlass, error) {
	if _, err := b.findSwitch(bottle); err != nil {
		return foundGlass{}, err
	}
	glass, err := b.findGlass(ctx)
	if err != nil {
		return foundGlass{}, err
	}
	req := pourIntoGlassesReq{
		bottle:        bottle,
		pourMs:        pourMs,
		glasses:       []GlassXY{{X: glass.center.X, Y: glass.center.Y}},
		mouthOffsetMM: mouthOffsetMM,
	}
	return glass, b.pourIntoGlasses(ctx, req)
}
