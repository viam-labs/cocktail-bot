package bartender

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	pourLipFrameName       = "pour-lip"
	defaultLipTiltDegs     = 110.0
	defaultLipStepDegs     = 2.0
	lipLineToleranceMM     = 0.5
	lipOrientToleranceDegs = 1.0
	// Upright refuses if the lip has drifted this far since the pour, i.e. the arm was moved in between.
	lipDriftLimitMM = 5.0
)

type pourAboutLipReq struct {
	prePourPose string
	// Both in the gripper frame; the cup must be upright when the tilt starts.
	lipMM          r3.Vector
	rimCenterMM    r3.Vector
	tiltDegs       float64
	stepDegs       float64
	velDegsPerSec  float64
	accDegsPerSec2 float64
}

// lipPour is what upright_about_lip needs to retrace a pour about the same world point and axis.
type lipPour struct {
	lipOffsetMM r3.Vector
	lip         r3.Vector
	axis        r3.Vector
	startOrient spatialmath.Orientation
	tiltDegs    float64
	stepDegs    float64
	opts        *arm.MoveOptions
}

// lipTiltAxis returns the world axis that tips the top of an upright cup toward the lip: horizontal and
// tangent to the rim at the lip, so rotating about it leaves the lip where it is.
func lipTiltAxis(lip, rimCenter r3.Vector) (r3.Vector, error) {
	outward := lip.Sub(rimCenter)
	outward.Z = 0
	if outward.Norm() < 1 {
		return r3.Vector{}, errors.New("lip_mm and rim_center_mm are less than 1 mm apart horizontally; cannot tell which way to pour")
	}
	return r3.Vector{Z: 1}.Cross(outward).Normalize(), nil
}

// lipTiltPoses returns poses for the lip frame from fromDegs (exclusive) to toDegs (inclusive) of rotation
// about axis, every at most stepDegs. The point never changes, so the lip stays put while the cup turns.
func lipTiltPoses(lip, axis r3.Vector, startOrient spatialmath.Orientation, fromDegs, toDegs, stepDegs float64) []spatialmath.Pose {
	steps := int(math.Ceil(math.Abs(toDegs-fromDegs) / stepDegs))
	start := spatialmath.NewPoseFromOrientation(startOrient)
	poses := make([]spatialmath.Pose, 0, steps)
	for i := 1; i <= steps; i++ {
		degs := fromDegs + (toDegs-fromDegs)*float64(i)/float64(steps)
		rot := &spatialmath.R4AA{Theta: degs * math.Pi / 180, RX: axis.X, RY: axis.Y, RZ: axis.Z}
		// Pre-multiply so the rotation is about the world axis, not the lip frame's own.
		orient := spatialmath.Compose(spatialmath.NewPoseFromOrientation(rot), start).Orientation()
		poses = append(poses, spatialmath.NewPose(lip, orient))
	}
	return poses
}

func tiltFromStartDegs(startOrient, now spatialmath.Orientation) float64 {
	rel := spatialmath.Compose(
		spatialmath.NewPoseFromOrientation(now),
		spatialmath.PoseInverse(spatialmath.NewPoseFromOrientation(startOrient)),
	).Orientation().AxisAngles()
	return math.Abs(rel.Theta) * 180 / math.Pi
}

func (b *bartender) lipFrameSystem(ctx context.Context, lipOffsetMM r3.Vector) (*referenceframe.FrameSystem, referenceframe.FrameSystemInputs, error) {
	fs, fsInputs, err := b.currentInputs(ctx)
	if err != nil {
		return nil, nil, err
	}
	lipFrame, err := referenceframe.NewStaticFrame(pourLipFrameName, spatialmath.NewPoseFromPoint(lipOffsetMM))
	if err != nil {
		return nil, nil, err
	}
	if err := fs.AddFrame(lipFrame, fs.Frame(b.cfg.GripperName)); err != nil {
		return nil, nil, fmt.Errorf("attach %s to %s: %w", pourLipFrameName, b.cfg.GripperName, err)
	}
	return fs, fsInputs, nil
}

func (b *bartender) worldPoseIn(fs *referenceframe.FrameSystem, fsInputs referenceframe.FrameSystemInputs, frame string, pt r3.Vector) (spatialmath.Pose, error) {
	tf, err := fs.Transform(fsInputs.ToLinearInputs(), referenceframe.NewPoseInFrame(frame, spatialmath.NewPoseFromPoint(pt)), referenceframe.World)
	if err != nil {
		return nil, fmt.Errorf("transform %s to world: %w", frame, err)
	}
	return tf.(*referenceframe.PoseInFrame).Pose(), nil
}

