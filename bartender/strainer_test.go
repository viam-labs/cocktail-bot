package bartender

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/test"
)

// strainerFrameSystem has a grip point at gripPose and a strainer obstacle box at strainerPose.
func strainerFrameSystem(t *testing.T, gripPose, strainerPose spatialmath.Pose) *referenceframe.FrameSystem {
	t.Helper()
	fs := referenceframe.NewEmptyFrameSystem("strainer")
	grip, err := referenceframe.NewStaticFrame(gripPointFrameName, gripPose)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(grip, fs.World()), test.ShouldBeNil)
	box, err := spatialmath.NewBox(strainerPose, r3.Vector{X: 200, Y: 125, Z: 50}, "strainer")
	test.That(t, err, test.ShouldBeNil)
	obstacle, err := referenceframe.NewStaticFrameWithGeometry("strainer", spatialmath.NewZeroPose(), box)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(obstacle, fs.World()), test.ShouldBeNil)
	return fs
}

func carriedStrainerInWorld(t *testing.T, fs *referenceframe.FrameSystem, carried referenceframe.Frame) spatialmath.Geometry {
	t.Helper()
	test.That(t, fs.AddFrame(carried, fs.Frame(gripPointFrameName)), test.ShouldBeNil)
	geoms, err := referenceframe.FrameSystemGeometries(fs, referenceframe.NewZeroInputs(fs))
	test.That(t, err, test.ShouldBeNil)
	return geoms[heldStrainerFrameName].Geometries()[0]
}

func TestCarriedStrainerStartsWhereTheObstacleIsAndTwistsWithTheGripper(t *testing.T) {
	grip := spatialmath.NewPose(r3.Vector{X: 300, Y: -200, Z: 250}, &spatialmath.OrientationVectorDegrees{OX: 1, Theta: 30})
	onMount := spatialmath.NewPose(r3.Vector{X: 310, Y: -180, Z: 380}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 15})
	fs := strainerFrameSystem(t, grip, onMount)

	carried, err := carriedObstacleFrame(fs, referenceframe.NewZeroInputs(fs), gripPointFrameName, "strainer", heldStrainerFrameName)
	test.That(t, err, test.ShouldBeNil)

	seated := carriedStrainerInWorld(t, strainerFrameSystem(t, grip, onMount), carried)
	test.That(t, seated.Label(), test.ShouldEqual, heldStrainerFrameName)
	test.That(t, spatialmath.PoseAlmostEqual(seated.Pose(), onMount), test.ShouldBeTrue)

	twist := spatialmath.NewPoseFromOrientation(&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 45})
	twisted := spatialmath.Compose(grip, twist)
	locked := carriedStrainerInWorld(t, strainerFrameSystem(t, twisted, onMount), carried)
	want := spatialmath.Compose(twisted, spatialmath.Compose(spatialmath.PoseInverse(grip), onMount))
	test.That(t, spatialmath.PoseAlmostEqual(locked.Pose(), want), test.ShouldBeTrue)
}

func TestCarriedObstacleFrameNeedsTheObstacle(t *testing.T) {
	fs := strainerFrameSystem(t, spatialmath.NewZeroPose(), spatialmath.NewZeroPose())
	_, err := carriedObstacleFrame(fs, referenceframe.NewZeroInputs(fs), gripPointFrameName, "missing-strainer", heldStrainerFrameName)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "has 0 geometries")
}

func strainerTestBartender(t *testing.T) *bartender {
	t.Helper()
	held, err := buildHeldObjectFrame(&HeldObjectGeometry{Type: "cylinder", RadiusMM: 38, LengthMM: 166}, nil)
	test.That(t, err, test.ShouldBeNil)
	return &bartender{
		cfg: &Config{
			ArmName:                     "arm",
			GripperName:                 "gripper",
			StrainerObstacle:            "strainer",
			StrainerMountObstacle:       "strainer-mount",
			StrainerMountShieldObstacle: "strainer-mount-shield",
		},
		heldGeomFrame: held,
	}
}

func allowedPairs(c *motionplan.Constraints) []AllowedCollision {
	if c == nil {
		return nil
	}
	var out []AllowedCollision
	for _, spec := range c.CollisionSpecification {
		for _, a := range spec.Allows {
			out = append(out, AllowedCollision{Frame1: a.Frame1, Frame2: a.Frame2})
		}
	}
	return out
}

