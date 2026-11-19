package bartender

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	poseCleanerGrabHover      = "cleaner-grab-hover"
	poseCleanerGrabApproach   = "cleaner-grab-approach"
	poseCleanerGrabLift       = "cleaner-grab-lift"
	poseCleanerGrabCarryHover = "cleaner-grab-carry-hover"
	poseCleanerHover          = "cleaner-hover"
	poseCleanerFlipHover      = "cleaner-flip-hover"
	poseCleanerApproach       = "cleaner-approach"
	poseCleanerPress          = "cleaner-press"
)

type cleanShakerRequest struct {
	station        string
	powerOnDelayMs int
	dwellMs        int
	drainMs        int
}

// cleanShaker picks the shaker up from shaker-home with the cleaner-grab-*
// poses, rinses it on the cleaner, which sprays while the upside-down shaker
// is pressed onto it, and sets it back down at shaker-home with the same grip.
// The grip is lower than the other flows use so the shaker can be flipped.
func (b *bartender) cleanShaker(ctx context.Context, req cleanShakerRequest) error {
	if b.heldGeomFrame != nil {
		return errors.New("clean_shaker: gripper must be empty with the shaker at shaker-home")
	}
	sw, err := b.findSwitch(req.station)
	if err != nil {
		return err
	}
	cleanerAllow := b.cleanerAllowed()
	grip := b.cleanerGrip()

	if err := b.grabShakerAtHomeUsing(ctx, sw, grip, b.cleanerHeldShakerGeometry()); err != nil {
		return err
	}
	if _, err := b.carryHeldLevel(ctx, sw, poseCleanerHover, cleanerAllow...); err != nil {
		return fmt.Errorf("carry to cleaner-hover: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseCleanerFlipHover, cleanerAllow...); err != nil {
		return fmt.Errorf("flip shaker upside down at cleaner-hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseCleanerApproach, cleanerAllow...); err != nil {
		return fmt.Errorf("linear-lower to cleaner-approach: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseCleanerPress, cleanerAllow...); err != nil {
		return fmt.Errorf("linear-press onto cleaner: %w", err)
	}
	if err := b.rinse(ctx, req); err != nil {
		return err
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
	return b.returnShakerHomeUsing(ctx, sw, grip)
}

func (b *bartender) rinse(ctx context.Context, req cleanShakerRequest) error {
	if err := sleepCtx(ctx, time.Duration(req.powerOnDelayMs)*time.Millisecond); err != nil {
		return fmt.Errorf("power-on delay: %w", err)
	}
	stopCleaner, err := b.startCleaner(ctx)
	if err != nil {
		return err
	}
	if err := sleepCtx(ctx, time.Duration(req.dwellMs)*time.Millisecond); err != nil {
		_ = stopCleaner()
		return fmt.Errorf("rinse dwell: %w", err)
	}
	if err := stopCleaner(); err != nil {
		return err
	}
	if err := sleepCtx(ctx, time.Duration(req.drainMs)*time.Millisecond); err != nil {
		return fmt.Errorf("drain dwell: %w", err)
	}
	return nil
}

func (b *bartender) cleanerGrip() shakerGrip {
	grip := shakerGrip{
		hover:      poseCleanerGrabHover,
		approach:   poseCleanerGrabApproach,
		lift:       poseCleanerGrabLift,
		carryHover: poseCleanerGrabCarryHover,
	}
	if sc := b.cfg.ShakerCleaning; sc != nil {
		grip.prestage = sc.PrestagePose
	}
	return grip
}

func (b *bartender) cleanerHeldShakerGeometry() *HeldObjectGeometry {
	if sc := b.cfg.ShakerCleaning; sc != nil && sc.HeldShakerGeometry != nil {
		return sc.HeldShakerGeometry
	}
	return b.cfg.HeldShakerGeometry
}