// Tilts the held cup about its lip, leaving it tilted until uprightAboutLip. Only one pour can be
// outstanding since there is one arm.
func (b *bartender) pourAboutLip(ctx context.Context, req pourAboutLipReq) error {
	b.lipPourMu.Lock()
	defer b.lipPourMu.Unlock()
	if b.lipPour != nil {
		return errors.New("already tilted about a lip; run upright_about_lip first")
	}
	if req.prePourPose != "" {
		if _, err := b.carryHeldToUniversal(ctx, req.prePourPose); err != nil {
			return fmt.Errorf("carry to %s: %w", req.prePourPose, err)
		}
	}
	fs, fsInputs, err := b.lipFrameSystem(ctx, req.lipMM)
	if err != nil {
		return err
	}
	lip, err := b.worldPoseIn(fs, fsInputs, pourLipFrameName, r3.Vector{})
	if err != nil {
		return err
	}
	rimCenter, err := b.worldPoseIn(fs, fsInputs, b.cfg.GripperName, req.rimCenterMM)
	if err != nil {
		return err
	}
	axis, err := lipTiltAxis(lip.Point(), rimCenter.Point())
	if err != nil {
		return err
	}
	opts := moveOptionsFromCfg(req.velDegsPerSec, req.accDegsPerSec2)
	if opts == nil {
		opts = b.pourMoveOptions()
	}
	p := &lipPour{
		lipOffsetMM: req.lipMM,
		lip:         lip.Point(),
		axis:        axis,
		startOrient: lip.Orientation(),
		tiltDegs:    req.tiltDegs,
		stepDegs:    req.stepDegs,
		opts:        opts,
	}
	b.logger.Infow("pouring about lip", "lip", p.lip, "axis", p.axis, "tilt_degs", p.tiltDegs)
	moved, err := b.runLipTilt(ctx, fs, fsInputs, p, 0, p.tiltDegs, "pour-about-lip")
	if moved {
		b.lipPour = p
	}
	return err
}

// Retraces the outstanding pour from wherever it stopped (including a cancelled or failed pour) back to upright.
func (b *bartender) uprightAboutLip(ctx context.Context) error {
	b.lipPourMu.Lock()
	defer b.lipPourMu.Unlock()
	p := b.lipPour
	if p == nil {
		return errors.New("no pour_about_lip to return from")
	}
	fs, fsInputs, err := b.lipFrameSystem(ctx, p.lipOffsetMM)
	if err != nil {
		return err
	}
	lip, err := b.worldPoseIn(fs, fsInputs, pourLipFrameName, r3.Vector{})
	if err != nil {
		return err
	}
	if drift := lip.Point().Distance(p.lip); drift > lipDriftLimitMM {
		b.lipPour = nil
		return fmt.Errorf("lip is %.0f mm from where the pour left it; the arm was moved, so return it upright by hand", drift)
	}
	from := math.Min(tiltFromStartDegs(p.startOrient, lip.Orientation()), p.tiltDegs)
	if _, err := b.runLipTilt(ctx, fs, fsInputs, p, from, 0, "upright-about-lip"); err != nil {
		return err
	}
	b.lipPour = nil
	return nil
}

// runLipTilt reports whether the arm was commanded, so callers know if the cup may have left upright.
func (b *bartender) runLipTilt(
	ctx context.Context, fs *referenceframe.FrameSystem, fsInputs referenceframe.FrameSystemInputs,
	p *lipPour, fromDegs, toDegs float64, label string,
) (bool, error) {
	poses := lipTiltPoses(p.lip, p.axis, p.startOrient, fromDegs, toDegs, p.stepDegs)
	if len(poses) == 0 {
		return false, nil
	}
	goals := make([]*armplanning.PlanState, len(poses))
	for i, pose := range poses {
		goals[i] = armplanning.NewPlanState(referenceframe.FrameSystemPoses{
			pourLipFrameName: referenceframe.NewPoseInFrame(referenceframe.World, pose),
		}, nil)
	}
	req := &armplanning.PlanRequest{
		FrameSystem: fs,
		StartState:  armplanning.NewPlanState(nil, fsInputs),
		Goals:       goals,
		Constraints: &motionplan.Constraints{
			LinearConstraint:      []motionplan.LinearConstraint{{LineToleranceMm: lipLineToleranceMM}},
			OrientationConstraint: []motionplan.OrientationConstraint{{OrientationToleranceDegs: lipOrientToleranceDegs}},
		},
	}
	plan, err := b.planMotion(ctx, req, label)
	if err != nil {
		return false, err
	}
	positions, err := plan.Trajectory().GetFrameInputs(b.cfg.ArmName)
	if err != nil {
		return false, fmt.Errorf("extract trajectory: %w", err)
	}
	if err := b.arm.MoveThroughJointPositions(ctx, positions, p.opts, nil); err != nil {
		return true, fmt.Errorf("execute %s: %w", label, err)
	}
	return true, nil
}
