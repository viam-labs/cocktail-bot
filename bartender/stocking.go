package bartender

import (
	"context"
	"fmt"

	toggleswitch "go.viam.com/rdk/components/switch"
)

func (b *bartender) stockBottle(ctx context.Context, slotName string) error {
	if err := b.refuseFridgeSlots("stock_bottle", slotName); err != nil {
		return err
	}
	slotSw, loadingSw, err := b.stockingSwitches(slotName)
	if err != nil {
		return err
	}
	parkedBlocker, slotExtra, err := b.prepareBlockerFor(ctx, slotSw)
	if err != nil {
		return err
	}
	if err := b.pickupBottleAt(ctx, loadingSw, nil); err != nil {
		return fmt.Errorf("pickup from loading spot: %w", err)
	}
	if err := b.returnBottleAt(ctx, slotSw, slotExtra); err != nil {
		return err
	}
	return b.restoreBlocker(ctx, parkedBlocker)
}

func (b *bartender) retrieveBottle(ctx context.Context, slotName string) error {
	if err := b.refuseFridgeSlots("retrieve_bottle", slotName); err != nil {
		return err
	}
	slotSw, loadingSw, err := b.stockingSwitches(slotName)
	if err != nil {
		return err
	}
	parkedBlocker, slotExtra, err := b.prepareBlockerFor(ctx, slotSw)
	if err != nil {
		return err
	}
	if err := b.pickupBottleAt(ctx, slotSw, slotExtra); err != nil {
		return err
	}
	if err := b.returnBottleAt(ctx, loadingSw, nil); err != nil {
		return fmt.Errorf("return to loading spot: %w", err)
	}
	return b.restoreBlocker(ctx, parkedBlocker)
}

func (b *bartender) moveBottle(ctx context.Context, fromName, toName string) error {
	if err := b.refuseFridgeSlots("move_bottle", fromName, toName); err != nil {
		return err
	}
	fromSw, err := b.findSwitch(fromName)
	if err != nil {
		return fmt.Errorf("from switch %q: %w", fromName, err)
	}
	toSw, err := b.findSwitch(toName)
	if err != nil {
		return fmt.Errorf("to switch %q: %w", toName, err)
	}
	fromBlocker, err := b.blockerForTarget(ctx, fromSw)
	if err != nil {
		return err
	}
	toBlocker, err := b.blockerForTarget(ctx, toSw)
	if err != nil {
		return err
	}
	parkedBlocker := ""
	if fromBlocker != "" && b.slotOccupied(fromBlocker) {
		parkedBlocker = fromBlocker
	}
	if toBlocker != "" && b.slotOccupied(toBlocker) {
		if parkedBlocker != "" && parkedBlocker != toBlocker {
			return fmt.Errorf("move_bottle: both %q and %q are blocked by occupied slots (%q, %q); clear one manually first", fromName, toName, fromBlocker, toBlocker)
		}
		parkedBlocker = toBlocker
	}
	fromExtra := b.extraAllowedForBlocker(fromBlocker)
	toExtra := b.extraAllowedForBlocker(toBlocker)
	if parkedBlocker != "" {
		if err := b.shuffleBlockerToParking(ctx, parkedBlocker); err != nil {
			return fmt.Errorf("shuffle blocker aside: %w", err)
		}
	}
	if err := b.pickupBottleAt(ctx, fromSw, fromExtra); err != nil {
		return err
	}
	if err := b.returnBottleAt(ctx, toSw, toExtra); err != nil {
		return err
	}
	return b.restoreBlocker(ctx, parkedBlocker)
}

func (b *bartender) stockingSwitches(slotName string) (toggleswitch.Switch, toggleswitch.Switch, error) {
	if b.cfg.LoadingSpotSwitchName == "" {
		return nil, nil, fmt.Errorf("loading_spot_switch_name is unset")
	}
	slotSw, err := b.findSwitch(slotName)
	if err != nil {
		return nil, nil, fmt.Errorf("slot switch %q: %w", slotName, err)
	}
	loadingSw, err := b.findSwitch(b.cfg.LoadingSpotSwitchName)
	if err != nil {
		return nil, nil, fmt.Errorf("loading-spot switch %q: %w", b.cfg.LoadingSpotSwitchName, err)
	}
	return slotSw, loadingSw, nil
}

// prepareBlockerFor parks the blocker of the given slot (if one is occupied)
// and returns the parked-blocker name for a later restore, plus allowances for
// moves that pass near the blocker's obstacle volume.
func (b *bartender) prepareBlockerFor(ctx context.Context, slotSw toggleswitch.Switch) (string, []AllowedCollision, error) {
	blockerName, err := b.blockerForTarget(ctx, slotSw)
	if err != nil {
		return "", nil, err
	}
	extraAllowed := b.extraAllowedForBlocker(blockerName)
	if blockerName == "" || !b.slotOccupied(blockerName) {
		return "", extraAllowed, nil
	}
	if err := b.shuffleBlockerToParking(ctx, blockerName); err != nil {
		return "", nil, fmt.Errorf("shuffle blocker aside: %w", err)
	}
	return blockerName, extraAllowed, nil
}

func (b *bartender) restoreBlocker(ctx context.Context, blockerName string) error {
	if blockerName == "" || b.cfg.ParkingSwitchName == "" {
		return nil
	}
	parkingSw, err := b.findSwitch(b.cfg.ParkingSwitchName)
	if err != nil {
		return nil
	}
	if err := b.shuffleBlockerFromParking(ctx, parkingSw, blockerName); err != nil {
		return fmt.Errorf("retrieve blocker from parking: %w", err)
	}
	return nil
}

func (b *bartender) extraAllowedForBlocker(blockerName string) []AllowedCollision {
	if blockerName == "" {
		return nil
	}
	obs, ok := b.cfg.PickupObstacles[blockerName]
	if !ok || obs == "" {
		return nil
	}
	return b.gripperAllowedVsObstacle(obs)
}

// refuseFridgeSlots keeps stocking out of the fridge until it opens and closes the door around its moves.
func (b *bartender) refuseFridgeSlots(cmd string, slots ...string) error {
	for _, slot := range slots {
		if b.cfg.isFridgeSlot(slot) {
			return fmt.Errorf("%s: %s is in the fridge, which the arm can't stock yet; move the bottle by hand and choose I moved it manually", cmd, slot)
		}
	}
	return nil
}
