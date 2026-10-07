package bartender

import (
	"testing"

	"go.viam.com/test"
)

func TestHeldBottleGeometryValidate(t *testing.T) {
	cases := []struct {
		name     string
		g        *HeldBottleGeometry
		wantErr  bool
		errMatch string
	}{
		{"nil ok", nil, false, ""},
		{"ok cylinder", &HeldBottleGeometry{Type: "cylinder", RadiusMM: 35, LengthMM: 300}, false, ""},
		{"wrong type", &HeldBottleGeometry{Type: "box", RadiusMM: 35, LengthMM: 300}, true, "cylinder"},
		{"zero radius", &HeldBottleGeometry{Type: "cylinder", RadiusMM: 0, LengthMM: 300}, true, "> 0"},
		{"negative length", &HeldBottleGeometry{Type: "cylinder", RadiusMM: 35, LengthMM: -1}, true, "> 0"},
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

func TestBuildHeldBottleFrameNil(t *testing.T) {
	frame, err := buildHeldBottleFrame(nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame, test.ShouldBeNil)
}

func TestBuildHeldBottleFrameCylinder(t *testing.T) {
	frame, err := buildHeldBottleFrame(&HeldBottleGeometry{Type: "cylinder", RadiusMM: 35, LengthMM: 300, ZOffsetMM: 150})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame, test.ShouldNotBeNil)
	test.That(t, frame.Name(), test.ShouldEqual, heldBottleFrameName)
}
