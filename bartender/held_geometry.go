package bartender

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

type HeldObjectGeometry struct {
	Type      string  `json:"type"`
	RadiusMM  float64 `json:"radius_mm"`
	LengthMM  float64 `json:"length_mm"`
	ZOffsetMM float64 `json:"z_offset_mm"`
}

func (g *HeldObjectGeometry) Validate(path string) error {
	if g == nil {
		return nil
	}
	if g.Type != "cylinder" {
		return fmt.Errorf("%s.type must be \"cylinder\" (only cylinder supported now), got %q", path, g.Type)
	}
	if g.RadiusMM <= 0 || g.LengthMM <= 0 {
		return fmt.Errorf("%s.radius_mm and length_mm must be > 0", path)
	}
	return nil
}

const heldObjectFrameName = "held-object"
const parkedObjectFrameName = "parked-object"

const gripPointFrameName = "grip-point"

// buildHeldObjectFrame attaches the held cylinder to the gripper with an
// orientation offset so the frame's +Z axis aligns with the object's long axis
// in the world at attach time. The no-spill carry constraint applies to this
// frame's orientation, so without that rotation the planner would be bounding
// the gripper's up vector, not the object's.
func buildHeldObjectFrame(g *HeldObjectGeometry, orientationInGripper spatialmath.Orientation) (referenceframe.Frame, error) {
	if g == nil {
		return nil, nil
	}
	if orientationInGripper == nil {
		orientationInGripper = &spatialmath.OrientationVectorDegrees{OZ: 1}
	}
	desiredWorldPose := spatialmath.NewPose(
		r3.Vector{X: 0, Y: 0, Z: -g.ZOffsetMM},
		&spatialmath.OrientationVectorDegrees{OZ: 1},
	)
	localGeomPose := spatialmath.Compose(spatialmath.NewPoseFromOrientation(orientationInGripper), desiredWorldPose)
	geom, err := spatialmath.NewCapsule(localGeomPose, g.RadiusMM, g.LengthMM, heldObjectFrameName)
	if err != nil {
		return nil, fmt.Errorf("build held-object geometry: %w", err)
	}
	return referenceframe.NewStaticFrameWithGeometry(heldObjectFrameName, spatialmath.NewPoseFromOrientation(orientationInGripper), geom)
}

// attachHeld installs the held cylinder on the gripper. The frame's orientation
// is set so its +Z axis matches the object's long axis in the world at this
// moment — the shaker is physically upright at every attach point, so the
// frame's world orientation is identity, and relative-to-gripper that is the
// inverse of the gripper's current world rotation. The attach is a no-op when
// g is nil so caller code doesn't branch on config.
func (b *bartender) attachHeld(ctx context.Context, g *HeldObjectGeometry) error {
	if g == nil {
		return nil
	}
	if b.heldGeomFrame != nil {
		return errors.New("attachHeld: already holding an object; detach first")
	}
	orient, err := b.gripperOrientationInGripperFromWorldUp(ctx)
	if err != nil {
		return err
	}
	frame, err := buildHeldObjectFrame(g, orient)
	if err != nil {
		return err
	}
	b.heldGeomFrame = frame
	return nil
}

// Idempotent so release paths don't branch on held-state. A strainer or
// attachment riding on the held shaker leaves the arm with it.
func (b *bartender) detachHeld() {
	b.heldGeomFrame = nil
	b.heldStrainerFrame = nil
	b.heldShakerAttachmentFrame = nil
}

