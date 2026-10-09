package bartender

import (
	"context"
	"fmt"
)

func (b *bartender) rotateShakers(ctx context.Context, strainFlow string) error {
	flowSw, err := b.findSwitch(strainFlow)
	if err != nil {
		return fmt.Errorf("strain_flow switch: %w", err)
	}
	shakerAAllow := b.pickupAllowedCollisions("shaker-a")
	shakerBAllow := b.pickupAllowedCollisions("shaker-b")

	if _, err := b.moveArmToPoseOnSwitch(ctx, flowSw, poseShakerBHover); err != nil {
		return fmt.Errorf("shaker-b-hover: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("open gripper before shaker B grab: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, flowSw, poseShakerBDescend, shakerBAllow...); err != nil {
		return fmt.Errorf("linear-to shaker-b-descend: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, flowSw, poseShakerBPlace, shakerBAllow...); err != nil {
		return fmt.Errorf("linear-to shaker-b-place: %w", err)
	}
	if _, err := b.gripper.Grab(ctx, nil); err != nil {
		return fmt.Errorf("close gripper on shaker B: %w", err)
	}
	if err := b.attachHeld(ctx, b.cfg.HeldShakerGeometry); err != nil {
		return fmt.Errorf("attach held shaker from shaker-b: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, flowSw, poseShakerBHoverAbove, shakerBAllow...); err != nil {
		return fmt.Errorf("linear-lift to shaker-b-hover-above: %w", err)
	}

	if _, err := b.carryHeldLevel(ctx, flowSw, poseFilterAHoverAbove, shakerAAllow...); err != nil {
		return fmt.Errorf("carry to filter-a-hover-above: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, flowSw, poseShakerALift, shakerAAllow...); err != nil {
		return fmt.Errorf("linear-descend to shaker-a-lift: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("release shaker at shaker-a-lift: %w", err)
	}
	b.detachHeld()
	if _, err := b.linearMoveToPose(ctx, flowSw, poseShakerAApproach, shakerAAllow...); err != nil {
		return fmt.Errorf("linear-to shaker-a-approach: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, flowSw, poseShakerAHover, shakerAAllow...); err != nil {
		return fmt.Errorf("linear-retreat to shaker-a-hover: %w", err)
	}

	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("end-home: %w", err)
	}
	return nil
}
