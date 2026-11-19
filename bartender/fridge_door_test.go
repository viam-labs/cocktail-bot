package bartender

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/gripper"
	toggleswitch "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
)

func testFridge() *Fridge {
	return &Fridge{DoorSwitchName: "fridge-door-handle", DoorObstacle: "fridge-door", Hinge: &FridgeHinge{X: 100, Y: 0}}
}

func TestConfigValidateFridge(t *testing.T) {
	base := func(f *Fridge) *Config {
		return &Config{ArmName: "arm", GripperName: "gripper", PoseSwitcherNames: []string{"fridge-door-handle"}, Fridge: f}
	}
	with := func(edit func(*Fridge)) *Fridge {
		f := testFridge()
		edit(f)
		return f
	}
	cases := []struct {
		name     string
		f        *Fridge
		errMatch string
	}{
		{"unset", nil, ""},
		{"ok", testFridge(), ""},
		{"no switch", with(func(f *Fridge) { f.DoorSwitchName = "" }), "door_switch_name is required"},
		{"switch not a switcher", with(func(f *Fridge) { f.DoorSwitchName = "nope" }), "not in pose_switcher_names"},
		{"no obstacle", with(func(f *Fridge) { f.DoorObstacle = "" }), "door_obstacle is required"},
		{"no hinge", with(func(f *Fridge) { f.Hinge = nil }), "hinge is required"},
		{"negative step", with(func(f *Fridge) { f.SwingStepDegs = -1 }), "swing_step_degs"},
		{"negative vel", with(func(f *Fridge) { f.SwingVelDegsPerSec = -1 }), "swing_vel_degs_per_sec"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deps, _, err := base(tc.f).Validate("svc")
			if tc.errMatch != "" {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.errMatch)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			if tc.f != nil {
				test.That(t, deps, test.ShouldContain, gripper.Named("fridge-door").String())
			}
		})
	}
}

func TestSwingAboutHinge(t *testing.T) {
	f := testFridge()
	p := spatialmath.NewPose(r3.Vector{X: 500, Y: 0, Z: 300}, &spatialmath.OrientationVectorDegrees{OX: 1, Theta: 0})
	swung := f.swingAboutHinge(p, 90)
	test.That(t, swung.Point().X, test.ShouldAlmostEqual, 100, 1e-6)
	test.That(t, swung.Point().Y, test.ShouldAlmostEqual, 400, 1e-6)
	test.That(t, swung.Point().Z, test.ShouldAlmostEqual, 300, 1e-6)
	want := spatialmath.NewPoseFromOrientation(&spatialmath.OrientationVectorDegrees{OY: 1, Theta: 0})
	test.That(t, spatialmath.OrientationAlmostEqual(swung.Orientation(), want.Orientation()), test.ShouldBeTrue)
}

func TestFridgeOpenDegs(t *testing.T) {
	f := testFridge()
	closed := spatialmath.NewPose(r3.Vector{X: 500, Y: 0, Z: 300}, &spatialmath.OrientationVectorDegrees{OX: 1})
	for _, degs := range []float64{90, -60} {
		got, err := f.openDegs(closed, f.swingAboutHinge(closed, degs))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldAlmostEqual, degs, 1e-6)
	}

	open := f.swingAboutHinge(closed, 90)
	_, err := f.openDegs(closed, spatialmath.NewPose(open.Point().Add(r3.Vector{Y: 40}), open.Orientation()))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "mm from where closed-grab lands")

	_, err = f.openDegs(closed, spatialmath.NewPose(open.Point(), closed.Orientation()))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "hold the handle the same way")

	_, err = f.openDegs(closed, closed)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "would not move")
}

func doorPart(t *testing.T, parent string, at r3.Vector) *referenceframe.FrameSystemPart {
	t.Helper()
	box, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{X: 20, Y: 400, Z: 800}, "fridge-door")
	test.That(t, err, test.ShouldBeNil)
	return &referenceframe.FrameSystemPart{FrameConfig: referenceframe.NewLinkInFrame(parent, spatialmath.NewPoseFromPoint(at), "fridge-door", box)}
}

