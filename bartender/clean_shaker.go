package bartender

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	poseCleanerHover     = "cleaner-hover"
	poseCleanerFlipHover = "cleaner-flip-hover"
	poseCleanerApproach  = "cleaner-approach"
	poseCleanerPress     = "cleaner-press"
)

// cleanShaker rinses the held shaker on the cleaner, which sprays while the
// upside-down shaker is pressed onto it, then returns the shaker home using
// the shaker-home-* poses on the same switch.
func (b *bartender) cleanShaker(ctx context.Context, stationSwitchName string, dwellMs, drainMs int) error {
	if b.heldGeomFrame == nil {
		return errors.New("clean_shaker: shaker must already be in the gripper")
	}
	sw, err := b.findSwitch(stationSwitchName)
	if err != nil {
		return err
	}
	cleanerAllow := b.cleanerAllowed()

	if _, err := b.carryHeldLevel(ctx, sw, poseCleanerHover, cleanerAllow...); err != nil {
		return fmt.Errorf("carry to cleaner-hover: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseCleanerFlipHover, cleanerAllow...); err != nil {
		return fmt.Errorf("flip shaker upside down at cleaner-hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseCleanerApproach, cleanerAllow...); err != nil {
		return fmt.Errorf("linear-lower to cleaner-approach: %w", err)
	}
	stopCleaner, err := b.startCleaner(ctx)
	if err != nil {
		return err
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseCleanerPress, cleanerAllow...); err != nil {
		_ = stopCleaner()
		return fmt.Errorf("linear-press onto cleaner: %w", err)
	}
	if err := sleepCtx(ctx, time.Duration(dwellMs)*time.Millisecond); err != nil {
		_ = stopCleaner()
		return fmt.Errorf("rinse dwell: %w", err)
	}
	if err := stopCleaner(); err != nil {
		return err
	}
	if err := sleepCtx(ctx, time.Duration(drainMs)*time.Millisecond); err != nil {
		return fmt.Errorf("drain dwell: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseCleanerApproach, cleanerAllow...); err != nil {
		return fmt.Errorf("linear-release from cleaner: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseCleanerFlipHover, cleanerAllow...); err != nil {
		return fmt.Errorf("linear-retreat to cleaner-flip-hover: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseCleanerHover, cleanerAllow...); err != nil {
		return fmt.Errorf("flip shaker upright at cleaner-hover: %w", err)
	}
	return b.returnShakerHome(ctx, sw)
}
