package bartender

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/kinematics"
	toggleswitch "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
)

func TestParsePourFromShaker(t *testing.T) {
	cases := []struct {
		name     string
		raw      any
		station  string
		pourMs   int
		wantErr  bool
		errMatch string
	}{
		{"ok", map[string]any{"station": "serving", "pour_ms": 5000.0}, "serving", 5000, false, ""},
		{"ok int", map[string]any{"station": "s", "pour_ms": 100}, "s", 100, false, ""},
		{"zero pour", map[string]any{"station": "s", "pour_ms": 0.0}, "s", 0, false, ""},
		{"not object", "serving", "", 0, true, "object"},
		{"missing station", map[string]any{"pour_ms": 100}, "", 0, true, "station"},
		{"missing pour", map[string]any{"station": "s"}, "", 0, true, "pour_ms"},
		{"negative pour", map[string]any{"station": "s", "pour_ms": -1.0}, "", 0, true, ">= 0"},
		{"string pour", map[string]any{"station": "s", "pour_ms": "100"}, "", 0, true, "must be a number"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			station, pourMs, err := parsePourFromShaker(tc.raw)
			if tc.wantErr {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.errMatch)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, station, test.ShouldEqual, tc.station)
			test.That(t, pourMs, test.ShouldEqual, tc.pourMs)
		})
	}
}

type servePose struct {
	degs []float64
	vel  float64
	acc  float64
}

type executedMove struct {
	end  []referenceframe.Input
	opts *arm.MoveOptions
}

// serveTestBartender holds a shaker on a UR5e and has a serving station whose serve poses are joint-space.
func serveTestBartender(t *testing.T, poses map[string]servePose) (*bartender, *inject.Switch, *[]executedMove) {
	t.Helper()
	parts := heldServeFrameSystem(t)
	fsSvc := inject.NewFrameSystemService("fs")
	fsSvc.FrameSystemConfigFunc = func(context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{Parts: parts}, nil
	}
	current := degsToInputs([]float64{0, -90, 90, -90, -90, 0})
	var moves []executedMove
	a := inject.NewArm("arm")
	a.CurrentInputsFunc = func(context.Context) ([]referenceframe.Input, error) { return current, nil }
	a.MoveThroughJointPositionsFunc = func(_ context.Context, positions [][]referenceframe.Input, opts *arm.MoveOptions, _ map[string]any) error {
		current = positions[len(positions)-1]
		moves = append(moves, executedMove{end: current, opts: opts})
		return nil
	}
	sw := inject.NewSwitch("serving")
	sw.GetNumberOfPositionsFunc = func(context.Context, map[string]any) (uint32, []string, error) {
		var names []string
		for n := range poses {
			names = append(names, n)
		}
		return uint32(len(names)), names, nil
	}
	sw.DoFunc = func(_ context.Context, cmd map[string]any) (map[string]any, error) {
		p := poses[cmd["get_pose_by_name"].(string)]
		joints := make([]any, len(p.degs))
		for i, d := range p.degs {
			joints[i] = d
		}
		resp := map[string]any{"joint_positions_degs": joints, "component_name": "arm", "reference_frame": "world"}
		if p.vel > 0 {
			resp["vel_degs_per_sec"] = p.vel
		}
		if p.acc > 0 {
			resp["acc_degs_per_sec2"] = p.acc
		}
		return resp, nil
	}
	held, err := buildHeldObjectFrame(&HeldObjectGeometry{Type: "cylinder", RadiusMM: 10, LengthMM: 50}, nil)
	test.That(t, err, test.ShouldBeNil)
	b := &bartender{
		cfg:           &Config{ArmName: "arm", GripperName: "gripper"},
		fsSvc:         fsSvc,
		arm:           a,
		logger:        logging.NewTestLogger(t),
		poseSwitches:  []toggleswitch.Switch{sw},
		heldGeomFrame: held,
	}
	return b, sw, &moves
}

