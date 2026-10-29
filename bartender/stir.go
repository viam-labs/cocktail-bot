package bartender

import (
	"context"
	"fmt"
	"time"
)

const (
	poseStirHover  = "stir-hover"
	poseStirInsert = "stir-insert"
)

// stir expects the spoon already in the gripper and leaves it there.
func (b *bartender) stir(ctx context.Context, stationSwitchName string, dwellMs int) error {
	sw, err := b.findSwitch(stationSwitchName)
	if err != nil {
		return err
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseStirHover); err != nil {
		return fmt.Errorf("stir-hover: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseStirInsert); err != nil {
		return fmt.Errorf("linear-insert spoon to stir-insert: %w", err)
	}
	dwell := time.Duration(dwellMs) * time.Millisecond
	if err := b.circleAroundPose(ctx, sw, poseStirInsert, dwell); err != nil {
		return fmt.Errorf("stir around stir-insert: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseStirHover); err != nil {
		return fmt.Errorf("linear-retract spoon to stir-hover: %w", err)
	}
	return nil
}
