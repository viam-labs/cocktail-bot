package bartender

import (
	"context"
	"fmt"
	"time"

	toggleswitch "go.viam.com/rdk/components/switch"
)

const (
	poseHome         = "home"
	poseHover        = "hover"
	poseCarryHover   = "carry-hover"
	poseLift         = "lift"
	poseGrab         = "grab"
	posePourApproach = "pour-approach"
	posePourTilt     = "pour-tilt"
)

func (b *bartender) pickupPourReturn(ctx context.Context, bottleSwitchName string, pourMs int) error {
	bottleSw, err := b.findSwitch(bottleSwitchName)
	if err != nil {
		return err
	}
	trip, err := b.pickupBottle(ctx, bottleSw)
	if err != nil {
		return err
	}

	if _, err := b.carryHeldToUniversal(ctx, posePourApproach); err != nil {
		return fmt.Errorf("carry to pour-approach: %w", err)
	}
	pourOpts := b.pourMoveOptions()
	if _, err := b.moveArmToPoseWithOpts(ctx, posePourTilt, pourOpts); err != nil {
		return fmt.Errorf("pour-tilt: %w", err)
	}
	if err := sleepCtx(ctx, time.Duration(pourMs)*time.Millisecond); err != nil {
		return fmt.Errorf("pour dwell: %w", err)
	}
	if _, err := b.moveArmToPoseWithOpts(ctx, posePourApproach, pourOpts); err != nil {
		return fmt.Errorf("pour-upright: %w", err)
	}

	return b.returnBottle(ctx, bottleSw, trip)
}

// bottleTrip is what pickupBottle changed to reach a bottle, for returnBottle to undo.
type bottleTrip struct {
	// Blocker moved to parking, or "" when nothing was in the way.
	parkedBlocker string
	openedFridge  bool
}

// Ends holding the bottle at its carry-hover pose.
func (b *bartender) pickupBottle(ctx context.Context, bottleSw toggleswitch.Switch) (bottleTrip, error) {
	if b.cfg.isFridgeSlot(bottleSw.Name().ShortName()) {
		return b.pickupFridgeBottle(ctx, bottleSw)
	}
	parkedBlocker, extraAllowed, err := b.prepareBlockerFor(ctx, bottleSw)
	if err != nil {
		return bottleTrip{}, err
	}
	if err := b.pickupBottleAt(ctx, bottleSw, extraAllowed); err != nil {
		return bottleTrip{}, err
	}
	return bottleTrip{parkedBlocker: parkedBlocker}, nil
}

// pickupFridgeBottle opens the fridge unless it already stands open, then picks the bottle up
// like any other. The fridge has one row, so there is never a blocker to park.
func (b *bartender) pickupFridgeBottle(ctx context.Context, bottleSw toggleswitch.Switch) (bottleTrip, error) {
	name := bottleSw.Name().ShortName()
	blocker, err := b.blockerForTarget(ctx, bottleSw)
	if err != nil {
		return bottleTrip{}, err
	}
	if blocker != "" {
		return bottleTrip{}, fmt.Errorf("fridge slot %q is blocked_by %q, but the fridge has one row and nothing to park", name, blocker)
	}
	var trip bottleTrip
	if b.fridgeDoorOpenDegs() == 0 {
		if err := b.openFridgeDoor(ctx); err != nil {
			return bottleTrip{}, fmt.Errorf("open fridge for %s: %w", name, err)
		}
		trip.openedFridge = true
	}
	if err := b.pickupBottleAt(ctx, bottleSw, nil); err != nil {
		return bottleTrip{}, err
	}
	return trip, nil
}

// bottleSpotAllowed is what moves to and from a bottle spot may touch: the spot's own bottle, the
// bottles protector, and the loading protector, which only the loading spot opens.
func (b *bartender) bottleSpotAllowed(name string, extra []AllowedCollision) []AllowedCollision {
	allowed := append(b.pickupAllowedCollisions(name), b.bottlesProtectorAllowed()...)
	if name == b.cfg.LoadingSpotSwitchName {
		allowed = append(allowed, b.loadingProtectorAllowed()...)
	}
	return append(allowed, extra...)
}

