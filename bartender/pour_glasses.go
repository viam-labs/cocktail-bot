package bartender

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/spatialmath"
)

const (
	defaultMaxPourOffsetMM   = 300.0
	defaultPourMouthOffsetMM = 100.0
)

type GlassXY struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (g GlassXY) vec() r3.Vector { return r3.Vector{X: g.X, Y: g.Y} }

type pourIntoGlassesReq struct {
	bottle  string
	pourMs  int
	glasses []GlassXY
	// nil = pour_mouth_offset_mm from config.
	mouthOffsetMM *float64
}

const minPourTiltRad = 5 * math.Pi / 180

// leanDirection is the horizontal direction the top of an upright bottle moves when the gripper rotates
// from pour-approach to pour-tilt. It uses the world-frame rotation between the two poses, so it holds
// whether the pour swings the gripper or twists the wrist about the gripper's own axis.
func leanDirection(approach, tilt spatialmath.Pose) (r3.Vector, error) {
	rel := spatialmath.Compose(
		spatialmath.NewPoseFromOrientation(tilt.Orientation()),
		spatialmath.PoseInverse(spatialmath.NewPoseFromOrientation(approach.Orientation())),
	).Orientation().AxisAngles()
	if math.Abs(rel.Theta) < minPourTiltRad {
		return r3.Vector{}, fmt.Errorf("pour-approach and pour-tilt differ by only %.1f° of rotation; cannot tell which way the bottle tips",
			rel.Theta*180/math.Pi)
	}
	axis := r3.Vector{X: rel.RX, Y: rel.RY, Z: rel.RZ}.Mul(math.Copysign(1, rel.Theta))
	// A point above the pivot moves along axis × up for a positive rotation.
	lean := axis.Cross(r3.Vector{Z: 1})
	lean.Z = 0
	if lean.Norm() < 0.1 {
		return r3.Vector{}, errors.New("pour-approach to pour-tilt rotates about the vertical, which does not tip the bottle")
	}
	return lean.Normalize(), nil
}

// pourShifts returns, per glass, the world XY translation of the saved pour poses that puts the bottle
// mouth (mouthOffsetMM from the gripper along lean) above the glass. Shifts over maxShiftMM are rejected
// so a bad detection can't send a full bottle across the workspace.
func pourShifts(glasses []GlassXY, tiltXY, lean r3.Vector, mouthOffsetMM, maxShiftMM float64) ([]r3.Vector, error) {
	tiltXY.Z = 0
	shifts := make([]r3.Vector, len(glasses))
	for i, g := range glasses {
		gripper := g.vec().Sub(lean.Mul(mouthOffsetMM))
		shift := gripper.Sub(tiltXY)
		if shift.Norm() > maxShiftMM {
			return nil, fmt.Errorf("glass %d at (%.0f, %.0f) needs the pour moved %.0f mm from the saved pour-tilt, over max_pour_offset_mm %.0f",
				i, g.X, g.Y, shift.Norm(), maxShiftMM)
		}
		shifts[i] = shift
	}
	return shifts, nil
}

func shiftedPose(pd *poseData, offset r3.Vector) *poseData {
	return &poseData{
		pose:          spatialmath.NewPose(pd.pose.Point().Add(offset), pd.pose.Orientation()),
		refFrame:      pd.refFrame,
		componentName: pd.componentName,
	}
}

// Replays the saved pour-approach/pour-tilt poses translated in world XY so the bottle mouth lands above
// each glass, with one bottle pickup for all of them. Height and tilt stay as saved.
func (b *bartender) pourIntoGlasses(ctx context.Context, req pourIntoGlassesReq) error {
	mouthOffsetMM := b.cfg.pourMouthOffsetMM()
	if req.mouthOffsetMM != nil {
		mouthOffsetMM = *req.mouthOffsetMM
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
	lean, err := leanDirection(approach.pose, tilt.pose)
	if err != nil {
		return err
	}
	offsets, err := pourShifts(req.glasses, tilt.pose.Point(), lean, mouthOffsetMM, b.cfg.maxPourOffsetMM())
	if err != nil {
		return err
	}

	if err := b.pickupBottle(ctx, bottleSw); err != nil {
		return err
	}
	pourOpts := b.pourMoveOptions()
	for i, off := range offsets {
		label := fmt.Sprintf("glass%d", i)
		glassApproach := shiftedPose(approach, off)
		b.logger.Infow("pouring", "glass", i, "x", req.glasses[i].X, "y", req.glasses[i].Y, "shift", off, "lean", lean, "mouth_offset_mm", mouthOffsetMM)
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
	mouthOffsetMM, err := parseMouthOffset(m)
	if err != nil {
		return pourIntoGlassesReq{}, fmt.Errorf("pour_into_glasses: %w", err)
	}
	return pourIntoGlassesReq{bottle: bottle, pourMs: pourMs, glasses: glasses, mouthOffsetMM: mouthOffsetMM}, nil
}

// parseMouthOffset reads the optional per-command 'mouth_offset_mm' (nil when absent).
func parseMouthOffset(m map[string]any) (*float64, error) {
	raw, ok := m["mouth_offset_mm"]
	if !ok {
		return nil, nil
	}
	v, ok := raw.(float64)
	if !ok {
		return nil, fmt.Errorf("'mouth_offset_mm' must be a number, got %T", raw)
	}
	return &v, nil
}
