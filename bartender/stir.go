package bartender

import (
	"context"
	"fmt"
	"time"
)

const (
	poseStirStorageHover = "stir-storage-hover"
	poseStirStoragePlace = "stir-storage-place"
	poseStirHover        = "stir-hover"
	poseStirInsert       = "stir-insert"

	defaultStirStation = "stir"

	stirrerPickupKey = "stirrer"
)

// stir takes the stirrer from storage, stirs in the shaker at shaker-home, and puts the stirrer back.
// The held stirrer geometry is attached from gripper close to gripper open.
func (b *bartender) stir(ctx context.Context, stationSwitchName string, dwellMs int) error {
	sw, err := b.findSwitch(stationSwitchName)
	if err != nil {
		return err
	}
	storageAllow := b.pickupAllowedCollisions(stirrerPickupKey)
	shakerAllow := b.pickupAllowedCollisions("shaker")

	if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseStirStorageHover); err != nil {
		return fmt.Errorf("stir-storage-hover: %w", err)
	}
	if err := b.gripperOpen(ctx); err != nil {
		return fmt.Errorf("open gripper before stirrer grab: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseStirStoragePlace, storageAllow...); err != nil {
		return fmt.Errorf("linear-to stir-storage-place: %w", err)
	}
	if _, err := b.gripper.Grab(ctx, nil); err != nil {
		return fmt.Errorf("close gripper on stirrer: %w", err)
	}
	if err := b.attachHeld(ctx, b.cfg.HeldStirrerGeometry); err != nil {
		return fmt.Errorf("attach held stirrer: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseStirStorageHover, storageAllow...); err != nil {
		return fmt.Errorf("linear-lift stirrer to stir-storage-hover: %w", err)
	}
	if _, err := b.carryHeldLevel(ctx, sw, poseStirHover); err != nil {
		return fmt.Errorf("carry stirrer to stir-hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseStirInsert, shakerAllow...); err != nil {
		return fmt.Errorf("linear-insert stirrer to stir-insert: %w", err)
	}
	dwell := time.Duration(dwellMs) * time.Millisecond
	if err := b.circleAroundPose(ctx, sw, poseStirInsert, dwell, shakerAllow...); err != nil {
		return fmt.Errorf("stir around stir-insert: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseStirHover, shakerAllow...); err != nil {
		return fmt.Errorf("linear-retract stirrer to stir-hover: %w", err)
	}
	if _, err := b.carryHeldLevel(ctx, sw, poseStirStorageHover, storageAllow...); err != nil {
		return fmt.Errorf("carry stirrer back to stir-storage-hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseStirStoragePlace, storageAllow...); err != nil {
		return fmt.Errorf("linear-return stirrer to stir-storage-place: %w", err)
	}
	if err := b.gripperOpen(ctx); err != nil {
		return fmt.Errorf("release stirrer: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseStirStorageHover, storageAllow...); err != nil {
		return fmt.Errorf("linear-retreat to stir-storage-hover: %w", err)
	}
	return nil
}