// heldServeFrameSystem is a UR5e (ee_link renamed link6) with a gripper, so the planner finds every geometry
// that armAndGripperVsHeldAllowed names.
func heldServeFrameSystem(t *testing.T) []*referenceframe.FrameSystemPart {
	t.Helper()
	ur5e, err := kinematics.ModelFromName("ur5e", "arm")
	test.That(t, err, test.ShouldBeNil)
	armJSON, err := json.Marshal(ur5e.ModelConfig())
	test.That(t, err, test.ShouldBeNil)
	armModel, err := referenceframe.UnmarshalModelJSON([]byte(strings.ReplaceAll(string(armJSON), "ee_link", "link6")), "arm")
	test.That(t, err, test.ShouldBeNil)
	gripperModel, err := referenceframe.UnmarshalModelJSON([]byte(`{"name": "gripper", "links": [
		{"id": "case-gripper", "parent": "world", "geometry": {"type": "box", "x": 10, "y": 10, "z": 10, "translation": {"x": 0, "y": 0, "z": 300}}},
		{"id": "claws", "parent": "case-gripper", "geometry": {"type": "box", "x": 10, "y": 10, "z": 10, "translation": {"x": 0, "y": 0, "z": 400}}}
	]}`), "gripper")
	test.That(t, err, test.ShouldBeNil)
	clawsMiddle, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{Z: 500}), r3.Vector{X: 10, Y: 10, Z: 10}, "claws-middle")
	test.That(t, err, test.ShouldBeNil)
	return []*referenceframe.FrameSystemPart{
		{FrameConfig: referenceframe.NewLinkInFrame(referenceframe.World, spatialmath.NewZeroPose(), "arm", nil), ModelFrame: armModel},
		{FrameConfig: referenceframe.NewLinkInFrame("arm", spatialmath.NewZeroPose(), "gripper", nil), ModelFrame: gripperModel},
		{FrameConfig: referenceframe.NewLinkInFrame("gripper", spatialmath.NewZeroPose(), "claws-middle", clawsMiddle)},
		{FrameConfig: referenceframe.NewLinkInFrame("gripper", spatialmath.NewZeroPose(), gripPointFrameName, nil)},
	}
}

// moveEndingAt names the pose whose joints a move ended at.
func moveEndingAt(t *testing.T, m executedMove, poses map[string]servePose) string {
	t.Helper()
	for name, p := range poses {
		match := true
		for i, want := range degsToInputs(p.degs) {
			if math.Abs(m.end[i]-want) > 1e-6 {
				match = false
				break
			}
		}
		if match {
			return name
		}
	}
	return ""
}

func degsPerSec(d float64) float64 { return d * math.Pi / 180 }

func TestServeFromShaker(t *testing.T) {
	approach := servePose{degs: []float64{20, -90, 90, -90, -90, 0}}
	preTilt := servePose{degs: []float64{20, -90, 90, -90, -90, 60}, vel: 90, acc: 125}
	tilt := servePose{degs: []float64{20, -90, 90, -90, -90, 100}, vel: 10, acc: 15}

	t.Run("without serve-pre-tilt tilts in one slow move", func(t *testing.T) {
		poses := map[string]servePose{poseServeApproach: approach, poseServeTilt: tilt}
		b, sw, moves := serveTestBartender(t, poses)

		test.That(t, b.serveFromShaker(context.Background(), sw, 0), test.ShouldBeNil)
		var got []string
		for _, m := range *moves {
			got = append(got, moveEndingAt(t, m, poses))
		}
		test.That(t, got, test.ShouldResemble, []string{poseServeApproach, poseServeTilt, poseServeApproach})
		tiltMove := (*moves)[1]
		test.That(t, tiltMove.opts.MaxVelRads, test.ShouldAlmostEqual, degsPerSec(10), 1e-9)
		test.That(t, tiltMove.opts.MaxAccRads, test.ShouldAlmostEqual, degsPerSec(15), 1e-9)
	})

	t.Run("with serve-pre-tilt goes there fast then pours slow", func(t *testing.T) {
		poses := map[string]servePose{poseServeApproach: approach, poseServePreTilt: preTilt, poseServeTilt: tilt}
		b, sw, moves := serveTestBartender(t, poses)

		test.That(t, b.serveFromShaker(context.Background(), sw, 0), test.ShouldBeNil)
		var got []string
		for _, m := range *moves {
			got = append(got, moveEndingAt(t, m, poses))
		}
		test.That(t, got, test.ShouldResemble, []string{poseServeApproach, poseServePreTilt, poseServeTilt, poseServeApproach})
		preTiltMove, tiltMove := (*moves)[1], (*moves)[2]
		test.That(t, preTiltMove.opts.MaxVelRads, test.ShouldAlmostEqual, degsPerSec(90), 1e-9)
		test.That(t, preTiltMove.opts.MaxAccRads, test.ShouldAlmostEqual, degsPerSec(125), 1e-9)
		test.That(t, tiltMove.opts.MaxVelRads, test.ShouldAlmostEqual, degsPerSec(10), 1e-9)
		test.That(t, tiltMove.opts.MaxAccRads, test.ShouldAlmostEqual, degsPerSec(15), 1e-9)
	})

	t.Run("serve-pre-tilt without a speed runs at the arm default", func(t *testing.T) {
		poses := map[string]servePose{
			poseServeApproach: approach,
			poseServePreTilt:  {degs: preTilt.degs},
			poseServeTilt:     tilt,
		}
		b, sw, moves := serveTestBartender(t, poses)

		test.That(t, b.serveFromShaker(context.Background(), sw, 0), test.ShouldBeNil)
		test.That(t, len(*moves), test.ShouldEqual, 4)
		test.That(t, moveEndingAt(t, (*moves)[1], poses), test.ShouldEqual, poseServePreTilt)
		test.That(t, (*moves)[1].opts, test.ShouldBeNil)
	})
}
