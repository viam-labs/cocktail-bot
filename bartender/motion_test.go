package bartender

import (
	"context"
	"math"
	"os"
	"strings"
	"testing"
	"time"

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

// The wrist starts level, holding a shaker from the side, and the straight joint path to a full turn of
// J6 rolls it upside down and back.
func TestCarryToJointPoseKeepsASideHeldObjectLevel(t *testing.T) {
	carryHeldShakerToJointPose(t, heldServeFrameSystem(t), &HeldObjectGeometry{Type: "cylinder", RadiusMM: 10, LengthMM: 50},
		[]float64{0, -90, 90, 0, -90, -90}, []float64{0, -90, 90, 0, -90, 270}, true)
}

// The machine's serve-approach (rocks glass) and cleaner-hover joints on 2026-11-19, when this carry failed
// to plan: the shaker stays upright, but the gripper turns to face the other way.
func TestCarryFromServeApproachToCleanerHover(t *testing.T) {
	carryHeldShakerToJointPose(t, xarm850FrameSystem(t), &HeldObjectGeometry{Type: "cylinder", RadiusMM: 38, LengthMM: 166, ZOffsetMM: -35.93},
		[]float64{-29.2129, -71.6603, -96.752, -116.2623, 100.6769, 23.2066},
		[]float64{-67.9021, -34.6367, -24.5059, 26.5218, 96.0237, 7.0845}, false)
}

// carryHeldShakerToJointPose grabs the shaker upright at startDegs, carries it level to a joint-space pose at
// poseDegs, and checks it ends at those joints without tilting past the no-spill tolerance. A refusal
// passes only when mayRefuse, and only once the planner has searched.
func carryHeldShakerToJointPose(t *testing.T, parts []*referenceframe.FrameSystemPart, shaker *HeldObjectGeometry, startDegs, poseDegs []float64, mayRefuse bool) {
	t.Helper()
	b, sw, executed := jointPoseBartender(t, startDegs, poseDegs)
	fsSvc := inject.NewFrameSystemService("fs")
	fsSvc.FrameSystemConfigFunc = func(context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{Parts: parts}, nil
	}
	b.fsSvc = fsSvc
	b.cfg.GripperName = "gripper"
	ctx := context.Background()
	test.That(t, b.attachHeld(ctx, shaker), test.ShouldBeNil)
	fs, inputs, err := b.currentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	heldAxis := func(joints []referenceframe.Input) r3.Vector {
		inputs[b.cfg.ArmName] = joints
		tf, err := fs.Transform(inputs.ToLinearInputs(), referenceframe.NewZeroPoseInFrame(heldObjectFrameName), referenceframe.World)
		test.That(t, err, test.ShouldBeNil)
		return tf.(*referenceframe.PoseInFrame).Pose().Orientation().OrientationVectorRadians().Vector()
	}
	upright := heldAxis(degsToInputs(startDegs))

	planCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = b.carryHeldLevel(planCtx, sw, "cleaner-hover")
	if err != nil && mayRefuse {
		test.That(t, planCtx.Err(), test.ShouldNotBeNil)
		return
	}
	test.That(t, err, test.ShouldBeNil)
	last := (*executed)[len(*executed)-1]
	for i, want := range degsToInputs(poseDegs) {
		test.That(t, last[i], test.ShouldAlmostEqual, want, 1e-6)
	}
	prev := degsToInputs(startDegs)
	for _, next := range *executed {
		for step := 0; step <= 20; step++ {
			joints := make([]referenceframe.Input, len(next))
			for i := range next {
				joints[i] = prev[i] + (next[i]-prev[i])*float64(step)/20
			}
			test.That(t, upright.Angle(heldAxis(joints)).Degrees(), test.ShouldBeLessThan, defaultNoSpillOrientationToleranceDegs+5)
		}
		prev = next
	}
}

// xarm850FrameSystem is the bartender's xArm850, with link_6 renamed link6 as in the URDF the machine loads,
// and a gripper whose grip point sits 235 mm out from the flange like the machine's.
func xarm850FrameSystem(t *testing.T) []*referenceframe.FrameSystemPart {
	t.Helper()
	raw, err := os.ReadFile("testdata/xarm850_kinematics.json")
	test.That(t, err, test.ShouldBeNil)
	armModel, err := referenceframe.UnmarshalModelJSON([]byte(strings.ReplaceAll(string(raw), "link_6", "link6")), "arm")
	test.That(t, err, test.ShouldBeNil)
	gripperModel, err := referenceframe.UnmarshalModelJSON([]byte(`{"name": "gripper", "links": [
		{"id": "case-gripper", "parent": "world", "geometry": {"type": "box", "x": 60, "y": 60, "z": 60, "translation": {"x": 0, "y": 0, "z": 30}}},
		{"id": "claws", "parent": "case-gripper", "geometry": {"type": "box", "x": 20, "y": 60, "z": 60, "translation": {"x": 0, "y": 0, "z": 100}}}
	]}`), "gripper")
	test.That(t, err, test.ShouldBeNil)
	clawsMiddle, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{X: 42, Y: 55, Z: 115}, "claws-middle")
	test.That(t, err, test.ShouldBeNil)
	return []*referenceframe.FrameSystemPart{
		{FrameConfig: referenceframe.NewLinkInFrame(referenceframe.World, spatialmath.NewZeroPose(), "arm", nil), ModelFrame: armModel},
		{FrameConfig: referenceframe.NewLinkInFrame("arm", spatialmath.NewPoseFromPoint(r3.Vector{Z: 105}), "gripper", nil), ModelFrame: gripperModel},
		{FrameConfig: referenceframe.NewLinkInFrame("gripper", spatialmath.NewPoseFromPoint(r3.Vector{Z: 90}), "claws-middle", clawsMiddle)},
		{FrameConfig: referenceframe.NewLinkInFrame("gripper", spatialmath.NewPoseFromPoint(r3.Vector{Z: 130}), gripPointFrameName, nil)},
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

func TestJointPoseGoal(t *testing.T) {
	startDegs := []float64{-170, -90, 90, -90, -90, 170}
	poseDegs := []float64{170, -90, 90, -90, -90, -170}
	cases := []struct {
		name        string
		nearestTurn bool
		wantDegs    []float64
	}{
		{"takes the nearest turn when the pose allows it", true, []float64{-190, -90, 90, -90, -90, 190}},
		{"is exact otherwise", false, poseDegs},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, sw, executed := jointPoseBartender(t, startDegs, poseDegs)
			plain := sw.DoFunc
			sw.DoFunc = func(ctx context.Context, cmd map[string]any) (map[string]any, error) {
				resp, err := plain(ctx, cmd)
				resp["nearest_turn"] = tc.nearestTurn
				return resp, err
			}

			_, err := b.moveArmToJointGoalOnSwitch(context.Background(), sw, "prestage")
			test.That(t, err, test.ShouldBeNil)
			test.That(t, *executed, test.ShouldNotBeEmpty)
			last := (*executed)[len(*executed)-1]
			for i, want := range degsToInputs(tc.wantDegs) {
				test.That(t, last[i], test.ShouldAlmostEqual, want, 1e-6)
			}
		})
	}
}

func TestNearestTurn(t *testing.T) {
	fullTurns := referenceframe.Limit{Min: -2 * math.Pi, Max: 2 * math.Pi}
	deg := func(d float64) float64 { return d * math.Pi / 180 }
	cases := []struct {
		name                string
		goal, current, want float64
		limit               referenceframe.Limit
	}{
		{"goal already nearest", deg(40), deg(0), deg(40), fullTurns},
		{"a turn down is nearer", deg(170), deg(-170), deg(-190), fullTurns},
		{"a turn up is nearer", deg(-170), deg(170), deg(190), fullTurns},
		{"nearer turn is past the joint limit", deg(10), deg(355), deg(10), fullTurns},
		{"nearer turn is too close to the joint limit", deg(-40), deg(330), deg(-40), fullTurns},
		{"joint without a full turn of travel", deg(170), deg(-170), deg(170), referenceframe.Limit{Min: -math.Pi, Max: math.Pi}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			test.That(t, nearestTurn(tc.goal, tc.current, tc.limit), test.ShouldAlmostEqual, tc.want, 1e-9)
		})
	}
}