func TestSwingDoorPart(t *testing.T) {
	f := testFridge()
	other := &referenceframe.FrameSystemPart{FrameConfig: referenceframe.NewLinkInFrame(referenceframe.World, spatialmath.NewZeroPose(), "fridge", nil)}
	door := doorPart(t, referenceframe.World, r3.Vector{X: 400, Z: 500})
	parts := []*referenceframe.FrameSystemPart{other, door}

	swung, err := f.swingDoorPart(parts, 90)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, swung[0], test.ShouldEqual, other)
	test.That(t, parts[1], test.ShouldEqual, door)
	p := swung[1].FrameConfig.Pose().Point()
	test.That(t, p.X, test.ShouldAlmostEqual, 100, 1e-6)
	test.That(t, p.Y, test.ShouldAlmostEqual, 300, 1e-6)
	test.That(t, p.Z, test.ShouldAlmostEqual, 500, 1e-6)
	test.That(t, swung[1].FrameConfig.Geometry(), test.ShouldEqual, door.FrameConfig.Geometry())

	_, err = f.swingDoorPart([]*referenceframe.FrameSystemPart{other}, 90)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not in the frame system")

	_, err = f.swingDoorPart([]*referenceframe.FrameSystemPart{doorPart(t, "fridge", r3.Vector{})}, 90)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "must be parented to world")
}

func TestFridgeDoorStateSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	ds, err := newDataStore(dir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.FridgeDoorOpenDegs(), test.ShouldEqual, 0)
	test.That(t, ds.SetFridgeDoorOpenDegs(75), test.ShouldBeNil)

	reopened, err := newDataStore(dir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reopened.FridgeDoorOpenDegs(), test.ShouldEqual, 75)
}

func openDoorBartender(t *testing.T) *bartender {
	t.Helper()
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.SetFridgeDoorOpenDegs(80), test.ShouldBeNil)
	return &bartender{cfg: &Config{Fridge: testFridge()}, dataStore: ds}
}

func TestMakeCocktailRefusedWhileFridgeDoorOpen(t *testing.T) {
	b := openDoorBartender(t)
	_, err := b.handleMakeCocktail(context.Background(), map[string]any{"drink_id": "anything"})
	test.That(t, errors.Is(err, errFridgeDoorOpen), test.ShouldBeTrue)
}

func TestCancelMarksFridgeDoorShutOnlyWhenIdle(t *testing.T) {
	b := openDoorBartender(t)
	_, stop := b.withCancel(context.Background())
	defer stop()
	_, err := b.handleCancel()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, b.fridgeDoorOpenDegs(), test.ShouldEqual, 80)

	_, err = b.handleCancel()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, b.fridgeDoorOpenDegs(), test.ShouldEqual, 0)
	test.That(t, b.checkFridgeDoorShut(), test.ShouldBeNil)
}

func poseResponse(p spatialmath.Pose) map[string]any {
	ov := p.Orientation().OrientationVectorDegrees()
	return map[string]any{
		"x": p.Point().X, "y": p.Point().Y, "z": p.Point().Z,
		"o_x": ov.OX, "o_y": ov.OY, "o_z": ov.OZ, "theta": ov.Theta,
		"component_name": "arm", "reference_frame": referenceframe.World,
	}
}

// fridgeTestBartender is a UR5e whose current end effector pose is closed-grab, with the hinge
// 300 mm away so the door opens 30°.
func fridgeTestBartender(t *testing.T) (*bartender, *[][][]referenceframe.Input) {
	t.Helper()
	parts := append(heldServeFrameSystem(t), doorPart(t, referenceframe.World, r3.Vector{X: 2000, Y: 2000, Z: 400}))
	fsSvc := inject.NewFrameSystemService("fs")
	fsSvc.FrameSystemConfigFunc = func(context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{Parts: parts}, nil
	}
	current := degsToInputs([]float64{0, -90, 90, -90, -90, 0})
	var moves [][][]referenceframe.Input
	a := inject.NewArm("arm")
	a.CurrentInputsFunc = func(context.Context) ([]referenceframe.Input, error) { return current, nil }
	a.MoveThroughJointPositionsFunc = func(_ context.Context, positions [][]referenceframe.Input, _ *arm.MoveOptions, _ map[string]any) error {
		current = positions[len(positions)-1]
		moves = append(moves, positions)
		return nil
	}
	g := inject.NewGripper("gripper")
	g.OpenFunc = func(context.Context, map[string]any) error { return nil }
	g.GrabFunc = func(context.Context, map[string]any) (bool, error) { return true, nil }
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)

	fs, err := referenceframe.NewFrameSystem("test", parts, nil)
	test.That(t, err, test.ShouldBeNil)
	inputs := referenceframe.NewZeroInputs(fs)
	inputs["arm"] = current
	tf, err := fs.Transform(inputs.ToLinearInputs(), referenceframe.NewPoseInFrame("arm", spatialmath.NewZeroPose()), referenceframe.World)
	test.That(t, err, test.ShouldBeNil)
	closedGrab := tf.(*referenceframe.PoseInFrame).Pose()
	f := &Fridge{
		DoorSwitchName: "fridge-door-handle",
		DoorObstacle:   "fridge-door",
		Hinge:          &FridgeHinge{X: closedGrab.Point().X - 300, Y: closedGrab.Point().Y},
	}
	openGrab := f.swingAboutHinge(closedGrab, 30)
	back := func(p spatialmath.Pose) spatialmath.Pose {
		return spatialmath.NewPose(p.Point().Add(r3.Vector{Z: 40}), p.Orientation())
	}
	poses := map[string]spatialmath.Pose{
		poseFridgeClosedHover: back(closedGrab), poseFridgeClosedGrab: closedGrab, poseFridgeClosedRetreat: back(closedGrab),
		poseFridgeOpenHover: back(openGrab), poseFridgeOpenGrab: openGrab, poseFridgeOpenRetreat: back(openGrab),
	}
	sw := inject.NewSwitch("fridge-door-handle")
	sw.DoFunc = func(_ context.Context, cmd map[string]any) (map[string]any, error) {
		p, ok := poses[cmd["get_pose_by_name"].(string)]
		if !ok {
			return nil, errors.New("unknown pose")
		}
		return poseResponse(p), nil
	}
	return &bartender{
		cfg:          &Config{ArmName: "arm", GripperName: "gripper", Fridge: f},
		fsSvc:        fsSvc,
		arm:          a,
		gripper:      g,
		logger:       logging.NewTestLogger(t),
		poseSwitches: []toggleswitch.Switch{sw},
		dataStore:    ds,
	}, &moves
}

