package bartender

import (
	"context"
	"fmt"
	"math"
	"time"

	toggleswitch "go.viam.com/rdk/components/switch"
)

const defaultPourIntoShakerDwellMs = 500

func (b *bartender) pourIntoShakerDwell() time.Duration {
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

func (b *bartender) resolvePour(ingredient string, oz float64) (toggleswitch.Switch, int, error) {
	slot, err := b.slotForIngredient(ingredient)
	if err != nil {
		return nil, 0, err
	}
	bottleSw, err := b.findSwitch(slot)
	if err != nil {
		return nil, 0, err
	}
	pourerOz, ok := b.dataStore.PourerOz()[ingredient]
	if !ok {
		return nil, 0, fmt.Errorf("no pourer oz configured for ingredient %q", ingredient)
	}
	pours, err := pourCount(oz, pourerOz)
	if err != nil {
		return nil, 0, err
	}
	return bottleSw, pours, nil
}

func (b *bartender) pourIntoShaker(ctx context.Context, ingredient string, oz float64) error {
	bottleSw, pours, err := b.resolvePour(ingredient, oz)
	if err != nil {
		return err
	}

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
		if err := sleepCtx(ctx, b.pourIntoShakerDwell()); err != nil {
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
