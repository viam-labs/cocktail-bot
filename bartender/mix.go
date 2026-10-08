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
)

func (b *bartender) mix(ctx context.Context, stationSwitchName string, dwellMs int) error {
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
	if err := b.attachHeld(ctx, b.cfg.HeldShakerGeometry); err != nil {
		return fmt.Errorf("attach held shaker: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseShakerLift); err != nil {
		return fmt.Errorf("linear-lift shaker: %w", err)
	}
	if _, err := b.carryHeldLevel(ctx, sw, poseShakerCarryHover); err != nil {
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

	if err := sleepCtx(ctx, time.Duration(dwellMs)*time.Millisecond); err != nil {
		return fmt.Errorf("mixer dwell: %w", err)
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
