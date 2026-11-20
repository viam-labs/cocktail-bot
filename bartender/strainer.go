package bartender

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	toggleswitch "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	poseStrainerHover    = "strainer-hover"
	poseStrainerApproach = "strainer-approach"
	poseStrainerSeat     = "strainer-seat"
	poseStrainerLock     = "strainer-lock"
	poseStrainerRetreat  = "strainer-retreat"
	poseGarbageHover     = "garbage-hover"
	poseGarbageTilt      = "garbage-tilt"
)

const heldStrainerFrameName = "held-strainer"

// strain fits the hand strainer onto the shaker, picking the shaker up from shaker-home first when a stir
// left it there.
func (b *bartender) strain(ctx context.Context, d *RecipeDefaults) error {
	if b.heldGeomFrame == nil {
		shakerSw, err := b.findSwitch(d.ShakerSource)
		if err != nil {
			return fmt.Errorf("shaker source switch %q: %w", d.ShakerSource, err)
		}
		if err := b.pickupShaker(ctx, shakerSw, "shaker-home"); err != nil {
			return fmt.Errorf("pickup shaker before strain: %w", err)
		}
	}
	flowSw, err := b.findSwitch(d.StrainFlow)
	if err != nil {
		return fmt.Errorf("strain_flow switch %q: %w", d.StrainFlow, err)
	}
	return b.attachStrainer(ctx, flowSw)
}

