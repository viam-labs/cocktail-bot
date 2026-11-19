package bartender

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	toggleswitch "go.viam.com/rdk/components/switch"
)

const defaultPourIntoShakerDwellMs = 500

type pourerDwell struct {
	oz float64
	ms int
}

func parsePourDwells(raw map[string]int) ([]pourerDwell, error) {
	out := make([]pourerDwell, 0, len(raw))
	for k, ms := range raw {
		oz, err := strconv.ParseFloat(k, 64)
		if err != nil || oz <= 0 {
			return nil, fmt.Errorf("key %q must be a pourer size in oz > 0", k)
		}
		if ms <= 0 {
			return nil, fmt.Errorf("%q: dwell must be > 0 ms, got %d", k, ms)
		}
		for _, d := range out {
			if math.Abs(d.oz-oz) < 1e-6 {
				return nil, fmt.Errorf("%q duplicates pourer size %v", k, oz)
			}
		}
		out = append(out, pourerDwell{oz: oz, ms: ms})
	}
	return out, nil
}

// pourDwell is the tilt dwell for a bottle with this pourer size.
func (b *bartender) pourDwell(pourerOz float64) time.Duration {
	dwells, _ := parsePourDwells(b.cfg.PourDwellMsByPourerOz)
	for _, d := range dwells {
		if math.Abs(d.oz-pourerOz) < 1e-6 {
			return time.Duration(d.ms) * time.Millisecond
		}
	}
	ms := b.cfg.PourIntoShakerDwellMs
	if ms <= 0 {
		ms = defaultPourIntoShakerDwellMs
	}
	return time.Duration(ms) * time.Millisecond
}

func (b *bartender) slotForIngredient(ingredient string) (string, error) {
	if b.dataStore == nil {
		return "", errDataStoreNotConfigured
	}
	for slot, ing := range b.dataStore.SlotAssignments() {
		if ing == ingredient {
			return slot, nil
		}
	}
	return "", fmt.Errorf("ingredient %q is not assigned to any slot", ingredient)
}

type resolvedPour struct {
	bottleSw toggleswitch.Switch
	pours    int
	dwell    time.Duration
}

func (b *bartender) resolvePour(ingredient string, oz float64) (resolvedPour, error) {
	slot, err := b.slotForIngredient(ingredient)
	if err != nil {
		return resolvedPour{}, err
	}
	bottleSw, err := b.findSwitch(slot)
	if err != nil {
		return resolvedPour{}, err
	}
	pourerOz, ok := b.dataStore.PourerOz()[ingredient]
	if !ok {
		return resolvedPour{}, fmt.Errorf("no pourer oz configured for ingredient %q", ingredient)
	}
	pours, err := pourCount(oz, pourerOz)
	if err != nil {
		return resolvedPour{}, err
	}
	return resolvedPour{bottleSw: bottleSw, pours: pours, dwell: b.pourDwell(pourerOz)}, nil
}

func (b *bartender) pourIntoShaker(ctx context.Context, ingredient string, oz float64) error {
	rp, err := b.resolvePour(ingredient, oz)
	if err != nil {
		return err
	}
	bottleSw, pours := rp.bottleSw, rp.pours
	b.logger.CInfof(ctx, "pouring %v oz %s: %d tilt(s), %v dwell each", oz, ingredient, pours, rp.dwell)

	if err := b.pickupBottle(ctx, bottleSw); err != nil {
		return err
	}
	if _, err := b.carryHeldToUniversal(ctx, posePourApproach); err != nil {
		return fmt.Errorf("carry to pour-approach: %w", err)
	}

	pourOpts := b.pourMoveOptions()
	for i := 0; i < pours; i++ {
		if _, err := b.moveArmToPoseWithOpts(ctx, posePourTilt, pourOpts); err != nil {
			return fmt.Errorf("pour-tilt (pour %d/%d): %w", i+1, pours, err)
		}
		if err := sleepCtx(ctx, rp.dwell); err != nil {
			return fmt.Errorf("pour dwell: %w", err)
		}
		if _, err := b.moveArmToPoseWithOpts(ctx, posePourApproach, pourOpts); err != nil {
			return fmt.Errorf("pour-upright (pour %d/%d): %w", i+1, pours, err)
		}
	}

	return b.returnBottle(ctx, bottleSw)
}

func pourCount(oz, pourerOz float64) (int, error) {
	if oz <= 0 {
		return 0, fmt.Errorf("oz must be > 0, got %v", oz)
	}
	if pourerOz <= 0 {
		return 0, fmt.Errorf("pourer_oz must be > 0, got %v", pourerOz)
	}
	pours := int(math.Round(oz / pourerOz))
	if math.Abs(float64(pours)*pourerOz-oz) > 1e-6 {
		return 0, fmt.Errorf("oz %v is not an exact multiple of pourer_oz %v for this bottle", oz, pourerOz)
	}
	if pours < 1 {
		return 0, fmt.Errorf("oz %v with pourer_oz %v resolves to zero pours", oz, pourerOz)
	}
	return pours, nil
}
