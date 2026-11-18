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
	if err := b.serveFromShaker(ctx, sw, pourMs); err != nil {
		return err
	}
	return b.returnShakerHome(ctx, sw)
}

// serveFromShaker pours the held shaker into the glass at sw and leaves it held at serve-approach.
func (b *bartender) serveFromShaker(ctx context.Context, sw toggleswitch.Switch, pourMs int) error {
	approach, err := fetchPose(ctx, sw, poseServeApproach)
	if err != nil {
		return err
	}
	tilt, err := fetchPose(ctx, sw, poseServeTilt)
	if err != nil {
		return err
	}
	return b.serveFromShakerAt(ctx, sw, pourMs, approach, tilt)
}

// pourFromShakerAt runs the pour_from_shaker sequence with the given serve-approach and serve-tilt poses,
// so callers can move the serve point (e.g. to a glass found by vision).
func (b *bartender) pourFromShakerAt(ctx context.Context, sw toggleswitch.Switch, pourMs int, approach, tilt *poseData) error {
	if err := b.serveFromShakerAt(ctx, sw, pourMs, approach, tilt); err != nil {
		return err
	}
	return b.returnShakerHome(ctx, sw)
}

func (b *bartender) serveFromShakerAt(ctx context.Context, sw toggleswitch.Switch, pourMs int, approach, tilt *poseData) error {
	if b.heldGeomFrame == nil {
		return errors.New("pour_from_shaker: shaker must already be in the gripper (call strain_shaker first in a chain)")
	}
	station := sw.Name().ShortName()

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
	return nil
}
