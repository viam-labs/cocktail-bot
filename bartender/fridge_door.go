package bartender

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/components/arm"
	toggleswitch "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	poseFridgeClosedHover   = "closed-hover"
	poseFridgeClosedGrab    = "closed-grab"
	poseFridgeClosedRetreat = "closed-retreat"
	poseFridgeOpenHover     = "open-hover"
	poseFridgeOpenGrab      = "open-grab"
	poseFridgeOpenRetreat   = "open-retreat"

	defaultFridgeSwingStepDegs = 10.0
	// Under armplanning's 10 mm / 10° cut-off, so each step stays a straight line rather than a
	// free-form cBiRTT path while the gripper is clamped on the handle. A 10° step's chord sags
	// about 2 mm off the arc at a 500 mm handle radius.
	fridgeSwingLineToleranceMm          = 9.0
	fridgeSwingOrientationToleranceDegs = 9.0
	// How far open-grab may sit from where closed-grab lands when swung about the hinge.
	fridgeGrabMismatchMm   = 15.0
	fridgeGrabMismatchDegs = 10.0
)

// Fridge is the fridge door the arm swings open by its handle to reach bottles inside.
type Fridge struct {
	// Switch holding the closed-* and open-* handle poses.
	DoorSwitchName string `json:"door_switch_name"`
	// Obstacle modeling the door panel shut. While the door stands open it is swung about the hinge.
	DoorObstacle string `json:"door_obstacle"`
	// Vertical hinge axis, world mm.
	Hinge         *FridgeHinge `json:"hinge"`
	SwingStepDegs float64      `json:"swing_step_degs,omitempty"`
	// Bottle slots inside the fridge, from slot_names. Picking one up opens the door first.
	Slots []string `json:"slots,omitempty"`
	// Arm default when unset.
	SwingVelDegsPerSec  float64 `json:"swing_vel_degs_per_sec,omitempty"`
	SwingAccDegsPerSec2 float64 `json:"swing_acc_degs_per_sec2,omitempty"`
}

type FridgeHinge struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (f *Fridge) validate(path string, switchers, slots map[string]struct{}) error {
	if f.DoorSwitchName == "" {
		return errors.New(path + ".door_switch_name is required")
	}
	if _, ok := switchers[f.DoorSwitchName]; !ok {
		return fmt.Errorf("%s.door_switch_name %q is not in pose_switcher_names", path, f.DoorSwitchName)
	}
	if f.DoorObstacle == "" {
		return errors.New(path + ".door_obstacle is required")
	}
	if f.Hinge == nil {
		return errors.New(path + ".hinge is required")
	}
	if f.SwingStepDegs < 0 {
		return errors.New(path + ".swing_step_degs must be > 0 if set")
	}
	if f.SwingVelDegsPerSec < 0 {
		return errors.New(path + ".swing_vel_degs_per_sec must be > 0 if set")
	}
	if f.SwingAccDegsPerSec2 < 0 {
		return errors.New(path + ".swing_acc_degs_per_sec2 must be > 0 if set")
	}
	for _, slot := range f.Slots {
		if _, ok := slots[slot]; !ok {
			return fmt.Errorf("%s.slots[%q] is not in slot_names", path, slot)
		}
	}
	return nil
}

func (c *Config) isFridgeSlot(slot string) bool {
	return c.Fridge != nil && slices.Contains(c.Fridge.Slots, slot)
}

func (f *Fridge) swingStepDegs() float64 {
	if f.SwingStepDegs == 0 {
		return defaultFridgeSwingStepDegs
	}
	return f.SwingStepDegs
}

// swingAboutHinge turns p by degs about the hinge axis, moving its position along the arc and
// yawing its orientation with the door.
func (f *Fridge) swingAboutHinge(p spatialmath.Pose, degs float64) spatialmath.Pose {
	hinge := spatialmath.NewPoseFromPoint(r3.Vector{X: f.Hinge.X, Y: f.Hinge.Y})
	rz := spatialmath.NewPoseFromOrientation(&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: degs})
	return spatialmath.Compose(hinge, spatialmath.Compose(rz, spatialmath.Compose(spatialmath.PoseInverse(hinge), p)))
}

