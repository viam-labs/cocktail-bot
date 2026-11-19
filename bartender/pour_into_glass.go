package bartender

import (
	"context"
	"fmt"

	toggleswitch "go.viam.com/rdk/components/switch"
)

const (
	posePourGlassApproach = "direct-pour-approach"
	posePourGlassTilt     = "direct-pour-tilt"
	posePourGlassRecover  = "direct-pour-recover"
)

// pourIntoGlass pours straight from the bottle. With a glass it uses that glass's direct-pour poses;
// without one, the first pose switcher that defines them.
func (b *bartender) pourIntoGlass(ctx context.Context, ingredient string, oz float64, glass *Glass) error {
	rp, err := b.resolvePour(ingredient, oz)
	if err != nil {
		return err
	}
	bottleSw, pours := rp.bottleSw, rp.pours
	b.logger.CInfof(ctx, "pouring %v oz %s: %d tilt(s), %v dwell each", oz, ingredient, pours, rp.dwell)

	poses := map[string]*poseData{}
	var glassSw toggleswitch.Switch
	glassSwName, glassAllow := b.glassServe(glass, "")
	if glassSwName != "" {
		if glassSw, err = b.findSwitch(glassSwName); err != nil {
			return fmt.Errorf("glass %q: %w", glass.ID, err)
		}
	}
	for _, name := range []string{posePourGlassApproach, posePourGlassTilt, posePourGlassRecover} {
		var pd *poseData
		if glassSw != nil {
			pd, err = fetchPose(ctx, glassSw, name)
		} else {
			pd, _, err = b.resolvePose(ctx, name)
		}
		if err != nil {
			return fmt.Errorf("direct pour into glass needs %q, %q and %q: %w",
				posePourGlassApproach, posePourGlassTilt, posePourGlassRecover, err)
		}
		poses[name] = pd
	}

	trip, err := b.pickupBottle(ctx, bottleSw)
	if err != nil {
		return err
	}
	if _, err := b.carryHeldLevelToResolved(ctx, poses[posePourGlassApproach], posePourGlassApproach+":carry", glassAllow...); err != nil {
		return fmt.Errorf("carry to %s: %w", posePourGlassApproach, err)
	}

	pourOpts := b.pourMoveOptions()
	overGlass := b.withAllowedCollisions(nil, glassAllow)
	for i := 0; i < pours; i++ {
		if _, err := b.moveToResolvedPose(ctx, poses[posePourGlassTilt], posePourGlassTilt, overGlass, pourOpts); err != nil {
			return fmt.Errorf("%s (pour %d/%d): %w", posePourGlassTilt, i+1, pours, err)
		}
		if err := sleepCtx(ctx, rp.dwell); err != nil {
			return fmt.Errorf("direct pour dwell: %w", err)
		}
		if _, err := b.moveToResolvedPose(ctx, poses[posePourGlassApproach], posePourGlassApproach, overGlass, pourOpts); err != nil {
			return fmt.Errorf("%s (pour %d/%d): %w", posePourGlassApproach, i+1, pours, err)
		}
	}

	if _, err := b.moveToResolvedPose(ctx, poses[posePourGlassRecover], posePourGlassRecover, overGlass, pourOpts); err != nil {
		return fmt.Errorf("%s: %w", posePourGlassRecover, err)
	}

	return b.returnBottle(ctx, bottleSw, trip)
}
