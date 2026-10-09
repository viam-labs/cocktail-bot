package bartender

import (
	"context"
	"errors"
	"fmt"
	"time"

	toggleswitch "go.viam.com/rdk/components/switch"
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
	approach, err := fetchPose(ctx, sw, poseServeApproach)
	if err != nil {
		return err
	}
	tilt, err := fetchPose(ctx, sw, poseServeTilt)
	if err != nil {
		return err
	}
	return b.pourFromShakerAt(ctx, sw, pourMs, approach, tilt)
}

// pourFromShakerAt runs the pour_from_shaker sequence with the given serve-approach and serve-tilt poses,
// so callers can move the serve point (e.g. to a glass found by vision).
func (b *bartender) pourFromShakerAt(ctx context.Context, sw toggleswitch.Switch, pourMs int, approach, tilt *poseData) error {
	if b.heldGeomFrame == nil {
		return errors.New("pour_from_shaker: shaker must already be in the gripper (call strain_shaker first in a chain)")
	}
	station := sw.Name().ShortName()
	shakerAllow := b.pickupAllowedCollisions("shaker")

	if _, err := b.carryHeldLevelToResolved(ctx, approach, station+":"+poseServeApproach+":carry"); err != nil {
		return fmt.Errorf("carry to serve-approach: %w", err)
	}
	serveOpts := b.serveMoveOptions()
	if _, err := b.moveToResolvedPose(ctx, tilt, station+":"+poseServeTilt, nil, serveOpts); err != nil {
		return fmt.Errorf("tilt shaker over glass: %w", err)
	}
	if err := sleepCtx(ctx, time.Duration(pourMs)*time.Millisecond); err != nil {
		return fmt.Errorf("pour dwell: %w", err)
	}
	if _, err := b.moveToResolvedPose(ctx, approach, station+":"+poseServeApproach, nil, serveOpts); err != nil {
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
