package bartender

import (
	"context"
	"fmt"
	"math"
	"time"
)

const pourIntoShakerDwell = 500 * time.Millisecond

func (b *bartender) pourIntoShaker(ctx context.Context, bottleSwitchName string, oz float64) error {
	bottleSw, err := b.findSwitch(bottleSwitchName)
	if err != nil {
		return err
	}
	pourerOz, ok := b.cfg.BottlePourerOz[bottleSwitchName]
	if !ok {
		return fmt.Errorf("no bottle_pourer_oz configured for bottle %q", bottleSwitchName)
	}
	pours, err := pourCount(oz, pourerOz)
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
		if err := sleepCtx(ctx, pourIntoShakerDwell); err != nil {
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
