package bartender

import (
	"context"
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

const testShakerAttachment = "shaker-attachment"

// shakerAttachmentBartender has a ur5e at joints, a grip point 180 mm past its flange, and the shaker
// attachment standing at home.
func shakerAttachmentBartender(t *testing.T, joints *[]float64, home spatialmath.Pose) *bartender {
	t.Helper()
	model, err := kinematics.ModelFromName("ur5e", "arm")
	test.That(t, err, test.ShouldBeNil)
	cube, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{X: 25, Y: 25, Z: 25}, testShakerAttachment)
	test.That(t, err, test.ShouldBeNil)
	fsSvc := inject.NewFrameSystemService("fs")
	fsSvc.FrameSystemConfigFunc = func(context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{Parts: []*referenceframe.FrameSystemPart{
			{FrameConfig: referenceframe.NewLinkInFrame(referenceframe.World, spatialmath.NewZeroPose(), "arm", nil), ModelFrame: model},
			{FrameConfig: referenceframe.NewLinkInFrame("arm", spatialmath.NewPoseFromPoint(r3.Vector{Z: 50}), "gripper", nil)},
			{FrameConfig: referenceframe.NewLinkInFrame("gripper", spatialmath.NewPoseFromPoint(r3.Vector{Z: 130}), gripPointFrameName, nil)},
			{FrameConfig: referenceframe.NewLinkInFrame(referenceframe.World, home, testShakerAttachment, cube)},
		}}, nil
	}
	a := inject.NewArm("arm")
	a.CurrentInputsFunc = func(context.Context) ([]referenceframe.Input, error) {
		return degsToInputs(*joints), nil
	}
	g := inject.NewGripper("gripper")
	g.OpenFunc = func(context.Context, map[string]interface{}) error { return nil }
	return &bartender{
		cfg: &Config{
			ArmName:                  "arm",
			GripperName:              "gripper",
			PickupObstacles:          map[string]string{shakerPickupKey: "shaker-obstacle"},
			ShakerAttachmentObstacle: testShakerAttachment,
		},
		fsSvc:   fsSvc,
		arm:     a,
		gripper: g,
		logger:  logging.NewTestLogger(t),
	}
}

func shakerGeometry() *HeldObjectGeometry {
	return &HeldObjectGeometry{Type: "cylinder", RadiusMM: 38, LengthMM: 166, ZOffsetMM: -36}
}

// worldGeometry is the one geometry frame carries, in the world, with the arm where it is now.
func worldGeometry(t *testing.T, b *bartender, frame string) (spatialmath.Geometry, bool) {
	t.Helper()
	fs, inputs, err := b.currentInputs(context.Background())
	test.That(t, err, test.ShouldBeNil)
	geoms, err := referenceframe.FrameSystemGeometries(fs, inputs)
	test.That(t, err, test.ShouldBeNil)
	g, ok := geoms[frame]
	if !ok || len(g.Geometries()) == 0 {
		return nil, false
	}
	return g.Geometries()[0], true
}

func gripPointInWorld(t *testing.T, b *bartender) spatialmath.Pose {
	t.Helper()
	fs, inputs, err := b.currentInputs(context.Background())
	test.That(t, err, test.ShouldBeNil)
	tf, err := fs.Transform(inputs.ToLinearInputs(), referenceframe.NewZeroPoseInFrame(gripPointFrameName), referenceframe.World)
	test.That(t, err, test.ShouldBeNil)
	return tf.(*referenceframe.PoseInFrame).Pose()
}

func TestShakerAttachmentRidesWithTheShakerAndStaysWhereItIsSetDown(t *testing.T) {
	ctx := context.Background()
	joints := []float64{0, -90, 90, -90, -90, 0}
	home := spatialmath.NewPose(r3.Vector{X: 167, Y: 606, Z: 157.5}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 45})
	b := shakerAttachmentBartender(t, &joints, home)

	atHome, ok := worldGeometry(t, b, testShakerAttachment+"_origin")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, spatialmath.PoseAlmostEqual(atHome.Pose(), home), test.ShouldBeTrue)

	test.That(t, b.attachHeldShaker(ctx, shakerGeometry()), test.ShouldBeNil)
	test.That(t, b.absentObstacles()[testShakerAttachment], test.ShouldBeTrue)
	_, ok = worldGeometry(t, b, testShakerAttachment+"_origin")
	test.That(t, ok, test.ShouldBeFalse)
	gripped, ok := worldGeometry(t, b, heldShakerAttachmentFrameName)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, spatialmath.PoseAlmostEqual(gripped.Pose(), home), test.ShouldBeTrue)

	inGrip := spatialmath.PoseBetween(gripPointInWorld(t, b), home)
	joints = []float64{20, -80, 70, -60, -90, 45}
	setDown := spatialmath.Compose(gripPointInWorld(t, b), inGrip)
	carried, ok := worldGeometry(t, b, heldShakerAttachmentFrameName)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, spatialmath.PoseAlmostEqual(carried.Pose(), setDown), test.ShouldBeTrue)
	test.That(t, spatialmath.PoseAlmostEqual(carried.Pose(), home), test.ShouldBeFalse)

	test.That(t, b.setShakerDownAway(ctx), test.ShouldBeNil)
	test.That(t, b.heldGeomFrame, test.ShouldBeNil)
	test.That(t, b.heldShakerAttachmentFrame, test.ShouldBeNil)
	test.That(t, b.absentObstacles()[testShakerAttachment], test.ShouldBeTrue)
	joints = []float64{0, -90, 90, -90, -90, 0}
	parked, ok := worldGeometry(t, b, parkedShakerAttachmentFrameName)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, spatialmath.PoseAlmostEqual(parked.Pose(), setDown), test.ShouldBeTrue)

	joints = []float64{20, -80, 70, -60, -90, 45}
	test.That(t, b.attachHeldShaker(ctx, shakerGeometry()), test.ShouldBeNil)
	test.That(t, b.parkedShakerAttachmentFrame, test.ShouldBeNil)
	regripped, ok := worldGeometry(t, b, heldShakerAttachmentFrameName)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, spatialmath.PoseAlmostEqual(regripped.Pose(), setDown), test.ShouldBeTrue)

	test.That(t, b.gripperOpen(ctx), test.ShouldBeNil)
	test.That(t, b.absentObstacles()[testShakerAttachment], test.ShouldBeFalse)
}

