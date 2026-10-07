package bartender

import (
	"context"
	"fmt"
	"time"
)

const posePull = "pull"

func (b *bartender) dispenseIce(ctx context.Context, leverSwitchName string, dwellMs int) error {
	sw, err := b.findSwitch(leverSwitchName)
	if err != nil {
		return err
	}
	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("start-home: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseHover); err != nil {
		return fmt.Errorf("hover %s: %w", leverSwitchName, err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseGrab); err != nil {
		return fmt.Errorf("engage lever %s: %w", leverSwitchName, err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, posePull); err != nil {
		return fmt.Errorf("pull lever %s: %w", leverSwitchName, err)
	}
	if err := sleepCtx(ctx, time.Duration(dwellMs)*time.Millisecond); err != nil {
		return fmt.Errorf("ice dwell: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseGrab); err != nil {
		return fmt.Errorf("release lever %s: %w", leverSwitchName, err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseHover); err != nil {
		return fmt.Errorf("retreat %s: %w", leverSwitchName, err)
	}
	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("end-home: %w", err)
	}
	return nil
}
