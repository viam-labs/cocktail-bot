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

	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("start-home: %w", err)
	}

	if _, err := b.moveArmToPoseOnSwitch(ctx, flowSw, poseShakerBHover); err != nil {
		return fmt.Errorf("shaker-b-hover: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("open gripper before shaker-b-place: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, flowSw, poseShakerBPlace, shakerBAllow...); err != nil {
		return fmt.Errorf("linear-to shaker-b-place: %w", err)
	}
	if _, err := b.gripper.Grab(ctx, nil); err != nil {
		return fmt.Errorf("close gripper on parked shaker: %w", err)
	}
	if err := b.attachHeld(ctx, b.cfg.HeldShakerGeometry); err != nil {
		return fmt.Errorf("attach held shaker from shaker-b: %w", err)
	}
	if _, err := b.carryHeldLevel(ctx, flowSw, poseShakerBHover, shakerBAllow...); err != nil {
		return fmt.Errorf("carry up to shaker-b-hover: %w", err)
	}

	if _, err := b.carryHeldLevel(ctx, flowSw, poseShakerACarryHover, shakerAAllow...); err != nil {
		return fmt.Errorf("carry to shaker-a-carry-hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, flowSw, poseShakerAApproach, shakerAAllow...); err != nil {
		return fmt.Errorf("linear-descend to shaker-a-approach: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("release shaker at shaker-a-approach: %w", err)
	}
	b.detachHeld()
	if _, err := b.linearMoveToPose(ctx, flowSw, poseShakerAHover, shakerAAllow...); err != nil {
		return fmt.Errorf("linear-retreat to shaker-a-hover: %w", err)
	}

	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("end-home: %w", err)
	}
	return nil
}