// attachStrainer brings the held shaker up under the strainer on its mount, twists to lock the strainer
// on, and backs away carrying it. The strainer rides on the shaker from strainer-seat on.
func (b *bartender) attachStrainer(ctx context.Context, flowSw toggleswitch.Switch) error {
	if b.heldGeomFrame == nil {
		return errors.New("attach strainer: the shaker must be in the gripper")
	}
	if b.heldStrainerFrame != nil {
		return errors.New("attach strainer: already carrying the strainer")
	}
	shield := b.strainerShieldAllowed()
	carrying := slices.Concat(shield, b.carriedStrainerVsMountAllowed())
	if _, err := b.carryHeldLevel(ctx, flowSw, poseStrainerHover); err != nil {
		return fmt.Errorf("carry to strainer-hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, flowSw, poseStrainerApproach, shield...); err != nil {
		return fmt.Errorf("linear-under the strainer at strainer-approach: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, flowSw, poseStrainerSeat, slices.Concat(shield, b.strainerOnMountAllowed())...); err != nil {
		return fmt.Errorf("linear-lift into the strainer at strainer-seat: %w", err)
	}
	if err := b.attachHeldStrainer(ctx); err != nil {
		return err
	}
	if _, err := b.linearCarryToPose(ctx, flowSw, poseStrainerLock, carrying...); err != nil {
		return fmt.Errorf("twist to lock the strainer at strainer-lock: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, flowSw, poseStrainerRetreat, carrying...); err != nil {
		return fmt.Errorf("linear-retreat off the mount to strainer-retreat: %w", err)
	}
	return nil
}

// returnStrainer runs the attach moves in reverse: back onto the mount, twist to unlock, leave the
// strainer there, and lower the shaker out from under it.
func (b *bartender) returnStrainer(ctx context.Context, flowSw toggleswitch.Switch) error {
	if b.heldStrainerFrame == nil {
		return errors.New("return strainer: not carrying the strainer")
	}
	shield := b.strainerShieldAllowed()
	carrying := slices.Concat(shield, b.carriedStrainerVsMountAllowed())
	if _, err := b.carryHeldLevel(ctx, flowSw, poseStrainerRetreat, carrying...); err != nil {
		return fmt.Errorf("carry to strainer-retreat: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, flowSw, poseStrainerLock, carrying...); err != nil {
		return fmt.Errorf("linear-onto the mount at strainer-lock: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, flowSw, poseStrainerSeat, carrying...); err != nil {
		return fmt.Errorf("twist to unlock the strainer at strainer-seat: %w", err)
	}
	b.detachHeldStrainer()
	if _, err := b.linearCarryToPose(ctx, flowSw, poseStrainerApproach, slices.Concat(shield, b.strainerOnMountAllowed())...); err != nil {
		return fmt.Errorf("linear-lower out of the strainer to strainer-approach: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, flowSw, poseStrainerHover, shield...); err != nil {
		return fmt.Errorf("linear-retreat to strainer-hover: %w", err)
	}
	return nil
}

// dumpShakerIce tips the ice the strainer held back into the garbage.
func (b *bartender) dumpShakerIce(ctx context.Context, flowSw toggleswitch.Switch, dumpMs int) error {
	if _, err := b.carryHeldLevel(ctx, flowSw, poseGarbageHover); err != nil {
		return fmt.Errorf("carry to garbage-hover: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, flowSw, poseGarbageTilt); err != nil {
		return fmt.Errorf("tilt shaker over garbage: %w", err)
	}
	if err := sleepCtx(ctx, time.Duration(dumpMs)*time.Millisecond); err != nil {
		return fmt.Errorf("dump dwell: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, flowSw, poseGarbageHover); err != nil {
		return fmt.Errorf("untilt shaker at garbage-hover: %w", err)
	}
	return nil
}

// strainerShieldAllowed lets the arm, gripper, shaker and strainer into the shield around the mount.
func (b *bartender) strainerShieldAllowed() []AllowedCollision {
	if b.cfg.StrainerMountShieldObstacle == "" {
		return nil
	}
	return b.gripperAllowedVsObstacle(b.cfg.StrainerMountShieldObstacle)
}

// strainerOnMountAllowed lets the shaker seat into the strainer while it still sits on its mount.
func (b *bartender) strainerOnMountAllowed() []AllowedCollision {
	return b.gripperAllowedVsObstacle(b.cfg.StrainerObstacle)
}

// carriedStrainerVsMountAllowed lets the strainer on the shaker touch the mount it is leaving or rejoining.
func (b *bartender) carriedStrainerVsMountAllowed() []AllowedCollision {
	if b.cfg.StrainerMountObstacle == "" {
		return nil
	}
	return []AllowedCollision{{Frame1: heldStrainerFrameName, Frame2: b.cfg.StrainerMountObstacle}}
}

// attachHeldStrainer makes the strainer obstacle part of what the gripper carries, where it sits now, so
// it twists and tilts with the shaker. While it rides along, its spot on the mount is left out of planning.
func (b *bartender) attachHeldStrainer(ctx context.Context) error {
	if b.cfg.StrainerObstacle == "" {
		return errors.New("attach strainer: strainer_obstacle is not configured")
	}
	if b.heldStrainerFrame != nil {
		return errors.New("attach strainer: already carrying the strainer")
	}
	fs, inputs, err := b.currentInputs(ctx)
	if err != nil {
		return err
	}
	parent := fs.Frame(gripPointFrameName)
	if parent == nil {
		parent = fs.Frame(b.cfg.GripperName)
	}
	if parent == nil {
		return fmt.Errorf("attach strainer: no %q or %q frame to carry it from", gripPointFrameName, b.cfg.GripperName)
	}
	frame, err := carriedObstacleFrame(fs, inputs, parent.Name(), b.cfg.StrainerObstacle, heldStrainerFrameName)
	if err != nil {
		return fmt.Errorf("attach strainer: %w", err)
	}
	b.heldStrainerFrame = frame
	return nil
}

// Idempotent so release paths don't branch on whether the strainer was attached.
func (b *bartender) detachHeldStrainer() {
	b.heldStrainerFrame = nil
}

// carriedObstacleFrame is a frame named name, to hang off parent, holding obstacle's geometry exactly
// where the obstacle is now.
func carriedObstacleFrame(
	fs *referenceframe.FrameSystem, inputs referenceframe.FrameSystemInputs, parent, obstacle, name string,
) (referenceframe.Frame, error) {
	all, err := referenceframe.FrameSystemGeometries(fs, inputs)
	if err != nil {
		return nil, fmt.Errorf("obstacle geometries: %w", err)
	}
	var found []spatialmath.Geometry
	// A configured geometry lives in the part's "<name>_origin" frame and is labeled with that frame's name.
	for _, frame := range []string{obstacle, obstacle + "_origin"} {
		if inFrame, ok := all[frame]; ok {
			found = append(found, inFrame.Geometries()...)
		}
	}
	if len(found) != 1 {
		return nil, fmt.Errorf("obstacle %q has %d geometries, need exactly 1", obstacle, len(found))
	}
	parentTF, err := fs.Transform(inputs.ToLinearInputs(), referenceframe.NewZeroPoseInFrame(parent), referenceframe.World)
	if err != nil {
		return nil, fmt.Errorf("locate %q: %w", parent, err)
	}
	inParent := found[0].Transform(spatialmath.PoseInverse(parentTF.(*referenceframe.PoseInFrame).Pose()))
	inParent.SetLabel(name)
	// rdk places a static frame's geometry at its parent, ignoring the frame's own pose; see CLAUDE.md.
	return referenceframe.NewStaticFrameWithGeometry(name, spatialmath.NewZeroPose(), inParent)
}
