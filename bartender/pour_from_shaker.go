package bartender

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	toggleswitch "go.viam.com/rdk/components/switch"
)

const (
	poseServeApproach = "serve-approach"
	poseServePreTilt  = "serve-pre-tilt"
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
// glassAllow lets the held shaker into the glass's obstacle over the glass.
func (b *bartender) serveFromShaker(ctx context.Context, sw toggleswitch.Switch, pourMs int, glassAllow ...AllowedCollision) error {
	approach, err := fetchPose(ctx, sw, poseServeApproach)
	if err != nil {
		return err
	}
	preTilt, err := fetchOptionalPose(ctx, sw, poseServePreTilt)
	if err != nil {
		return err
	}
	tilt, err := fetchPose(ctx, sw, poseServeTilt)
	if err != nil {
		return err
	}
	return b.serveFromShakerAt(ctx, sw, pourMs, approach, preTilt, tilt, glassAllow...)
}

// fetchOptionalPose returns nil, nil when sw has no pose named poseName.
func fetchOptionalPose(ctx context.Context, sw toggleswitch.Switch, poseName string) (*poseData, error) {
	_, names, err := sw.GetNumberOfPositions(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list poses on %q: %w", sw.Name().ShortName(), err)
	}
	if !slices.Contains(names, poseName) {
		return nil, nil
	}
	return fetchPose(ctx, sw, poseName)
}

// pourFromShakerAt runs the pour_from_shaker sequence with the given serve poses (preTilt may be nil),
// so callers can move the serve point (e.g. to a glass found by vision).
func (b *bartender) pourFromShakerAt(ctx context.Context, sw toggleswitch.Switch, pourMs int, approach, preTilt, tilt *poseData) error {
	if err := b.serveFromShakerAt(ctx, sw, pourMs, approach, preTilt, tilt); err != nil {
		return err
	}
	return b.returnShakerHome(ctx, sw)
}

// serveFromShakerAt tilts from approach to tilt at the serve speed. A non-nil preTilt is reached first at its
// own pose speed, so only the part of the tilt that pours runs slow.
func (b *bartender) serveFromShakerAt(ctx context.Context, sw toggleswitch.Switch, pourMs int, approach, preTilt, tilt *poseData, glassAllow ...AllowedCollision) error {
	if b.heldGeomFrame == nil {
		return errors.New("pour_from_shaker: shaker must already be in the gripper (call strain_shaker first in a chain)")
	}
	station := sw.Name().ShortName()

	if _, err := b.carryHeldLevelToResolved(ctx, approach, station+":"+poseServeApproach+":carry", glassAllow...); err != nil {
		return fmt.Errorf("carry to serve-approach: %w", err)
	}
	overGlass := b.withAllowedCollisions(nil, glassAllow)
	if preTilt != nil {
		if _, err := b.moveToResolvedPose(ctx, preTilt, station+":"+poseServePreTilt, overGlass, nil); err != nil {
			return fmt.Errorf("pre-tilt shaker over glass: %w", err)
		}
	}
	serveOpts := b.serveMoveOptions()
	if _, err := b.moveToResolvedPose(ctx, tilt, station+":"+poseServeTilt, overGlass, serveOpts); err != nil {
		return fmt.Errorf("tilt shaker over glass: %w", err)
	}
	if err := sleepCtx(ctx, time.Duration(pourMs)*time.Millisecond); err != nil {
		return fmt.Errorf("pour dwell: %w", err)
	}
	if _, err := b.moveToResolvedPose(ctx, approach, station+":"+poseServeApproach, overGlass, serveOpts); err != nil {
		return fmt.Errorf("untilt shaker over glass: %w", err)
	}
	return nil
}