// attachParked models the parked bottle as standing upright on the table at
// gripPoint, wherever the gripper happened to point when it let go.
func (b *bartender) attachParked(gripPoint r3.Vector) error {
	g := b.cfg.ParkedBottleGeometry
	if g == nil {
		g = b.cfg.HeldBottleGeometry
	}
	if g == nil {
		return nil
	}
	if b.parkedGeomFrame != nil {
		return errors.New("attachParked: parked geometry already set; detach first")
	}
	center := gripPoint.Add(r3.Vector{Z: -g.ZOffsetMM})
	geom, err := spatialmath.NewCapsule(spatialmath.NewPoseFromPoint(center), g.RadiusMM, g.LengthMM, parkedObjectFrameName)
	if err != nil {
		return fmt.Errorf("build parked-bottle geometry: %w", err)
	}
	// rdk places a static frame's geometry at its parent, ignoring the frame's own pose; see CLAUDE.md.
	frame, err := referenceframe.NewStaticFrameWithGeometry(parkedObjectFrameName, spatialmath.NewZeroPose(), geom)
	if err != nil {
		return err
	}
	b.parkedGeomFrame = frame
	return nil
}

// gripPointAtPose is where the claws close when pd's component reaches pd.
// Poses usually target the arm's flange, which sits a tool length short of the bottle.
func (b *bartender) gripPointAtPose(ctx context.Context, pd *poseData) (r3.Vector, error) {
	pd, err := b.cartesianPose(ctx, pd)
	if err != nil {
		return r3.Vector{}, err
	}
	fs, fsInputs, err := b.currentInputs(ctx)
	if err != nil {
		return r3.Vector{}, err
	}
	li := fsInputs.ToLinearInputs()
	worldPose := func(frame string, pose spatialmath.Pose) (spatialmath.Pose, error) {
		tf, err := fs.Transform(li, referenceframe.NewPoseInFrame(frame, pose), referenceframe.World)
		if err != nil {
			return nil, fmt.Errorf("transform %q to world: %w", frame, err)
		}
		return tf.(*referenceframe.PoseInFrame).Pose(), nil
	}
	componentName := pd.componentName
	if componentName == "" {
		componentName = b.cfg.ArmName
	}
	gripFrame := gripPointFrameName
	if fs.Frame(gripFrame) == nil {
		gripFrame = b.cfg.GripperName
	}
	goal, err := worldPose(pd.refFrame, pd.pose)
	if err != nil {
		return r3.Vector{}, err
	}
	component, err := worldPose(componentName, spatialmath.NewZeroPose())
	if err != nil {
		return r3.Vector{}, err
	}
	grip, err := worldPose(gripFrame, spatialmath.NewZeroPose())
	if err != nil {
		return r3.Vector{}, err
	}
	return spatialmath.Compose(goal, spatialmath.PoseBetween(component, grip)).Point(), nil
}

func (b *bartender) detachParked() {
	b.parkedGeomFrame = nil
}

var errParkingOccupied = errors.New("bottle parking spot occupied. move bottle back and press Cancel on the stream deck")

func (b *bartender) checkParkingClear() error {
	if b.parkedGeomFrame != nil {
		return errParkingOccupied
	}
	return nil
}

// gripperOpen opens the gripper and detaches the held-object geometry — opening
// the claws means we are no longer holding whatever was attached. Call this
// instead of b.gripper.Open so the virtual capsule can't drift out of sync.
func (b *bartender) gripperOpen(ctx context.Context) error {
	if err := b.gripper.Open(ctx, nil); err != nil {
		return err
	}
	b.detachHeld()
	return nil
}

// Returns the rotation to apply inside the gripper's frame so a child frame's
// +Z axis points along world +Z — the inverse of the gripper's current world
// rotation.
func (b *bartender) gripperOrientationInGripperFromWorldUp(ctx context.Context) (spatialmath.Orientation, error) {
	fs, fsInputs, err := b.currentInputs(ctx)
	if err != nil {
		return nil, err
	}
	gripperInWorld, err := fs.Transform(
		fsInputs.ToLinearInputs(),
		referenceframe.NewPoseInFrame(b.cfg.GripperName, spatialmath.NewZeroPose()),
		referenceframe.World,
	)
	if err != nil {
		return nil, fmt.Errorf("transform gripper to world: %w", err)
	}
	gripperPose := gripperInWorld.(*referenceframe.PoseInFrame).Pose()
	return spatialmath.PoseBetween(gripperPose, spatialmath.NewZeroPose()).Orientation(), nil
}
