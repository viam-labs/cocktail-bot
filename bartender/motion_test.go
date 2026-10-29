package bartender

import (
	"math"
	"testing"

	"go.viam.com/test"
)

func TestMoveOptionsFromCfgNilWhenZero(t *testing.T) {
	test.That(t, moveOptionsFromCfg(0, 0), test.ShouldBeNil)
}

func TestMoveOptionsFromCfgConvertsDegsToRads(t *testing.T) {
	opts := moveOptionsFromCfg(180, 90)
	test.That(t, opts, test.ShouldNotBeNil)
	test.That(t, opts.MaxVelRads, test.ShouldAlmostEqual, math.Pi, 1e-9)
	test.That(t, opts.MaxAccRads, test.ShouldAlmostEqual, math.Pi/2, 1e-9)
}

func TestMoveOptionsFromCfgVelOnly(t *testing.T) {
	opts := moveOptionsFromCfg(60, 0)
	test.That(t, opts, test.ShouldNotBeNil)
	test.That(t, opts.MaxVelRads, test.ShouldAlmostEqual, math.Pi/3, 1e-9)
	test.That(t, opts.MaxAccRads, test.ShouldEqual, 0.0)
}

func TestMoveOptionsFromCfgAccOnly(t *testing.T) {
	opts := moveOptionsFromCfg(0, 60)
	test.That(t, opts, test.ShouldNotBeNil)
	test.That(t, opts.MaxVelRads, test.ShouldEqual, 0.0)
	test.That(t, opts.MaxAccRads, test.ShouldAlmostEqual, math.Pi/3, 1e-9)
}

func TestArmAndGripperVsHeldAllowedNilWhenEmptyHanded(t *testing.T) {
	b := &bartender{cfg: &Config{ArmName: "arm-1", GripperName: "gripper-1"}}
	test.That(t, b.armAndGripperVsHeldAllowed(), test.ShouldBeNil)
}

func TestArmAndGripperVsHeldAllowedKeepsLowerArmChecked(t *testing.T) {
	b := &bartender{cfg: &Config{ArmName: "arm-1", GripperName: "gripper-1"}}
	frame, err := buildHeldObjectFrame(&HeldObjectGeometry{Type: "cylinder", RadiusMM: 40, LengthMM: 370}, nil)
	test.That(t, err, test.ShouldBeNil)
	b.heldGeomFrame = frame

	allowed := map[string]bool{}
	for _, a := range b.armAndGripperVsHeldAllowed() {
		test.That(t, a.Frame2, test.ShouldEqual, heldObjectFrameName)
		allowed[a.Frame1] = true
	}
	for _, f := range []string{"claws-middle", "gripper-1:claws", "gripper-1:case-gripper", "arm-1:link6"} {
		test.That(t, allowed[f], test.ShouldBeTrue)
	}
	for _, suffix := range []string{"link_base", "link1", "link2", "link3", "link4", "link5"} {
		test.That(t, allowed["arm-1:"+suffix], test.ShouldBeFalse)
	}
}