// openDegs is how far the door turns between closed-grab and open-grab, signed like a yaw about
// world +Z. It errors when open-grab is not where closed-grab lands when swung that far, since the
// gripper holds the handle rigidly through the swing.
func (f *Fridge) openDegs(closedGrab, openGrab spatialmath.Pose) (float64, error) {
	c := closedGrab.Point().Sub(r3.Vector{X: f.Hinge.X, Y: f.Hinge.Y})
	o := openGrab.Point().Sub(r3.Vector{X: f.Hinge.X, Y: f.Hinge.Y})
	degs := (math.Atan2(o.Y, o.X) - math.Atan2(c.Y, c.X)) * 180 / math.Pi
	if degs > 180 {
		degs -= 360
	} else if degs <= -180 {
		degs += 360
	}
	if math.Abs(degs) < 1 {
		return 0, fmt.Errorf("%s and %s are the same angle about the hinge; the door would not move", poseFridgeClosedGrab, poseFridgeOpenGrab)
	}
	swung := f.swingAboutHinge(closedGrab, degs)
	if d := swung.Point().Sub(openGrab.Point()).Norm(); d > fridgeGrabMismatchMm {
		return 0, fmt.Errorf("%s is %.0f mm from where %s lands swung %.0f° about the hinge (max %.0f); check hinge and both grab poses",
			poseFridgeOpenGrab, d, poseFridgeClosedGrab, degs, fridgeGrabMismatchMm)
	}
	rot := spatialmath.OrientationBetween(swung.Orientation(), openGrab.Orientation()).AxisAngles().Theta * 180 / math.Pi
	if rot > fridgeGrabMismatchDegs {
		return 0, fmt.Errorf("%s is turned %.0f° from %s swung %.0f° about the hinge (max %.0f); the gripper must hold the handle the same way in both",
			poseFridgeOpenGrab, rot, poseFridgeClosedGrab, degs, fridgeGrabMismatchDegs)
	}
	return degs, nil
}

// swingDoorPart returns parts with door's frame swung degs about the hinge. The door must hang
// off world so the hinge, given in world, applies to its pose directly.
func (f *Fridge) swingDoorPart(parts []*referenceframe.FrameSystemPart, degs float64) ([]*referenceframe.FrameSystemPart, error) {
	out := make([]*referenceframe.FrameSystemPart, len(parts))
	copy(out, parts)
	for i, p := range out {
		lif := p.FrameConfig
		if lif.Name() != f.DoorObstacle {
			continue
		}
		if lif.Parent() != referenceframe.World {
			return nil, fmt.Errorf("fridge door obstacle %q must be parented to world, not %q", f.DoorObstacle, lif.Parent())
		}
		swung := referenceframe.NewLinkInFrame(lif.Parent(), f.swingAboutHinge(lif.Pose(), degs), lif.Name(), lif.Geometry())
		swung.SetUUID(lif.UUID())
		swung.SetMetadata(lif.Metadata())
		out[i] = &referenceframe.FrameSystemPart{FrameConfig: swung, ModelFrame: p.ModelFrame}
		return out, nil
	}
	return nil, fmt.Errorf("fridge door obstacle %q is not in the frame system", f.DoorObstacle)
}

var errFridgeNotConfigured = errors.New("fridge is not configured on the bartender")

// errFridgeDoorOpen means the door is recorded standing open outside a fridge flow, e.g. after a
// failed swing, so the arm cannot know where the door really is.
var errFridgeDoorOpen = errors.New("fridge door is recorded open. shut it by hand and press Cancel on the stream deck")

func (b *bartender) fridgeDoorOpenDegs() float64 {
	if b.dataStore == nil {
		return 0
	}
	return b.dataStore.FridgeDoorOpenDegs()
}

func (b *bartender) checkFridgeDoorShut() error {
	if b.cfg.Fridge != nil && b.fridgeDoorOpenDegs() != 0 {
		return errFridgeDoorOpen
	}
	return nil
}

// markFridgeDoorShut records that someone shut the door by hand.
func (b *bartender) markFridgeDoorShut() {
	if b.dataStore == nil || b.fridgeDoorOpenDegs() == 0 {
		return
	}
	if err := b.dataStore.SetFridgeDoorOpenDegs(0); err != nil {
		b.logger.Errorw("record fridge door shut; a restart may still think it is open", "err", err)
	}
}

// fridgeDoorAllowed lets the gripper touch the door obstacle, whose handle it grips.
func (b *bartender) fridgeDoorAllowed() []AllowedCollision {
	return []AllowedCollision{
		{Frame1: "claws-middle", Frame2: b.cfg.Fridge.DoorObstacle},
		{Frame1: b.cfg.GripperName + ":claws", Frame2: b.cfg.Fridge.DoorObstacle},
		{Frame1: b.cfg.GripperName + ":case-gripper", Frame2: b.cfg.Fridge.DoorObstacle},
	}
}