func endEffectorAt(t *testing.T, b *bartender, joints []referenceframe.Input) r3.Vector {
	t.Helper()
	fs, inputs, err := b.currentInputs(context.Background())
	test.That(t, err, test.ShouldBeNil)
	inputs["arm"] = joints
	tf, err := fs.Transform(inputs.ToLinearInputs(), referenceframe.NewPoseInFrame("arm", spatialmath.NewZeroPose()), referenceframe.World)
	test.That(t, err, test.ShouldBeNil)
	return tf.(*referenceframe.PoseInFrame).Pose().Point()
}

func TestOpenAndCloseFridgeDoor(t *testing.T) {
	b, moves := fridgeTestBartender(t)
	ctx := context.Background()
	closedGrab, err := b.fridgeGrabInWorld(ctx, b.poseSwitches[0], poseFridgeClosedGrab)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, b.openFridgeDoor(ctx), test.ShouldBeNil)
	test.That(t, b.fridgeDoorOpenDegs(), test.ShouldAlmostEqual, 30, 1e-6)
	// closed-hover, closed-grab, the swing as one move, open-retreat.
	test.That(t, len(*moves), test.ShouldEqual, 4)
	swing := (*moves)[2]
	test.That(t, len(swing), test.ShouldBeGreaterThan, 3)
	end := endEffectorAt(t, b, swing[len(swing)-1])
	test.That(t, end.Sub(b.cfg.Fridge.swingAboutHinge(closedGrab.pose, 30).Point()).Norm(), test.ShouldBeLessThan, 2)

	err = b.openFridgeDoor(ctx)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "already recorded open")

	test.That(t, b.closeFridgeDoor(ctx), test.ShouldBeNil)
	test.That(t, b.fridgeDoorOpenDegs(), test.ShouldEqual, 0)
	test.That(t, len(*moves), test.ShouldEqual, 8)
	swing = (*moves)[6]
	end = endEffectorAt(t, b, swing[len(swing)-1])
	test.That(t, end.Sub(closedGrab.pose.Point()).Norm(), test.ShouldBeLessThan, 2)

	err = b.closeFridgeDoor(ctx)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "already recorded shut")
}

func TestCurrentInputsSwingsOpenDoorObstacle(t *testing.T) {
	b, _ := fridgeTestBartender(t)
	doorAt := func() r3.Vector {
		fs, inputs, err := b.currentInputs(context.Background())
		test.That(t, err, test.ShouldBeNil)
		tf, err := fs.Transform(inputs.ToLinearInputs(), referenceframe.NewPoseInFrame("fridge-door", spatialmath.NewZeroPose()), referenceframe.World)
		test.That(t, err, test.ShouldBeNil)
		return tf.(*referenceframe.PoseInFrame).Pose().Point()
	}
	shut := doorAt()
	test.That(t, b.dataStore.SetFridgeDoorOpenDegs(30), test.ShouldBeNil)
	open := doorAt()
	want := b.cfg.Fridge.swingAboutHinge(spatialmath.NewPoseFromPoint(shut), 30).Point()
	test.That(t, open.Sub(want).Norm(), test.ShouldBeLessThan, 1e-6)
}
