package bartender

import (
	"context"
	"fmt"
	"time"
)

const (
	poseShakerHover      = "shaker-hover"
	poseShakerDrop       = "shaker-drop"
	poseShakerLift       = "shaker-lift"
	poseShakerCarryHover = "shaker-carry-hover"
	poseDepositDrop      = "deposit-drop"
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
	if _, err := b.linearMoveToPose(ctx, sw, poseShakerDrop); err != nil {
		return fmt.Errorf("linear-descend to shaker drop: %w", err)
	}
	if err := b.attachHeld(b.cfg.HeldShakerGeometry); err != nil {
		return fmt.Errorf("attach held shaker: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseShakerLift); err != nil {
		return fmt.Errorf("linear-lift shaker: %w", err)
	}
	if _, err := b.carryHeldLevel(ctx, sw, poseShakerCarryHover); err != nil {
		return fmt.Errorf("carry to shaker carry-hover: %w", err)
	}

	if _, err := b.carryHeldLevel(ctx, sw, poseHover); err != nil {
		return fmt.Errorf("carry to ice-station hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseDepositDrop); err != nil {
		return fmt.Errorf("linear-descend to deposit: %w", err)
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

	if _, err := b.linearMoveToPose(ctx, sw, poseDepositDrop); err != nil {
		return fmt.Errorf("linear-descend to deposited shaker: %w", err)
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
	if _, err := b.linearCarryToPose(ctx, sw, poseShakerLift); err != nil {
		return fmt.Errorf("carry down to shaker lift: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseShakerDrop); err != nil {
		return fmt.Errorf("linear-drop shaker: %w", err)
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
