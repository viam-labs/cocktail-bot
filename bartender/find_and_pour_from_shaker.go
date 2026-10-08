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

type skippedGlass struct {
	glass  foundGlass
	reason string
}

// findAndPourFromShaker finds the glasses, then runs the full pour_from_shaker sequence once per glass on the
// serving station, with serve-approach and serve-tilt moved to that glass's x, y (z and orientation as saved).
// Glasses too far from the saved serve-tilt are skipped; if none is left, nothing is grabbed.
func (b *bartender) findAndPourFromShaker(ctx context.Context, pourMs int) ([]foundGlass, []skippedGlass, error) {
	sw, err := b.findSwitch(b.cfg.servingStation())
	if err != nil {
		return nil, nil, err
	}
	approach, err := fetchPose(ctx, sw, poseServeApproach)
	if err != nil {
		return nil, nil, err
	}
	tilt, err := fetchPose(ctx, sw, poseServeTilt)
	if err != nil {
		return nil, nil, err
	}
	if approach, err = b.poseInWorld(ctx, approach); err != nil {
		return nil, nil, err
	}
	if tilt, err = b.poseInWorld(ctx, tilt); err != nil {
		return nil, nil, err
	}

	glasses, err := b.findGlasses(ctx)
	if err != nil {
		return nil, nil, err
	}
	var targets []foundGlass
	var skipped []skippedGlass
	for _, g := range glasses {
		if err := checkServeShift(g.center, tilt.pose.Point(), b.cfg.maxPourOffsetMM()); err != nil {
			b.logger.Warnw("skipping glass", "x", g.center.X, "y", g.center.Y, "err", err)
			skipped = append(skipped, skippedGlass{glass: g, reason: err.Error()})
			continue
		}
		targets = append(targets, g)
	}
	if len(targets) == 0 {
		return nil, skipped, fmt.Errorf("found %d glass(es) but none within max_pour_offset_mm of the saved serve-tilt: %s",
			len(glasses), skipped[0].reason)
	}

	var poured []foundGlass
	for i, g := range targets {
		b.logger.Infow("serving into found glass", "glass", i+1, "of", len(targets), "x", g.center.X, "y", g.center.Y,
			"saved_serve_tilt", tilt.pose.Point(), "station", sw.Name().ShortName())
		if err := b.pourFromShakerAt(ctx, sw, pourMs,
			withXY(approach, g.center.X, g.center.Y),
			withXY(tilt, g.center.X, g.center.Y)); err != nil {
			return poured, skipped, fmt.Errorf("glass %d of %d at (%.0f, %.0f): %w", i+1, len(targets), g.center.X, g.center.Y, err)
		}
		poured = append(poured, g)
	}
	return poured, skipped, nil
}
