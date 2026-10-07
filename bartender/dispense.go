package bartender

import (
	"context"
	"fmt"
	"time"
)

const (
	poseShakerHover      = "shaker-hover"
	poseShakerGrab       = "shaker-grab"
	poseShakerCarryHover = "shaker-carry-hover"
	poseDeposit          = "deposit"
	posePull             = "pull"
)

func (b *bartender) dispenseIce(ctx context.Context, stationSwitchName string, dwellMs int) error {
	sw, err := b.findSwitch(stationSwitchName)
	if err != nil {
		return err
	}

	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("start-home: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseShakerHover); err != nil {
		return fmt.Errorf("shaker-hover: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("open gripper: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseShakerGrab); err != nil {
		return fmt.Errorf("linear-grab shaker: %w", err)
	}
	if _, err := b.gripper.Grab(ctx, nil); err != nil {
		return fmt.Errorf("close gripper on shaker: %w", err)
	}
	if err := b.attachHeld(b.cfg.HeldShakerGeometry); err != nil {
		return fmt.Errorf("attach held shaker: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseShakerCarryHover); err != nil {
		return fmt.Errorf("linear-lift shaker: %w", err)
	}

	if _, err := b.carryHeldLevel(ctx, sw, poseHover); err != nil {
		return fmt.Errorf("carry to ice-station hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseDeposit); err != nil {
		return fmt.Errorf("linear-deposit shaker: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("release shaker at deposit: %w", err)
	}
	b.detachHeld()
	if _, err := b.linearMoveToPose(ctx, sw, poseHover); err != nil {
		return fmt.Errorf("linear-retreat from deposit: %w", err)
	}

	if _, err := b.linearMoveToPose(ctx, sw, poseGrab); err != nil {
		return fmt.Errorf("engage lever: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, posePull); err != nil {
		return fmt.Errorf("pull lever: %w", err)
	}
	if err := sleepCtx(ctx, time.Duration(dwellMs)*time.Millisecond); err != nil {
		return fmt.Errorf("ice dwell: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseGrab); err != nil {
		return fmt.Errorf("release lever: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseHover); err != nil {
		return fmt.Errorf("retreat from lever: %w", err)
	}

	if _, err := b.linearMoveToPose(ctx, sw, poseDeposit); err != nil {
		return fmt.Errorf("linear-descend to deposited shaker: %w", err)
	}
	if _, err := b.gripper.Grab(ctx, nil); err != nil {
		return fmt.Errorf("close gripper on shaker with ice: %w", err)
	}
	if err := b.attachHeld(b.cfg.HeldShakerGeometry); err != nil {
		return fmt.Errorf("attach held shaker with ice: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseHover); err != nil {
		return fmt.Errorf("linear-lift shaker with ice: %w", err)
	}

	if _, err := b.carryHeldLevel(ctx, sw, poseShakerCarryHover); err != nil {
		return fmt.Errorf("carry back to shaker carry-hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseShakerGrab); err != nil {
		return fmt.Errorf("linear-return shaker: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("release shaker at home: %w", err)
	}
	b.detachHeld()
	if _, err := b.linearMoveToPose(ctx, sw, poseShakerHover); err != nil {
		return fmt.Errorf("linear-retreat from shaker: %w", err)
	}
	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("end-home: %w", err)
	}
	return nil
}
