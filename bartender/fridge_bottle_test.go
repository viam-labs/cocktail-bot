package bartender

import (
	"context"
	"testing"

	toggleswitch "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
)

// fridgeBottleBartender adds fridge slot "slot-a-fridge-front" to fridgeTestBartender. Every bottle
// pose is open-retreat, where opening the door leaves the arm, so the bottle moves are trivial.
func fridgeBottleBartender(t *testing.T, blockedBy string) (*bartender, toggleswitch.Switch, map[string][]any) {
	t.Helper()
	b, _ := fridgeTestBartender(t)
	retreat, err := b.fridgeGrabInWorld(context.Background(), b.poseSwitches[0], poseFridgeOpenRetreat)
	test.That(t, err, test.ShouldBeNil)
	asked := map[string][]any{}
	slot := inject.NewSwitch("slot-a-fridge-front")
	slot.DoFunc = func(_ context.Context, cmd map[string]any) (map[string]any, error) {
		asked["slot-a-fridge-front"] = append(asked["slot-a-fridge-front"], cmd["get_pose_by_name"])
		resp := poseResponse(retreat.pose)
		if cmd["get_pose_by_name"] == poseHover && blockedBy != "" {
			resp["blocked_by"] = blockedBy
		}
		return resp, nil
	}
	b.poseSwitches = append(b.poseSwitches, slot)
	b.cfg.SlotNames = []string{"slot-a-fridge-front"}
	b.cfg.Fridge.Slots = []string{"slot-a-fridge-front"}
	return b, slot, asked
}

func TestFridgeBottleOpensAndClosesTheDoor(t *testing.T) {
	b, slot, _ := fridgeBottleBartender(t, "")
	ctx := context.Background()

	trip, err := b.pickupBottle(ctx, slot)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, trip, test.ShouldResemble, bottleTrip{openedFridge: true})
	test.That(t, b.fridgeDoorOpenDegs(), test.ShouldAlmostEqual, 30, 1e-6)

	test.That(t, b.returnBottle(ctx, slot, trip), test.ShouldBeNil)
	test.That(t, b.fridgeDoorOpenDegs(), test.ShouldEqual, 0)
}

func TestFridgeBottleLeavesAnOpenDoorOpen(t *testing.T) {
	b, slot, _ := fridgeBottleBartender(t, "")
	ctx := context.Background()
	test.That(t, b.openFridgeDoor(ctx), test.ShouldBeNil)

	trip, err := b.pickupBottle(ctx, slot)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, trip, test.ShouldResemble, bottleTrip{})
	test.That(t, b.returnBottle(ctx, slot, trip), test.ShouldBeNil)
	test.That(t, b.fridgeDoorOpenDegs(), test.ShouldAlmostEqual, 30, 1e-6)
}

func TestFridgeBottleRefusesABlocker(t *testing.T) {
	b, slot, asked := fridgeBottleBartender(t, "slot-a-fridge-back")
	_, err := b.pickupBottle(context.Background(), slot)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "one row")
	test.That(t, asked["slot-a-fridge-front"], test.ShouldResemble, []any{poseHover})
	test.That(t, b.fridgeDoorOpenDegs(), test.ShouldEqual, 0)
}

func TestStockingRefusesFridgeSlots(t *testing.T) {
	b := &bartender{cfg: &Config{Fridge: &Fridge{Slots: []string{"slot-a-fridge-front"}}}}
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"stock", func() error { return b.stockBottle(ctx, "slot-a-fridge-front") }},
		{"retrieve", func() error { return b.retrieveBottle(ctx, "slot-a-fridge-front") }},
		{"move out", func() error { return b.moveBottle(ctx, "slot-a-fridge-front", "slot-a-bottom-front") }},
		{"move in", func() error { return b.moveBottle(ctx, "slot-a-bottom-front", "slot-a-fridge-front") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "slot-a-fridge-front is in the fridge")
		})
	}
}

func TestConfigValidateFridgeSlots(t *testing.T) {
	cfg := &Config{
		ArmName:           "arm",
		GripperName:       "gripper",
		PoseSwitcherNames: []string{"fridge-door-handle", "slot-a-fridge-front"},
		SlotNames:         []string{"slot-a-fridge-front"},
		Fridge:            testFridge(),
	}
	cfg.Fridge.Slots = []string{"slot-a-fridge-front"}
	_, _, err := cfg.Validate("svc")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cfg.isFridgeSlot("slot-a-fridge-front"), test.ShouldBeTrue)
	test.That(t, cfg.isFridgeSlot("slot-a-bottom-front"), test.ShouldBeFalse)

	cfg.Fridge.Slots = []string{"slot-b-fridge-front"}
	_, _, err = cfg.Validate("svc")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `fridge.slots["slot-b-fridge-front"] is not in slot_names`)
}
