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
	parkingAllow := b.pickupAllowedCollisions("parking")
	receiveAllow := b.pickupAllowedCollisions("receive-shaker")

	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("start-home: %w", err)
	}

	if _, err := b.moveArmToPoseOnSwitch(ctx, flowSw, poseParkingHover); err != nil {
		return fmt.Errorf("parking-hover: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("open gripper before parking grab: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, flowSw, poseParkingGrab, parkingAllow...); err != nil {
		return fmt.Errorf("linear-to parking grab: %w", err)
	}
	if _, err := b.gripper.Grab(ctx, nil); err != nil {
		return fmt.Errorf("close gripper on parked shaker: %w", err)
	}
	if err := b.attachHeld(ctx, b.cfg.HeldShakerGeometry); err != nil {
		return fmt.Errorf("attach held shaker at parking: %w", err)
	}
	if _, err := b.carryHeldLevel(ctx, flowSw, poseParkingCarryHover, parkingAllow...); err != nil {
		return fmt.Errorf("carry to parking-carry-hover: %w", err)
	}

	if _, err := b.carryHeldLevel(ctx, flowSw, poseReceiveShakerCarryHover, receiveAllow...); err != nil {
		return fmt.Errorf("carry to receive-shaker-carry-hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, flowSw, poseReceiveShakerGrab, receiveAllow...); err != nil {
		return fmt.Errorf("linear-to receive-shaker grab: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("release shaker at receive-shaker: %w", err)
	}
	b.detachHeld()
	if _, err := b.linearMoveToPose(ctx, flowSw, poseReceiveShakerHover, receiveAllow...); err != nil {
		return fmt.Errorf("linear-retreat from receive-shaker: %w", err)
	}

	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("end-home: %w", err)
	}
	return nil
}
