package bartender

import (
	"context"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/kinematics"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
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

func TestPickupAllowedCollisionsCoversGripperClaws(t *testing.T) {
	b := &bartender{cfg: &Config{
		ArmName:         "arm-1",
		GripperName:     "gripper-1",
		PickupObstacles: map[string]string{"slot-b-top-front": "slot-b-top-front-bottle"},
	}}
	allowed := map[string]bool{}
	for _, a := range b.pickupAllowedCollisions("slot-b-top-front") {
		test.That(t, a.Frame2, test.ShouldEqual, "slot-b-top-front-bottle")
		allowed[a.Frame1] = true
	}
	for _, f := range []string{"claws-middle", "gripper-1:claws", heldObjectFrameName} {
		test.That(t, allowed[f], test.ShouldBeTrue)
	}
	test.That(t, allowed["gripper-1:case-gripper"], test.ShouldBeFalse)
}

func TestPickupAllowedCollisionsNilWithoutObstacle(t *testing.T) {
	b := &bartender{cfg: &Config{GripperName: "gripper-1"}}
	test.That(t, b.pickupAllowedCollisions("slot-b-top-front"), test.ShouldBeNil)
}

func TestComputeCircularPoses(t *testing.T) {
	center := spatialmath.NewPose(
		r3.Vector{X: 100, Y: 200, Z: 300},
		&spatialmath.OrientationVectorDegrees{OX: 1, OY: 0, OZ: 0, Theta: 45},
	)
	const radius = 4.0
	const n = 8
	poses := computeCircularPoses(center, radius, n)
	test.That(t, len(poses), test.ShouldEqual, n)

	first := poses[0].Point()
	test.That(t, first.X, test.ShouldAlmostEqual, 104, 1e-9)
	test.That(t, first.Y, test.ShouldAlmostEqual, 200, 1e-9)

	chord := 2 * radius * math.Sin(math.Pi/n)
	for i, p := range poses {
		test.That(t, p.Point().Sub(center.Point()).Norm(), test.ShouldAlmostEqual, radius, 1e-9)
		test.That(t, p.Point().Z, test.ShouldAlmostEqual, center.Point().Z, 1e-9)
		test.That(t, spatialmath.OrientationAlmostEqual(p.Orientation(), center.Orientation()), test.ShouldBeTrue)
		next := poses[(i+1)%n].Point()
		test.That(t, p.Point().Sub(next).Norm(), test.ShouldAlmostEqual, chord, 1e-9)
	}
}

func TestFetchPoseCircleParams(t *testing.T) {
	cases := []struct {
		name       string
		resp       map[string]any
		wantRadius float64
		wantPoints int
	}{
		{"unset uses defaults", map[string]any{}, defaultCircleRadiusMm, defaultCirclePointsPerRev},
		{"over grpc", map[string]any{"circle_radius_mm": 15.0, "circle_points_per_rev": 12.0}, 15, 12},
		{"in process", map[string]any{"circle_radius_mm": 15.0, "circle_points_per_rev": 12}, 15, 12},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sw := inject.NewSwitch("stir")
			sw.DoFunc = func(context.Context, map[string]any) (map[string]any, error) {
				return tc.resp, nil
			}
			pd, err := fetchPose(context.Background(), sw, "stir-insert")
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pd.circleRadius(), test.ShouldEqual, tc.wantRadius)
			test.That(t, pd.circlePoints(), test.ShouldEqual, tc.wantPoints)
		})
	}
}

// jointPoseBartender has a UR5e at startDegs and a switch whose only pose is a joint-space pose at poseDegs.
func jointPoseBartender(t *testing.T, startDegs, poseDegs []float64) (*bartender, *inject.Switch, *[][]referenceframe.Input) {
	t.Helper()
	model, err := kinematics.ModelFromName("ur5e", "arm")
	test.That(t, err, test.ShouldBeNil)
	fsSvc := inject.NewFrameSystemService("fs")
	fsSvc.FrameSystemConfigFunc = func(context.Context) (*framesystem.Config, error) {
		lif := referenceframe.NewLinkInFrame(referenceframe.World, spatialmath.NewZeroPose(), "arm", nil)
		return &framesystem.Config{Parts: []*referenceframe.FrameSystemPart{{FrameConfig: lif, ModelFrame: model}}}, nil
	}
	a := inject.NewArm("arm")
	a.CurrentInputsFunc = func(context.Context) ([]referenceframe.Input, error) { return degsToInputs(startDegs), nil }
	var executed [][]referenceframe.Input
	a.MoveThroughJointPositionsFunc = func(_ context.Context, positions [][]referenceframe.Input, _ *arm.MoveOptions, _ map[string]any) error {
		executed = positions
		return nil
	}
	sw := inject.NewSwitch("stir")
	sw.DoFunc = func(context.Context, map[string]any) (map[string]any, error) {
		joints := make([]any, len(poseDegs))
		for i, d := range poseDegs {
			joints[i] = d
		}
		return map[string]any{"joint_positions_degs": joints, "component_name": "arm", "reference_frame": "world"}, nil
	}
	b := &bartender{cfg: &Config{ArmName: "arm"}, fsSvc: fsSvc, arm: a, logger: logging.NewTestLogger(t)}
	return b, sw, &executed
}

func degsToInputs(degs []float64) []referenceframe.Input {
	out := make([]referenceframe.Input, len(degs))
	for i, d := range degs {
		out[i] = d * math.Pi / 180
	}
	return out
}

func TestCarryToJointPoseEndsAtThoseJoints(t *testing.T) {
	poseDegs := []float64{40, -90, 90, -90, -90, -150}
	b, sw, executed := jointPoseBartender(t, []float64{0, -90, 90, -90, -90, 0}, poseDegs)

	_, err := b.carryHeldLevel(context.Background(), sw, "stir-hover")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, *executed, test.ShouldNotBeEmpty)
	last := (*executed)[len(*executed)-1]
	for i, want := range degsToInputs(poseDegs) {
		test.That(t, last[i], test.ShouldAlmostEqual, want, 1e-6)
	}
}

func TestCartesianPoseOfJointPoseIsWhereThoseJointsPutTheArm(t *testing.T) {
	poseDegs := []float64{40, -90, 90, -90, -90, -150}
	b, sw, _ := jointPoseBartender(t, []float64{0, -90, 90, -90, -90, 0}, poseDegs)
	pd, err := fetchPose(context.Background(), sw, "stir-hover")
	test.That(t, err, test.ShouldBeNil)

	got, err := b.cartesianPose(context.Background(), pd)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.jointPositionsDegs, test.ShouldBeEmpty)
	test.That(t, got.refFrame, test.ShouldEqual, referenceframe.World)

	model, err := kinematics.ModelFromName("ur5e", "arm")
	test.That(t, err, test.ShouldBeNil)
	want, err := model.Transform(degsToInputs(poseDegs))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostEqual(got.pose, want), test.ShouldBeTrue)

	cartesian := &poseData{pose: want, refFrame: referenceframe.World}
	same, err := b.cartesianPose(context.Background(), cartesian)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, same, test.ShouldEqual, cartesian)
}
