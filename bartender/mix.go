package bartender

import (
	"context"
	"fmt"
	"time"
)

const (
	poseMixerHover      = "mixer-hover"
	poseMixerTiltHover  = "mixer-tilt-hover"
	poseMixerTiltInsert = "mixer-tilt-insert"
	poseMixerInsert     = "mixer-insert"
	poseMixerLift       = "mixer-lift"
	poseMixerLower      = "mixer-lower"
)

func (b *bartender) mix(ctx context.Context, stationSwitchName string, dwellMs int) error {
	sw, err := b.findSwitch(stationSwitchName)
	if err != nil {
		return err
	}
	shakerAllow := b.pickupAllowedCollisions("shaker")

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

	if _, err := b.carryHeldLevel(ctx, sw, poseMixerHover); err != nil {
		return fmt.Errorf("carry to mixer-hover: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseMixerTiltHover); err != nil {
		return fmt.Errorf("tilt at mixer-hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseMixerTiltInsert); err != nil {
		return fmt.Errorf("linear-insert tilted into mixer: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseMixerInsert); err != nil {
		return fmt.Errorf("untilt inside mixer: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseMixerLift); err != nil {
		return fmt.Errorf("linear-lift inside mixer: %w", err)
	}

	deadline := time.Now().Add(time.Duration(dwellMs) * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, err := b.linearCarryToPose(ctx, sw, poseMixerLower); err != nil {
			return fmt.Errorf("linear-bob down to mixer-lower: %w", err)
		}
		if _, err := b.linearCarryToPose(ctx, sw, poseMixerLift); err != nil {
			return fmt.Errorf("linear-bob up to mixer-lift: %w", err)
		}
	}

	if _, err := b.linearCarryToPose(ctx, sw, poseMixerInsert); err != nil {
		return fmt.Errorf("linear-descend from mixer-lift: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseMixerTiltInsert); err != nil {
		return fmt.Errorf("tilt inside mixer: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseMixerTiltHover); err != nil {
		return fmt.Errorf("linear-retreat tilted from mixer: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseMixerHover); err != nil {
		return fmt.Errorf("untilt at mixer-hover: %w", err)
	}
	return nil
}