func TestCancelPutsTheShakerAttachmentBackHome(t *testing.T) {
	joints := []float64{0, -90, 90, -90, -90, 0}
	b := shakerAttachmentBartender(t, &joints, spatialmath.NewPoseFromPoint(r3.Vector{X: 167, Y: 606, Z: 157.5}))
	test.That(t, b.attachHeldShaker(context.Background(), shakerGeometry()), test.ShouldBeNil)
	test.That(t, b.setShakerDownAway(context.Background()), test.ShouldBeNil)
	test.That(t, b.absentObstacles()[testShakerAttachment], test.ShouldBeTrue)

	_, err := b.handleCancel()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, b.parkedShakerAttachmentFrame, test.ShouldBeNil)
	test.That(t, b.absentObstacles()[testShakerAttachment], test.ShouldBeFalse)
}

func TestBottleGripLeavesTheShakerAttachmentHome(t *testing.T) {
	joints := []float64{0, -90, 90, -90, -90, 0}
	b := shakerAttachmentBartender(t, &joints, spatialmath.NewPoseFromPoint(r3.Vector{X: 167, Y: 606, Z: 157.5}))
	test.That(t, b.attachHeld(context.Background(), shakerGeometry()), test.ShouldBeNil)
	test.That(t, b.heldShakerAttachmentFrame, test.ShouldBeNil)
	test.That(t, b.absentObstacles()[testShakerAttachment], test.ShouldBeFalse)
}

func TestShakerAttachmentAllowedCollisions(t *testing.T) {
	b := strainerTestBartender(t)
	b.cfg.PickupObstacles = map[string]string{shakerPickupKey: "shaker-obstacle", "slot-a": "slot-a-bottle"}
	b.cfg.MixerProtectorObstacle = "mixer-protector"
	b.heldShakerAttachmentFrame = referenceframe.NewZeroStaticFrame(heldShakerAttachmentFrameName)
	ear := func(obstacle string) AllowedCollision {
		return AllowedCollision{Frame1: heldShakerAttachmentFrameName, Frame2: obstacle}
	}

	test.That(t, b.pickupAllowedCollisions(shakerPickupKey), test.ShouldContain, ear("shaker-obstacle"))
	test.That(t, b.pickupAllowedCollisions("slot-a"), test.ShouldNotContain, ear("slot-a-bottle"))
	test.That(t, b.mixerProtectorAllowed(), test.ShouldContain, ear("mixer-protector"))
	test.That(t, b.strainerOnMountAllowed(), test.ShouldContain, ear("strainer"))

	_, serve := b.glassServe(&Glass{PoseSwitcher: "serve-highball", Obstacle: "highball-glass"}, "")
	test.That(t, serve, test.ShouldContain, AllowedCollision{Frame1: heldObjectFrameName, Frame2: "highball-glass"})
	test.That(t, serve, test.ShouldNotContain, ear("highball-glass"))

	pose := b.poseAllowedCollisions(&poseData{allowedObstacles: []string{"ice-maker", "table"}})
	test.That(t, pose, test.ShouldContain, AllowedCollision{Frame1: heldObjectFrameName, Frame2: "ice-maker"})
	test.That(t, pose, test.ShouldContain, ear("ice-maker"))
	test.That(t, pose, test.ShouldContain, ear("table"))

	clamped := b.armAndGripperVsHeldAllowed()
	test.That(t, clamped, test.ShouldContain, AllowedCollision{Frame1: heldObjectFrameName, Frame2: heldShakerAttachmentFrameName})
	test.That(t, clamped, test.ShouldNotContain, AllowedCollision{Frame1: "gripper:claws", Frame2: heldShakerAttachmentFrameName})
	test.That(t, allowedPairs(b.withAllowedCollisions(nil, b.mixerProtectorAllowed())), test.ShouldContain, ear("mixer-protector"))

	b.detachHeld()
	test.That(t, b.heldShakerAttachmentFrame, test.ShouldBeNil)
	b.heldGeomFrame = referenceframe.NewZeroStaticFrame(heldObjectFrameName)
	test.That(t, allowedPairs(b.withAllowedCollisions(nil, b.mixerProtectorAllowed())), test.ShouldNotContain, ear("mixer-protector"))
}