func (b *bartender) fridgeDoorSwitch() (toggleswitch.Switch, error) {
	if b.cfg.Fridge == nil {
		return nil, errFridgeNotConfigured
	}
	if b.dataStore == nil {
		return nil, fmt.Errorf("fridge door: %w; it is needed to remember whether the door is open", errDataStoreNotConfigured)
	}
	if b.heldGeomFrame != nil {
		return nil, errors.New("fridge door: gripper must be empty to grab the handle")
	}
	return b.findSwitch(b.cfg.Fridge.DoorSwitchName)
}

// openFridgeDoor grabs the handle of the shut door, swings the door open to open-grab and leaves it
// standing open, with its obstacle swung to match, until closeFridgeDoor.
func (b *bartender) openFridgeDoor(ctx context.Context) error {
	sw, err := b.fridgeDoorSwitch()
	if err != nil {
		return err
	}
	if degs := b.fridgeDoorOpenDegs(); degs != 0 {
		return fmt.Errorf("open fridge door: door is already recorded open at %.0f°", degs)
	}
	allow := b.fridgeDoorAllowed()
	if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseFridgeClosedHover); err != nil {
		return fmt.Errorf("fridge %s: %w", poseFridgeClosedHover, err)
	}
	if err := b.gripperOpen(ctx); err != nil {
		return fmt.Errorf("open gripper before grabbing fridge handle: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseFridgeClosedGrab, allow...); err != nil {
		return fmt.Errorf("linear-to fridge %s: %w", poseFridgeClosedGrab, err)
	}
	if _, err := b.gripper.Grab(ctx, nil); err != nil {
		return fmt.Errorf("close gripper on fridge handle: %w", err)
	}
	if err := b.swingFridgeDoor(ctx, sw, true); err != nil {
		return fmt.Errorf("swing fridge door open: %w", err)
	}
	if err := b.gripperOpen(ctx); err != nil {
		return fmt.Errorf("release fridge handle: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseFridgeOpenRetreat, allow...); err != nil {
		return fmt.Errorf("linear-retreat to fridge %s: %w", poseFridgeOpenRetreat, err)
	}
	return nil
}

// closeFridgeDoor grabs the handle of the open door and swings it back shut.
func (b *bartender) closeFridgeDoor(ctx context.Context) error {
	sw, err := b.fridgeDoorSwitch()
	if err != nil {
		return err
	}
	if b.fridgeDoorOpenDegs() == 0 {
		return errors.New("close fridge door: door is already recorded shut")
	}
	allow := b.fridgeDoorAllowed()
	if _, err := b.moveArmToPoseOnSwitch(ctx, sw, poseFridgeOpenHover); err != nil {
		return fmt.Errorf("fridge %s: %w", poseFridgeOpenHover, err)
	}
	if err := b.gripperOpen(ctx); err != nil {
		return fmt.Errorf("open gripper before grabbing fridge handle: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseFridgeOpenGrab, allow...); err != nil {
		return fmt.Errorf("linear-to fridge %s: %w", poseFridgeOpenGrab, err)
	}
	if _, err := b.gripper.Grab(ctx, nil); err != nil {
		return fmt.Errorf("close gripper on fridge handle: %w", err)
	}
	if err := b.swingFridgeDoor(ctx, sw, false); err != nil {
		return fmt.Errorf("swing fridge door shut: %w", err)
	}
	if err := b.gripperOpen(ctx); err != nil {
		return fmt.Errorf("release fridge handle: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sw, poseFridgeClosedRetreat, allow...); err != nil {
		return fmt.Errorf("linear-retreat to fridge %s: %w", poseFridgeClosedRetreat, err)
	}
	return nil
}

// swingFridgeDoor carries the gripped handle along its arc about the hinge, open or shut. Each step
// is planned against the door obstacle at that step's angle, then all steps run as one arm move so
// the swing does not stop at every step. The door is recorded open before an opening swing moves
// and shut only once a closing swing finishes, so a swing cut short leaves it recorded open.
func (b *bartender) swingFridgeDoor(ctx context.Context, sw toggleswitch.Switch, open bool) error {
	f := b.cfg.Fridge
	closedGrab, err := b.fridgeGrabInWorld(ctx, sw, poseFridgeClosedGrab)
	if err != nil {
		return err
	}
	openGrab, err := b.fridgeGrabInWorld(ctx, sw, poseFridgeOpenGrab)
	if err != nil {
		return err
	}
	openDegs, err := f.openDegs(closedGrab.pose, openGrab.pose)
	if err != nil {
		return err
	}
	from, to, label := 0.0, openDegs, "open"
	if !open {
		from, to, label = b.fridgeDoorOpenDegs(), 0, "shut"
	}
	positions, err := b.planFridgeSwing(ctx, closedGrab, from, to, sw.Name().ShortName()+":swing-"+label)
	if err != nil {
		return err
	}
	if open {
		if err := b.dataStore.SetFridgeDoorOpenDegs(openDegs); err != nil {
			return fmt.Errorf("record fridge door open: %w", err)
		}
	}
	if err := b.arm.MoveThroughJointPositions(ctx, positions, b.swingMoveOptions(), nil); err != nil {
		return fmt.Errorf("execute swing %s: %w", label, err)
	}
	if !open {
		if err := b.dataStore.SetFridgeDoorOpenDegs(0); err != nil {
			return fmt.Errorf("record fridge door shut: %w", err)
		}
	}
	return nil
}

func (b *bartender) fridgeGrabInWorld(ctx context.Context, sw toggleswitch.Switch, poseName string) (*poseData, error) {
	pd, err := fetchPose(ctx, sw, poseName)
	if err != nil {
		return nil, err
	}
	if pd, err = b.cartesianPose(ctx, pd); err != nil {
		return nil, err
	}
	return b.poseInWorld(ctx, pd)
}

// planFridgeSwing plans the handle from fromDegs to toDegs in steps of at most swing_step_degs,
// starting from the arm's current joints, and returns the joined joint trajectory.
func (b *bartender) planFridgeSwing(ctx context.Context, closedGrab *poseData, fromDegs, toDegs float64, label string) ([][]referenceframe.Input, error) {
	f := b.cfg.Fridge
	componentName := closedGrab.componentName
	if componentName == "" {
		componentName = b.cfg.ArmName
	}
	// The epsilon keeps float noise in openDegs, e.g. 30.0000001, from adding a sliver of a step.
	steps := max(1, int(math.Ceil(math.Abs(toDegs-fromDegs)/f.swingStepDegs()-1e-6)))
	constraints := &motionplan.Constraints{
		LinearConstraint: []motionplan.LinearConstraint{{
			LineToleranceMm:          fridgeSwingLineToleranceMm,
			OrientationToleranceDegs: fridgeSwingOrientationToleranceDegs,
		}},
	}
	constraints = b.withAllowedCollisions(constraints, b.fridgeDoorAllowed())

	var positions [][]referenceframe.Input
	var armInputs []referenceframe.Input
	for i := 1; i <= steps; i++ {
		degs := fromDegs + (toDegs-fromDegs)*float64(i)/float64(steps)
		fs, fsInputs, err := b.inputsWithFridgeDoorAt(ctx, degs)
		if err != nil {
			return nil, err
		}
		if armInputs != nil {
			fsInputs[b.cfg.ArmName] = armInputs
		}
		goal := armplanning.NewPlanState(referenceframe.FrameSystemPoses{
			componentName: referenceframe.NewPoseInFrame(referenceframe.World, f.swingAboutHinge(closedGrab.pose, degs)),
		}, nil)
		plan, err := b.planMotion(ctx, &armplanning.PlanRequest{
			FrameSystem: fs,
			StartState:  armplanning.NewPlanState(nil, fsInputs),
			Goals:       []*armplanning.PlanState{goal},
			Constraints: constraints,
		}, fmt.Sprintf("%s:%.0f", label, degs))
		if err != nil {
			return nil, err
		}
		segment, err := plan.Trajectory().GetFrameInputs(b.cfg.ArmName)
		if err != nil {
			return nil, fmt.Errorf("extract trajectory: %w", err)
		}
		if positions != nil {
			segment = segment[1:]
		}
		positions = append(positions, segment...)
		armInputs = positions[len(positions)-1]
	}
	return positions, nil
}

func (b *bartender) swingMoveOptions() *arm.MoveOptions {
	return moveOptionsFromCfg(b.cfg.Fridge.SwingVelDegsPerSec, b.cfg.Fridge.SwingAccDegsPerSec2)
}
