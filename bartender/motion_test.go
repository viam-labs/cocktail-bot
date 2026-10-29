package bartender

import (
	"context"
	"math"
	"testing"

	"github.com/golang/geo/r3"
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
