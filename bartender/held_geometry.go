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

// buildHeldObjectFrame attaches the held cylinder to the gripper with an
// orientation offset so the frame's +Z axis aligns with the object's long axis
// in the world at attach time. The no-spill carry constraint applies to this
// frame's orientation, so without that rotation the planner would be bounding
// the gripper's up vector, not the object's.
func buildHeldObjectFrame(g *HeldObjectGeometry, orientationInGripper spatialmath.Orientation) (referenceframe.Frame, error) {
	if g == nil {
		return nil, nil
	}
	geom, err := spatialmath.NewCapsule(
		spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: -g.ZOffsetMM}, &spatialmath.OrientationVectorDegrees{OZ: 1}),
		g.RadiusMM, g.LengthMM, heldObjectFrameName,
	)
	if err != nil {
		return nil, fmt.Errorf("build held-object geometry: %w", err)
	}
	if orientationInGripper == nil {
		orientationInGripper = &spatialmath.OrientationVectorDegrees{OZ: 1}
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

// Idempotent so release paths don't branch on held-state.
func (b *bartender) detachHeld() {
	b.heldGeomFrame = nil
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