func (b *bartender) pickupBottleAt(ctx context.Context, bottleSw toggleswitch.Switch, extra []AllowedCollision) error {
	name := bottleSw.Name().ShortName()
	allowed := b.bottleSpotAllowed(name, extra)
	if _, err := b.moveArmToPoseOnSwitch(ctx, bottleSw, poseHover, allowed...); err != nil {
		return fmt.Errorf("hover %s: %w", name, err)
	}
	if err := b.gripperOpen(ctx); err != nil {
		return fmt.Errorf("open gripper: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, bottleSw, poseGrab, allowed...); err != nil {
		return fmt.Errorf("linear-grab %s: %w", name, err)
	}
	if _, err := b.gripper.Grab(ctx, nil); err != nil {
		return fmt.Errorf("close gripper on %s: %w", name, err)
	}
	if err := b.attachHeld(ctx, b.cfg.HeldBottleGeometry); err != nil {
		return fmt.Errorf("attach held bottle: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, bottleSw, poseLift, allowed...); err != nil {
		return fmt.Errorf("linear-lift %s: %w", name, err)
	}
	if _, err := b.linearCarryToPose(ctx, bottleSw, poseCarryHover, allowed...); err != nil {
		return fmt.Errorf("carry-hover %s: %w", name, err)
	}
	return nil
}

// shakerGrip names the poses one way of gripping the shaker at shaker-home
// uses. prestage is an optional joint-space pose passed through before the grab.
type shakerGrip struct {
	prestage, hover, approach, lift, carryHover string
}

func (b *bartender) shakerHomeGrip() shakerGrip {
	return shakerGrip{
		prestage:   b.cfg.MixPrestagePose,
		hover:      poseShakerHomeHover,
		approach:   poseShakerHomeApproach,
		lift:       poseShakerHomeLift,
		carryHover: poseShakerHomeCarryHover,
	}
}

// grabShakerAtHome picks up the shaker from shaker-home using the
// shaker-home-* poses on sw and leaves it held at shaker-home-carry-hover.
// mix and strain_shaker share it so both grab the shaker the same way.
func (b *bartender) grabShakerAtHome(ctx context.Context, sw toggleswitch.Switch) error {
	return b.grabShakerAtHomeUsing(ctx, sw, b.shakerHomeGrip(), b.cfg.HeldShakerGeometry)
}

func (b *bartender) grabShakerAtHomeUsing(ctx context.Context, sw toggleswitch.Switch, grip shakerGrip, held *HeldObjectGeometry) error {
	shakerAllow := b.pickupAllowedCollisions("shaker")

	if grip.prestage != "" {
		if _, err := b.moveArmToJointGoalOnSwitch(ctx, sw, grip.prestage); err != nil {
			return fmt.Errorf("prestage before shaker pickup: %w", err)
		}
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, sw, grip.hover); err != nil {
		return fmt.Errorf("shaker-hover: %w", err)
	}
	if err := b.gripperOpen(ctx); err != nil {
		return fmt.Errorf("open gripper before shaker grab: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, grip.approach, shakerAllow...); err != nil {
		return fmt.Errorf("linear-to shaker grab: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, grip.lift, shakerAllow...); err != nil {
		return fmt.Errorf("linear-lift shaker before closing: %w", err)
	}
	if _, err := b.gripper.Grab(ctx, nil); err != nil {
		return fmt.Errorf("close gripper on shaker: %w", err)
	}
	if err := b.attachHeld(ctx, held); err != nil {
		return fmt.Errorf("attach held shaker: %w", err)
	}
	if _, err := b.carryHeldLevel(ctx, sw, grip.carryHover, shakerAllow...); err != nil {
		return fmt.Errorf("carry to shaker carry-hover: %w", err)
	}
	return nil
}

// returnShakerHome is the inverse of grabShakerAtHome: it sets the held shaker
// down at shaker-home using the shaker-home-* poses on sw and backs the empty
// gripper out to shaker-home-hover.
func (b *bartender) returnShakerHome(ctx context.Context, sw toggleswitch.Switch) error {
	return b.returnShakerHomeUsing(ctx, sw, b.shakerHomeGrip())
}

func (b *bartender) returnShakerHomeUsing(ctx context.Context, sw toggleswitch.Switch, grip shakerGrip) error {
	shakerAllow := b.pickupAllowedCollisions("shaker")

	if _, err := b.carryHeldLevel(ctx, sw, grip.carryHover, shakerAllow...); err != nil {
		return fmt.Errorf("carry back to shaker carry-hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sw, grip.lift, shakerAllow...); err != nil {
		return fmt.Errorf("linear-descend to shaker lift: %w", err)
	}
	if err := b.gripperOpen(ctx); err != nil {
		return fmt.Errorf("release shaker grip at lift: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, grip.approach, shakerAllow...); err != nil {
		return fmt.Errorf("linear-descend further to shaker grab: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, grip.hover, shakerAllow...); err != nil {
		return fmt.Errorf("linear-retreat from shaker: %w", err)
	}
	return nil
}

// Parks the arm at the carry-hover for the given shaker source with the shaker
// gripped and the HeldObjectGeometry attached, so an operator can open the
// simulation and visually confirm the capsule position relative to the gripper.
// Does not release.
func (b *bartender) pickupShaker(ctx context.Context, sw toggleswitch.Switch, source string) error {
	shakerAllow := b.pickupAllowedCollisions(source)
	switch source {
	case "shaker-home":
		if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseShakerHomeHover); err != nil {
			return fmt.Errorf("shaker-home-hover: %w", err)
		}
		if err := b.gripperOpen(ctx); err != nil {
			return fmt.Errorf("open gripper before shaker grab: %w", err)
		}
		if _, err := b.linearMoveToPose(ctx, sw, poseShakerHomeApproach, shakerAllow...); err != nil {
			return fmt.Errorf("linear-to shaker-home-approach: %w", err)
		}
		if _, err := b.linearMoveToPose(ctx, sw, poseShakerHomeLift, shakerAllow...); err != nil {
			return fmt.Errorf("linear-lift shaker before closing: %w", err)
		}
		if _, err := b.gripper.Grab(ctx, nil); err != nil {
			return fmt.Errorf("close gripper on shaker-home: %w", err)
		}
		if err := b.attachHeld(ctx, b.cfg.HeldShakerGeometry); err != nil {
			return fmt.Errorf("attach held shaker from shaker-home: %w", err)
		}
		if _, err := b.carryHeldLevel(ctx, sw, poseShakerHomeCarryHover, shakerAllow...); err != nil {
			return fmt.Errorf("carry to shaker-home-carry-hover: %w", err)
		}
	case "shaker-a":
		if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseShakerAHover); err != nil {
			return fmt.Errorf("shaker-a-hover: %w", err)
		}
		if err := b.gripperOpen(ctx); err != nil {
			return fmt.Errorf("open gripper before shaker-a grab: %w", err)
		}
		if _, err := b.linearMoveToPose(ctx, sw, poseShakerAApproach, shakerAllow...); err != nil {
			return fmt.Errorf("linear-to shaker-a-approach: %w", err)
		}
		if _, err := b.linearMoveToPose(ctx, sw, poseShakerALift, shakerAllow...); err != nil {
			return fmt.Errorf("linear-lift at shaker-a: %w", err)
		}
		if _, err := b.gripper.Grab(ctx, nil); err != nil {
			return fmt.Errorf("close gripper on shaker-a: %w", err)
		}
		if err := b.attachHeld(ctx, b.cfg.HeldShakerGeometry); err != nil {
			return fmt.Errorf("attach held shaker from shaker-a: %w", err)
		}
		if _, err := b.carryHeldLevel(ctx, sw, poseShakerACarryHover, shakerAllow...); err != nil {
			return fmt.Errorf("carry to shaker-a-carry-hover: %w", err)
		}
	case "shaker-b":
		if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseShakerBHover); err != nil {
			return fmt.Errorf("shaker-b-hover: %w", err)
		}
		if err := b.gripperOpen(ctx); err != nil {
			return fmt.Errorf("open gripper before shaker-b grab: %w", err)
		}
		if _, err := b.linearMoveToPose(ctx, sw, poseShakerBDescend, shakerAllow...); err != nil {
			return fmt.Errorf("linear-to shaker-b-descend: %w", err)
		}
		if _, err := b.linearMoveToPose(ctx, sw, poseShakerBPlace, shakerAllow...); err != nil {
			return fmt.Errorf("linear-to shaker-b-place: %w", err)
		}
		if _, err := b.gripper.Grab(ctx, nil); err != nil {
			return fmt.Errorf("close gripper on shaker-b: %w", err)
		}
		if err := b.attachHeld(ctx, b.cfg.HeldShakerGeometry); err != nil {
			return fmt.Errorf("attach held shaker from shaker-b: %w", err)
		}
		if _, err := b.linearCarryToPose(ctx, sw, poseShakerBHoverAbove, shakerAllow...); err != nil {
			return fmt.Errorf("linear-lift to shaker-b-hover-above: %w", err)
		}
	default:
		return fmt.Errorf("pickupShaker: unknown source %q (want shaker-home, shaker-a, or shaker-b)", source)
	}
	return nil
}

// Starts from anywhere with the bottle held upright; ends empty-handed at home.
// Undoes only what trip did: a blocker whose slot was empty was never parked,
// and a fridge that already stood open stays open.
func (b *bartender) returnBottle(ctx context.Context, bottleSw toggleswitch.Switch, trip bottleTrip) error {
	blockerName, err := b.blockerForTarget(ctx, bottleSw)
	if err != nil {
		return err
	}
	if err := b.returnBottleAt(ctx, bottleSw, b.extraAllowedForBlocker(blockerName)); err != nil {
		return err
	}
	if err := b.restoreBlocker(ctx, trip.parkedBlocker); err != nil {
		return err
	}
	if trip.openedFridge {
		if err := b.closeFridgeDoor(ctx); err != nil {
			return fmt.Errorf("close fridge after returning %s: %w", bottleSw.Name().ShortName(), err)
		}
	}
	return nil
}

func (b *bartender) returnBottleAt(ctx context.Context, bottleSw toggleswitch.Switch, extra []AllowedCollision) error {
	name := bottleSw.Name().ShortName()
	allowed := b.bottleSpotAllowed(name, extra)
	if _, err := b.carryHeldLevel(ctx, bottleSw, poseCarryHover, allowed...); err != nil {
		return fmt.Errorf("carry back to %s carry-hover: %w", name, err)
	}
	if _, err := b.linearCarryToPose(ctx, bottleSw, poseLift, allowed...); err != nil {
		return fmt.Errorf("linear-over %s: %w", name, err)
	}
	if _, err := b.linearCarryToPose(ctx, bottleSw, poseGrab, allowed...); err != nil {
		return fmt.Errorf("linear-return %s: %w", name, err)
	}
	if err := b.gripperOpen(ctx); err != nil {
		return fmt.Errorf("release gripper: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, bottleSw, poseHover, allowed...); err != nil {
		return fmt.Errorf("linear-retreat %s: %w", name, err)
	}
	return nil
}

// blockerForTarget returns the slot name declared via blocked_by on the
// target's hover pose, or empty when the target has no blocker.
func (b *bartender) blockerForTarget(ctx context.Context, bottleSw toggleswitch.Switch) (string, error) {
	pd, err := fetchPose(ctx, bottleSw, poseHover)
	if err != nil {
		return "", fmt.Errorf("fetch hover for blocker lookup: %w", err)
	}
	return pd.blockedBy, nil
}

func (b *bartender) slotOccupied(slotName string) bool {
	if b.dataStore == nil {
		return false
	}
	_, ok := b.dataStore.SlotAssignments()[slotName]
	return ok
}

func (b *bartender) shuffleBlockerToParking(ctx context.Context, blockerName string) error {
	if err := b.checkParkingClear(); err != nil {
		return err
	}
	if b.cfg.ParkingSwitchName == "" {
		return fmt.Errorf("blocker %q is in the way but parking_switch_name is unset", blockerName)
	}
	if b.slotOccupied(b.cfg.ParkingSwitchName) {
		return fmt.Errorf("parking slot %q has an assigned bottle; clear it first", b.cfg.ParkingSwitchName)
	}
	blockerSw, err := b.findSwitch(blockerName)
	if err != nil {
		return fmt.Errorf("find blocker switch %q: %w", blockerName, err)
	}
	parkingSw, err := b.findSwitch(b.cfg.ParkingSwitchName)
	if err != nil {
		return fmt.Errorf("find parking switch %q: %w", b.cfg.ParkingSwitchName, err)
	}
	if err := b.pickupBottleAt(ctx, blockerSw, nil); err != nil {
		return fmt.Errorf("pickup blocker %s: %w", blockerName, err)
	}
	if err := b.returnBottleAt(ctx, parkingSw, nil); err != nil {
		return fmt.Errorf("park blocker at %s: %w", b.cfg.ParkingSwitchName, err)
	}
	parkingGrab, err := fetchPose(ctx, parkingSw, poseGrab)
	if err != nil {
		return fmt.Errorf("fetch parking grab pose: %w", err)
	}
	if err := b.attachParked(parkingGrab.pose); err != nil {
		return fmt.Errorf("attach parked-bottle geometry: %w", err)
	}
	return nil
}

func (b *bartender) shuffleBlockerFromParking(ctx context.Context, parkingSw toggleswitch.Switch, blockerName string) error {
	blockerSw, err := b.findSwitch(blockerName)
	if err != nil {
		return fmt.Errorf("find blocker switch %q: %w", blockerName, err)
	}
	b.detachParked()
	if err := b.pickupBottleAt(ctx, parkingSw, nil); err != nil {
		return fmt.Errorf("pickup blocker from parking: %w", err)
	}
	if err := b.returnBottleAt(ctx, blockerSw, nil); err != nil {
		return fmt.Errorf("return blocker to %s: %w", blockerName, err)
	}
	return nil
}

func (b *bartender) carryHeldToUniversal(ctx context.Context, poseName string) (time.Duration, error) {
	_, sw, err := b.resolvePose(ctx, poseName)
	if err != nil {
		return 0, err
	}
	return b.carryHeldLevel(ctx, sw, poseName)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
