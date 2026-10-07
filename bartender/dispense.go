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
	poseDepositApproach  = "deposit-approach"
	poseDepositPlace     = "deposit-place"
	poseDepositDrop      = "deposit-drop"
	poseDepositExit      = "deposit-exit"
	poseLeverHover       = "lever-hover"
	poseLeverEngage      = "lever-engage"
	poseLeverPulled      = "lever-pulled"
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
		return fmt.Errorf("linear-into shaker cradle: %w", err)
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

	if _, err := b.carryHeldLevel(ctx, sw, poseDepositApproach); err != nil {
		return fmt.Errorf("carry to deposit-approach: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseDepositPlace); err != nil {
		return fmt.Errorf("linear-place shaker at deposit: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseDepositDrop); err != nil {
		return fmt.Errorf("linear-descend past deposit engagement: %w", err)
	}
	b.detachHeld()
	if _, err := b.linearMoveToPose(ctx, sw, poseDepositExit); err != nil {
		return fmt.Errorf("linear-exit from deposit: %w", err)
	}

	if _, err := b.linearMoveToPose(ctx, sw, poseLeverHover); err != nil {
		return fmt.Errorf("linear-to lever hover: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseLeverEngage); err != nil {
		return fmt.Errorf("linear-engage lever: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseLeverPulled); err != nil {
		return fmt.Errorf("linear-pull lever: %w", err)
	}
	if err := sleepCtx(ctx, time.Duration(dwellMs)*time.Millisecond); err != nil {
		return fmt.Errorf("ice dwell: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseLeverEngage); err != nil {
		return fmt.Errorf("linear-release lever: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseLeverHover); err != nil {
		return fmt.Errorf("linear-retreat from lever: %w", err)
	}

	if _, err := b.linearMoveToPose(ctx, sw, poseDepositExit); err != nil {
		return fmt.Errorf("linear-back to deposit-exit: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseDepositDrop); err != nil {
		return fmt.Errorf("linear-into deposit drop position: %w", err)
	}
	if err := b.attachHeld(b.cfg.HeldShakerGeometry); err != nil {
		return fmt.Errorf("attach held shaker with ice: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseDepositPlace); err != nil {
		return fmt.Errorf("linear-lift shaker through place: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseDepositApproach); err != nil {
		return fmt.Errorf("linear-lift to deposit-approach: %w", err)
	}

	if _, err := b.carryHeldLevel(ctx, sw, poseShakerCarryHover); err != nil {
		return fmt.Errorf("carry back to shaker carry-hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseShakerLift); err != nil {
		return fmt.Errorf("linear-down to shaker lift: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseShakerDrop); err != nil {
		return fmt.Errorf("linear-drop shaker into cradle: %w", err)
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
