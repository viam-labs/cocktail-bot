package bartender

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/spatialmath"
)

const defaultMaxPourOffsetMM = 300.0

type GlassXY struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (g GlassXY) vec() r3.Vector { return r3.Vector{X: g.X, Y: g.Y} }

type pourIntoGlassesReq struct {
	bottle  string
	pourMs  int
	glasses []GlassXY
}

// pourOffsets returns the world XY shift from the reference glass to each glass, rejecting any shift
// larger than maxOffsetMM so a bad detection can't send a full bottle across the workspace.
func pourOffsets(ref GlassXY, glasses []GlassXY, maxOffsetMM float64) ([]r3.Vector, error) {
	offsets := make([]r3.Vector, len(glasses))
	for i, g := range glasses {
		off := g.vec().Sub(ref.vec())
		if off.Norm() > maxOffsetMM {
			return nil, fmt.Errorf("glass %d at (%.0f, %.0f) is %.0f mm from pour_reference_glass, over max_pour_offset_mm %.0f",
				i, g.X, g.Y, off.Norm(), maxOffsetMM)
		}
		offsets[i] = off
	}
	return offsets, nil
}

func shiftedPose(pd *poseData, offset r3.Vector) *poseData {
	return &poseData{
		pose:          spatialmath.NewPose(pd.pose.Point().Add(offset), pd.pose.Orientation()),
		refFrame:      pd.refFrame,
		componentName: pd.componentName,
	}
}

// Replays the saved pour-approach/pour-tilt poses translated in world XY so the pour that works for
// the reference glass lands on each detected glass, with one bottle pickup for all of them.
func (b *bartender) pourIntoGlasses(ctx context.Context, req pourIntoGlassesReq) error {
	if b.cfg.PourReferenceGlass == nil {
		return errors.New("pour_into_glasses: pour_reference_glass is not configured")
	}
	offsets, err := pourOffsets(*b.cfg.PourReferenceGlass, req.glasses, b.cfg.maxPourOffsetMM())
	if err != nil {
		return err
	}
	bottleSw, err := b.findSwitch(req.bottle)
	if err != nil {
		return err
	}
	approach, _, err := b.resolvePose(ctx, posePourApproach)
	if err != nil {
		return err
	}
	tilt, _, err := b.resolvePose(ctx, posePourTilt)
	if err != nil {
		return err
	}
	if approach, err = b.poseInWorld(ctx, approach); err != nil {
		return err
	}
	if tilt, err = b.poseInWorld(ctx, tilt); err != nil {
		return err
	}

	if err := b.pickupBottle(ctx, bottleSw); err != nil {
		return err
	}
	pourOpts := b.pourMoveOptions()
	for i, off := range offsets {
		label := fmt.Sprintf("glass%d", i)
		glassApproach := shiftedPose(approach, off)
		b.logger.Infow("pouring", "glass", i, "x", req.glasses[i].X, "y", req.glasses[i].Y, "offset", off)
		if _, err := b.carryHeldLevelToResolved(ctx, glassApproach, label+":"+posePourApproach+":carry"); err != nil {
			return fmt.Errorf("carry to %s pour-approach: %w", label, err)
		}
		if _, err := b.moveToResolvedPose(ctx, shiftedPose(tilt, off), label+":"+posePourTilt, nil, pourOpts); err != nil {
			return fmt.Errorf("%s pour-tilt: %w", label, err)
		}
		if err := sleepCtx(ctx, time.Duration(req.pourMs)*time.Millisecond); err != nil {
			return fmt.Errorf("%s pour dwell: %w", label, err)
		}
		if _, err := b.moveToResolvedPose(ctx, glassApproach, label+":pour-upright", nil, pourOpts); err != nil {
			return fmt.Errorf("%s pour-upright: %w", label, err)
		}
	}
	return b.returnBottle(ctx, bottleSw)
}

func parsePourIntoGlasses(raw any) (pourIntoGlassesReq, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return pourIntoGlassesReq{}, fmt.Errorf("pour_into_glasses: expected object with 'bottle', 'pour_ms' and 'glasses', got %T", raw)
	}
	bottle, pourMs, err := parsePickupPourReturn(m)
	if err != nil {
		return pourIntoGlassesReq{}, fmt.Errorf("pour_into_glasses: %w", err)
	}
	rawGlasses, ok := m["glasses"].([]any)
	if !ok || len(rawGlasses) == 0 {
		return pourIntoGlassesReq{}, errors.New("pour_into_glasses: 'glasses' must be a non-empty list of {x, y}")
	}
	glasses := make([]GlassXY, len(rawGlasses))
	for i, rg := range rawGlasses {
		g, ok := rg.(map[string]any)
		if !ok {
			return pourIntoGlassesReq{}, fmt.Errorf("pour_into_glasses: glasses[%d] must be an object with 'x' and 'y'", i)
		}
		x, okX := g["x"].(float64)
		y, okY := g["y"].(float64)
		if !okX || !okY {
			return pourIntoGlassesReq{}, fmt.Errorf("pour_into_glasses: glasses[%d] needs numeric 'x' and 'y' (world mm)", i)
		}
		glasses[i] = GlassXY{X: x, Y: y}
	}
	return pourIntoGlassesReq{bottle: bottle, pourMs: pourMs, glasses: glasses}, nil
}
