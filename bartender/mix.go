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
	mixerAllow := b.mixerProtectorAllowed()

	if err := b.grabShakerAtHome(ctx, sw); err != nil {
		return err
	}

	if _, err := b.carryHeldLevel(ctx, sw, poseMixerHover, mixerAllow...); err != nil {
		return fmt.Errorf("carry to mixer-hover: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseMixerTiltHover, mixerAllow...); err != nil {
		return fmt.Errorf("tilt at mixer-hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseMixerTiltInsert, mixerAllow...); err != nil {
		return fmt.Errorf("linear-insert tilted into mixer: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseMixerInsert, mixerAllow...); err != nil {
		return fmt.Errorf("untilt inside mixer: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseMixerLift, mixerAllow...); err != nil {
		return fmt.Errorf("linear-lift inside mixer: %w", err)
	}

	stopMixer := b.startMixer(ctx)
	defer stopMixer()
	deadline := time.Now().Add(time.Duration(dwellMs) * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, err := b.linearCarryToPose(ctx, sw, poseMixerLower, mixerAllow...); err != nil {
			return fmt.Errorf("linear-bob down to mixer-lower: %w", err)
		}
		if _, err := b.linearCarryToPose(ctx, sw, poseMixerLift, mixerAllow...); err != nil {
			return fmt.Errorf("linear-bob up to mixer-lift: %w", err)
		}
	}
	stopMixer()

	if _, err := b.linearCarryToPose(ctx, sw, poseMixerInsert, mixerAllow...); err != nil {
		return fmt.Errorf("linear-descend from mixer-lift: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseMixerTiltInsert, mixerAllow...); err != nil {
		return fmt.Errorf("tilt inside mixer: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, poseMixerTiltHover, mixerAllow...); err != nil {
		return fmt.Errorf("linear-retreat tilted from mixer: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseMixerHover, mixerAllow...); err != nil {
		return fmt.Errorf("untilt at mixer-hover: %w", err)
	}
	return nil
}
