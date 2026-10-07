package bartender

import (
	"context"
	"fmt"
	"time"
)

const (
	poseHome         = "home"
	poseHover        = "hover"
	poseGrab         = "grab"
	posePourApproach = "pour-approach"
	posePourTilt     = "pour-tilt"
)

func (b *bartender) pickupPourReturn(ctx context.Context, bottleSwitchName string, pourMs int) error {
	bottleSw, err := b.findSwitch(bottleSwitchName)
	if err != nil {
		return err
	}

	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("start-home: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, bottleSw, poseHover); err != nil {
		return fmt.Errorf("hover %s: %w", bottleSwitchName, err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("open gripper: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, bottleSw, poseGrab); err != nil {
		return fmt.Errorf("linear-grab %s: %w", bottleSwitchName, err)
	}
	if _, err := b.gripper.Grab(ctx, nil); err != nil {
		return fmt.Errorf("close gripper on %s: %w", bottleSwitchName, err)
	}
	if err := b.attachHeldBottle(); err != nil {
		return fmt.Errorf("attach held bottle: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, bottleSw, poseHover); err != nil {
		return fmt.Errorf("linear-lift %s: %w", bottleSwitchName, err)
	}

	if _, err := b.carryHeldToUniversal(ctx, posePourApproach); err != nil {
		return fmt.Errorf("carry to pour-approach: %w", err)
	}
	if _, err := b.moveArmToPose(ctx, posePourTilt); err != nil {
		return fmt.Errorf("pour-tilt: %w", err)
	}
	if err := sleepCtx(ctx, time.Duration(pourMs)*time.Millisecond); err != nil {
		return fmt.Errorf("pour dwell: %w", err)
	}
	if _, err := b.moveArmToPose(ctx, posePourApproach); err != nil {
		return fmt.Errorf("pour-upright: %w", err)
	}

	if _, err := b.carryHeldLevel(ctx, bottleSw, poseHover); err != nil {
		return fmt.Errorf("carry back to %s hover: %w", bottleSwitchName, err)
	}
	if _, err := b.linearCarryToPose(ctx, bottleSw, poseGrab); err != nil {
		return fmt.Errorf("linear-return %s: %w", bottleSwitchName, err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("release gripper: %w", err)
	}
	b.detachHeldBottle()
	if _, err := b.linearMoveToPose(ctx, bottleSw, poseHover); err != nil {
		return fmt.Errorf("linear-retreat %s: %w", bottleSwitchName, err)
	}
	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("end-home: %w", err)
	}
	return nil
}

func (b *bartender) carryHeldToUniversal(ctx context.Context, poseName string) (time.Duration, error) {
	_, sw, err := b.resolvePose(ctx, poseName)
	if err != nil {
		return 0, err
	}
	return b.carryHeldLevel(ctx, sw, poseName)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
