package bartender

import (
	"math"
	"testing"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/test"
)

// tiltTestFS is a one-joint "arm" rolling about world X with the held object on it, so the
// joint angle is the held object's tilt from upright.
func tiltTestFS(t *testing.T) *referenceframe.FrameSystem {
	t.Helper()
	fs := referenceframe.NewEmptyFrameSystem("test")
	joint, err := referenceframe.NewRotationalFrame("arm", spatialmath.R4AA{RX: 1}, referenceframe.Limit{Min: -2 * math.Pi, Max: 2 * math.Pi})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(joint, fs.World()), test.ShouldBeNil)
	held, err := referenceframe.NewStaticFrame(heldObjectFrameName, spatialmath.NewZeroPose())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(held, joint), test.ShouldBeNil)
	return fs
}

func degs(d ...float64) [][]referenceframe.Input {
	out := make([][]referenceframe.Input, len(d))
	for i, v := range d {
		out[i] = []referenceframe.Input{v * math.Pi / 180}
	}
	return out
}

func TestHeldTiltPastEndpoints(t *testing.T) {
	cases := []struct {
		name    string
		start   float64
		path    [][]referenceframe.Input
		wantErr bool
	}{
		{"direct pour tilt", 0, degs(0, 137), false},
		{"tilt back upright", 137, degs(137, 0), false},
		{"intentional flip", 0, degs(0, 180), false},
		{"empty path", 0, nil, false},
		{"unwinding a full turn to the same tilt", 0, degs(0, -223), true},
		{"wound start unwinds past upside down", -354, degs(-354, -132), true},
		{"overshoot within slack", 0, degs(0, 100, 90), false},
		{"overshoot past slack", 0, degs(0, 120, 90), true},
	}
	fs := tiltTestFS(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := referenceframe.FrameSystemInputs{"arm": degs(tc.start)[0]}
			err := heldTiltPastEndpoints(fs, start, "arm", tc.path, heldTiltSlackDegs)
			if tc.wantErr {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, "unwinds a wrist joint")
			} else {
				test.That(t, err, test.ShouldBeNil)
			}
		})
	}
}

func TestHeldTiltPastEndpointsLeavesStartInputsAlone(t *testing.T) {
	start := referenceframe.FrameSystemInputs{"arm": degs(0)[0]}
	test.That(t, heldTiltPastEndpoints(tiltTestFS(t), start, "arm", degs(0, 90), heldTiltSlackDegs), test.ShouldBeNil)
	test.That(t, start["arm"][0], test.ShouldEqual, 0)
}
