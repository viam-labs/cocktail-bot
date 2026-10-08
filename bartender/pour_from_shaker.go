package bartender

import (
	"context"
	"fmt"
	"time"
)

const (
	poseServeApproach = "serve-approach"
	poseServeTilt     = "serve-tilt"
)

func (b *bartender) pourFromShaker(ctx context.Context, stationSwitchName string, pourMs int) error {
	sw, err := b.findSwitch(stationSwitchName)
	if err != nil {
		return err
	}
	shakerAllow := b.pickupAllowedCollisions("shaker")

	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("start-home: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseShakerHomeHover); err != nil {
		return fmt.Errorf("shaker-hover: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("open gripper before shaker grab: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseShakerHomeApproach, shakerAllow...); err != nil {
		return fmt.Errorf("linear-to shaker grab: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseShakerHomeLift, shakerAllow...); err != nil {
		return fmt.Errorf("linear-lift shaker before closing: %w", err)
	}
	if _, err := b.gripper.Grab(ctx, nil); err != nil {
		return fmt.Errorf("close gripper on shaker: %w", err)
	}
	if err := b.attachHeld(ctx, b.cfg.HeldShakerGeometry); err != nil {
		return fmt.Errorf("attach held shaker: %w", err)
	}
	if _, err := b.carryHeldLevel(ctx, sw, poseShakerHomeCarryHover, shakerAllow...); err != nil {
		return fmt.Errorf("carry to shaker carry-hover: %w", err)
	}

	if _, err := b.carryHeldLevel(ctx, sw, poseServeApproach); err != nil {
		return fmt.Errorf("carry to serve-approach: %w", err)
	}
	serveOpts := b.serveMoveOptions()
	if _, err := b.moveArmToPoseOnSwitchWithOpts(ctx, sw, poseServeTilt, serveOpts); err != nil {
		return fmt.Errorf("tilt shaker over glass: %w", err)
	}
	if err := sleepCtx(ctx, time.Duration(pourMs)*time.Millisecond); err != nil {
		return fmt.Errorf("pour dwell: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitchWithOpts(ctx, sw, poseServeApproach, serveOpts); err != nil {
		return fmt.Errorf("untilt shaker over glass: %w", err)
	}

	if _, err := b.carryHeldLevel(ctx, sw, poseShakerHomeCarryHover, shakerAllow...); err != nil {
		return fmt.Errorf("carry back to shaker carry-hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseShakerHomeLift, shakerAllow...); err != nil {
		return fmt.Errorf("linear-descend to shaker lift: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("release shaker grip at lift: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseShakerHomeApproach, shakerAllow...); err != nil {
		return fmt.Errorf("linear-descend further to shaker grab: %w", err)
	}
	b.detachHeld()
	if _, err := b.linearMoveToPose(ctx, sw, poseShakerHomeHover, shakerAllow...); err != nil {
		return fmt.Errorf("linear-retreat from shaker: %w", err)
	}
	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("end-home: %w", err)
	}
	return nil
}
