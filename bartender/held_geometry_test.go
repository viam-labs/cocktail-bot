package bartender

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/components/arm/kinematics"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
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
	test.That(t, b.attachParked(r3.Vector{}), test.ShouldBeNil)
	test.That(t, b.attachParked(r3.Vector{}), test.ShouldNotBeNil)

	_, err := b.handleCancel()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, b.parkedGeomFrame, test.ShouldBeNil)
	test.That(t, b.attachParked(r3.Vector{}), test.ShouldBeNil)
}

func parkedBartender(t *testing.T) *bartender {
	t.Helper()
	b := &bartender{cfg: &Config{
		ParkingSwitchName:  "parking",
		HeldBottleGeometry: &HeldObjectGeometry{Type: "cylinder", RadiusMM: 35, LengthMM: 300, ZOffsetMM: 150},
	}}
	test.That(t, b.attachParked(r3.Vector{}), test.ShouldBeNil)
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

func TestParkedBottleStandsUprightAtTheGripPoint(t *testing.T) {
	model, err := kinematics.ModelFromName("ur5e", "arm")
	test.That(t, err, test.ShouldBeNil)
	fsSvc := inject.NewFrameSystemService("fs")
	fsSvc.FrameSystemConfigFunc = func(context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{Parts: []*referenceframe.FrameSystemPart{
			{FrameConfig: referenceframe.NewLinkInFrame(referenceframe.World, spatialmath.NewZeroPose(), "arm", nil), ModelFrame: model},
			{FrameConfig: referenceframe.NewLinkInFrame("arm", spatialmath.NewPoseFromPoint(r3.Vector{Z: 50}), "gripper", nil)},
			{FrameConfig: referenceframe.NewLinkInFrame("gripper", spatialmath.NewPoseFromPoint(r3.Vector{Z: 130}), gripPointFrameName, nil)},
		}}, nil
	}
	a := inject.NewArm("arm")
	a.CurrentInputsFunc = func(context.Context) ([]referenceframe.Input, error) {
		return degsToInputs([]float64{0, -90, 90, -90, -90, 0}), nil
	}
	b := &bartender{
		cfg: &Config{
			ArmName:              "arm",
			GripperName:          "gripper",
			ParkedBottleGeometry: &HeldObjectGeometry{Type: "cylinder", RadiusMM: 55, LengthMM: 400, ZOffsetMM: -50},
		},
		fsSvc:  fsSvc,
		arm:    a,
		logger: logging.NewTestLogger(t),
	}

	approach := &spatialmath.OrientationVector{OX: -1, OY: 0.8, OZ: 0}
	approach.Normalize()
	flange := r3.Vector{X: -200, Y: 500, Z: 140}
	grab := &poseData{pose: spatialmath.NewPose(flange, approach), refFrame: referenceframe.World}

	gripPoint, err := b.gripPointAtPose(context.Background(), grab)
	test.That(t, err, test.ShouldBeNil)
	want := flange.Add(r3.Vector{X: approach.OX, Y: approach.OY}.Mul(180))
	test.That(t, gripPoint.Sub(want).Norm(), test.ShouldAlmostEqual, 0, 1e-6)

	test.That(t, b.attachParked(gripPoint), test.ShouldBeNil)
	fs := referenceframe.NewEmptyFrameSystem("parked")
	test.That(t, fs.AddFrame(b.parkedGeomFrame, fs.World()), test.ShouldBeNil)
	geoms, err := referenceframe.FrameSystemGeometries(fs, referenceframe.NewZeroInputs(fs))
	test.That(t, err, test.ShouldBeNil)
	capsule := geoms[parkedObjectFrameName].Geometries()[0]
	test.That(t, capsule.Pose().Point().Sub(gripPoint.Add(r3.Vector{Z: 50})).Norm(), test.ShouldAlmostEqual, 0, 1e-6)
	test.That(t, spatialmath.OrientationAlmostEqual(capsule.Pose().Orientation(), spatialmath.NewZeroOrientation()), test.ShouldBeTrue)
}