func TestMountedStrainerIsLeftOutOfPlanningOnlyWhileCarried(t *testing.T) {
	b := strainerTestBartender(t)
	onMount := AllowedCollision{Frame1: heldObjectFrameName, Frame2: "strainer"}
	carriedVsShield := AllowedCollision{Frame1: heldStrainerFrameName, Frame2: "strainer-mount-shield"}
	seat := append(b.strainerShieldAllowed(), b.strainerOnMountAllowed()...)

	test.That(t, b.absentObstacles()["strainer"], test.ShouldBeFalse)
	pairs := allowedPairs(b.withAllowedCollisions(nil, seat))
	test.That(t, pairs, test.ShouldContain, onMount)
	test.That(t, pairs, test.ShouldNotContain, carriedVsShield)

	b.heldStrainerFrame = referenceframe.NewZeroStaticFrame(heldStrainerFrameName)
	test.That(t, b.absentObstacles()["strainer"], test.ShouldBeTrue)
	pairs = allowedPairs(b.withAllowedCollisions(nil, seat))
	test.That(t, pairs, test.ShouldNotContain, onMount)
	test.That(t, pairs, test.ShouldContain, carriedVsShield)
	test.That(t, b.carriedStrainerVsMountAllowed(), test.ShouldResemble,
		[]AllowedCollision{{Frame1: heldStrainerFrameName, Frame2: "strainer-mount"}})
}

func TestCarriedStrainerIsClampedWithTheShaker(t *testing.T) {
	b := strainerTestBartender(t)
	test.That(t, b.armAndGripperVsHeldAllowed(), test.ShouldNotContain,
		AllowedCollision{Frame1: heldObjectFrameName, Frame2: heldStrainerFrameName})

	b.heldStrainerFrame = referenceframe.NewZeroStaticFrame(heldStrainerFrameName)
	allowed := b.armAndGripperVsHeldAllowed()
	test.That(t, allowed, test.ShouldContain, AllowedCollision{Frame1: heldObjectFrameName, Frame2: heldStrainerFrameName})
	test.That(t, allowed, test.ShouldContain, AllowedCollision{Frame1: "gripper:claws", Frame2: heldStrainerFrameName})
	test.That(t, allowed, test.ShouldContain, AllowedCollision{Frame1: "gripper:claws", Frame2: heldObjectFrameName})
	test.That(t, b.heldVsObstacleAllowed("highball-glass"), test.ShouldContain,
		AllowedCollision{Frame1: heldStrainerFrameName, Frame2: "highball-glass"})

	b.detachHeld()
	test.That(t, b.heldStrainerFrame, test.ShouldBeNil)
	test.That(t, b.absentObstacles()["strainer"], test.ShouldBeFalse)
}

func TestValidateStepsStrain(t *testing.T) {
	b := &bartender{cfg: &Config{StrainerObstacle: "strainer"}}
	ice, shake, strain := RecipeStep{Kind: "ice"}, RecipeStep{Kind: "mix", MixMode: "shake"}, RecipeStep{Kind: "strain"}

	test.That(t, b.validateSteps([]RecipeStep{ice, shake, strain}), test.ShouldBeNil)

	err := b.validateSteps([]RecipeStep{ice, strain, shake})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "step 3 mix: must come before the strain step")

	b.cfg.StrainerObstacle = ""
	err = b.validateSteps([]RecipeStep{ice, shake, strain})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "strainer_obstacle is not configured")
}

func TestParseStrainFlow(t *testing.T) {
	b := &bartender{cfg: &Config{RecipeDefaults: &RecipeDefaults{StrainFlow: "strain-flow"}}}
	flow, err := b.parseStrainFlow("attach_strainer", map[string]any{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, flow, test.ShouldEqual, "strain-flow")

	flow, err = b.parseStrainFlow("attach_strainer", map[string]any{"strain_flow": "other-flow"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, flow, test.ShouldEqual, "other-flow")

	_, err = b.parseStrainFlow("attach_strainer", "not-a-map")
	test.That(t, err, test.ShouldNotBeNil)

	b.cfg.RecipeDefaults = nil
	_, err = b.parseStrainFlow("return_strainer", map[string]any{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "return_strainer: 'strain_flow' is required")
}

func omit(m map[string]any, key string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if k != key {
			out[k] = v
		}
	}
	return out
}

func override(m map[string]any, key string, value any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	out[key] = value
	return out
}
