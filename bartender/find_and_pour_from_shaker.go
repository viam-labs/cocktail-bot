package bartender

import (
	"context"
	"fmt"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const defaultServingStation = "serving-glass-center"

// withXY keeps a world-frame pose's z and orientation and moves it to x, y.
func withXY(pd *poseData, x, y float64) *poseData {
	p := pd.pose.Point()
	return &poseData{
		pose:          spatialmath.NewPose(r3.Vector{X: x, Y: y, Z: p.Z}, pd.pose.Orientation()),
		refFrame:      referenceframe.World,
		componentName: pd.componentName,
	}
}

func checkServeShift(glass, savedTilt r3.Vector, maxShiftMM float64) error {
	shift := r3.Vector{X: glass.X - savedTilt.X, Y: glass.Y - savedTilt.Y}
	if shift.Norm() > maxShiftMM {
		return fmt.Errorf("glass at (%.0f, %.0f) is %.0f mm from the saved serve-tilt (%.0f, %.0f), over max_pour_offset_mm %.0f",
			glass.X, glass.Y, shift.Norm(), savedTilt.X, savedTilt.Y, maxShiftMM)
	}
	return nil
}

// findAndPourFromShaker finds the glass, then runs pour_from_shaker on the serving station with its
// serve-approach and serve-tilt moved to the glass's x, y (z and orientation as saved).
func (b *bartender) findAndPourFromShaker(ctx context.Context, pourMs int) (foundGlass, error) {
	sw, err := b.findSwitch(b.cfg.servingStation())
	if err != nil {
		return foundGlass{}, err
	}
	approach, err := fetchPose(ctx, sw, poseServeApproach)
	if err != nil {
		return foundGlass{}, err
	}
	tilt, err := fetchPose(ctx, sw, poseServeTilt)
	if err != nil {
		return foundGlass{}, err
	}
	if approach, err = b.poseInWorld(ctx, approach); err != nil {
		return foundGlass{}, err
	}
	if tilt, err = b.poseInWorld(ctx, tilt); err != nil {
		return foundGlass{}, err
	}

	glass, err := b.findGlass(ctx)
	if err != nil {
		return foundGlass{}, err
	}
	if err := checkServeShift(glass.center, tilt.pose.Point(), b.cfg.maxPourOffsetMM()); err != nil {
		return foundGlass{}, err
	}
	b.logger.Infow("serving into found glass", "x", glass.center.X, "y", glass.center.Y,
		"saved_serve_tilt", tilt.pose.Point(), "station", sw.Name().ShortName())
	return glass, b.pourFromShakerAt(ctx, sw, pourMs,
		withXY(approach, glass.center.X, glass.center.Y),
		withXY(tilt, glass.center.X, glass.center.Y))
}
