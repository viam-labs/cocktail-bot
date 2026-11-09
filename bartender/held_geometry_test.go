package bartender

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/rdk/spatialmath"
	"go.viam.com/test"
)

func TestHeldObjectGeometryValidate(t *testing.T) {
	cases := []struct {
		name     string
		g        *HeldObjectGeometry
		wantErr  bool
		errMatch string
	}{
		{"nil ok", nil, false, ""},
		{"ok cylinder", &HeldObjectGeometry{Type: "cylinder", RadiusMM: 35, LengthMM: 300}, false, ""},
		{"wrong type", &HeldObjectGeometry{Type: "box", RadiusMM: 35, LengthMM: 300}, true, "cylinder"},
		{"zero radius", &HeldObjectGeometry{Type: "cylinder", RadiusMM: 0, LengthMM: 300}, true, "> 0"},
		{"negative length", &HeldObjectGeometry{Type: "cylinder", RadiusMM: 35, LengthMM: -1}, true, "> 0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.g.Validate("x")
			if tc.wantErr {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.errMatch)
			} else {
				test.That(t, err, test.ShouldBeNil)
			}
		})
	}
}

func TestBuildHeldObjectFrameNil(t *testing.T) {
	frame, err := buildHeldObjectFrame(nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame, test.ShouldBeNil)
}

func TestBuildHeldObjectFrameCylinder(t *testing.T) {
	frame, err := buildHeldObjectFrame(&HeldObjectGeometry{Type: "cylinder", RadiusMM: 35, LengthMM: 300, ZOffsetMM: 150}, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame, test.ShouldNotBeNil)
	test.That(t, frame.Name(), test.ShouldEqual, heldObjectFrameName)
}

func TestCancelClearsParkedGeometry(t *testing.T) {
	b := &bartender{cfg: &Config{
		HeldBottleGeometry: &HeldObjectGeometry{Type: "cylinder", RadiusMM: 35, LengthMM: 300, ZOffsetMM: 150},
	}}
	test.That(t, b.attachParked(spatialmath.NewZeroPose()), test.ShouldBeNil)
	test.That(t, b.attachParked(spatialmath.NewZeroPose()), test.ShouldNotBeNil)

	_, err := b.handleCancel()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, b.parkedGeomFrame, test.ShouldBeNil)
	test.That(t, b.attachParked(spatialmath.NewZeroPose()), test.ShouldBeNil)
}

func parkedBartender(t *testing.T) *bartender {
	t.Helper()
	b := &bartender{cfg: &Config{
		ParkingSwitchName:  "parking",
		HeldBottleGeometry: &HeldObjectGeometry{Type: "cylinder", RadiusMM: 35, LengthMM: 300, ZOffsetMM: 150},
	}}
	test.That(t, b.attachParked(spatialmath.NewZeroPose()), test.ShouldBeNil)
	return b
}

func TestMakeCocktailRefusedWhileParkingOccupied(t *testing.T) {
	b := parkedBartender(t)
	_, err := b.handleMakeCocktail(context.Background(), map[string]any{"drink_id": "anything"})
	test.That(t, errors.Is(err, errParkingOccupied), test.ShouldBeTrue)
}

func TestShuffleToParkingRefusedBeforeMotionWhileOccupied(t *testing.T) {
	b := parkedBartender(t)
	err := b.shuffleBlockerToParking(context.Background(), "blocker")
	test.That(t, errors.Is(err, errParkingOccupied), test.ShouldBeTrue)
}
