package bartender

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/components/arm"
	toggleswitch "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

type ctxKey int

const orderIDKey ctxKey = 1

func ctxWithOrderID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, orderIDKey, id)
}

func orderIDFromCtx(ctx context.Context) string {
	id, _ := ctx.Value(orderIDKey).(string)
	return id
}

type poseData struct {
	pose          spatialmath.Pose
	refFrame      string
	componentName string
}

// DoCommand contract on the switcher: get_pose_by_name → {x, y, z, o_x, o_y, o_z, theta, reference_frame, component_name}.
func fetchPose(ctx context.Context, sw toggleswitch.Switch, poseName string) (*poseData, error) {
	resp, err := sw.DoCommand(ctx, map[string]any{"get_pose_by_name": poseName})
	if err != nil {
		return nil, fmt.Errorf("get pose %q from %q: %w", poseName, sw.Name().ShortName(), err)
	}
	x, _ := resp["x"].(float64)
	y, _ := resp["y"].(float64)
	z, _ := resp["z"].(float64)
	oX, _ := resp["o_x"].(float64)
	oY, _ := resp["o_y"].(float64)
	oZ, _ := resp["o_z"].(float64)
	theta, _ := resp["theta"].(float64)
	refFrame, _ := resp["reference_frame"].(string)
	if refFrame == "" {
		refFrame = referenceframe.World
	}
	componentName, _ := resp["component_name"].(string)
	return &poseData{
		pose: spatialmath.NewPose(
			r3.Vector{X: x, Y: y, Z: z},
			&spatialmath.OrientationVectorDegrees{OX: oX, OY: oY, OZ: oZ, Theta: theta},
		),
		refFrame:      refFrame,
		componentName: componentName,
	}, nil
}

func (b *bartender) resolvePose(ctx context.Context, poseName string) (*poseData, toggleswitch.Switch, error) {
	var lastErr error
	for _, sw := range b.poseSwitches {
		pd, err := fetchPose(ctx, sw, poseName)
		if err == nil {
			return pd, sw, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("no pose switchers configured")
	}
	return nil, nil, fmt.Errorf("pose %q not found on any switcher: %w", poseName, lastErr)
}

func (b *bartender) findSwitch(name string) (toggleswitch.Switch, error) {
	for _, sw := range b.poseSwitches {
		if sw.Name().ShortName() == name {
			return sw, nil
		}
	}
	return nil, fmt.Errorf("switch %q not in pose_switcher_names", name)
}

func (b *bartender) currentInputs(ctx context.Context) (*referenceframe.FrameSystem, referenceframe.FrameSystemInputs, error) {
	fsCfg, err := b.fsSvc.FrameSystemConfig(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("get frame system config: %w", err)
	}
	fs, err := referenceframe.NewFrameSystem("bartender", fsCfg.Parts, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("build frame system: %w", err)
	}
	if b.heldGeomFrame != nil {
		if err := fs.AddFrame(b.heldGeomFrame, fs.Frame(b.cfg.GripperName)); err != nil {
			return nil, nil, fmt.Errorf("attach held geometry frame: %w", err)
		}
	}
	armInputs, err := b.arm.CurrentInputs(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("get arm inputs: %w", err)
	}
	fsInputs := referenceframe.NewZeroInputs(fs)
	fsInputs[b.cfg.ArmName] = armInputs
	return fs, fsInputs, nil
}

// Every move the service makes routes through here — keep it the only call site
// of armplanning.PlanMotion so instrumentation (saves, logs, timeouts) stays uniform.
func (b *bartender) planMotion(ctx context.Context, req *armplanning.PlanRequest, label string) (motionplan.Plan, error) {
	start := time.Now()
	plan, _, err := armplanning.PlanMotion(ctx, b.logger, req)
	duration := time.Since(start).Round(time.Millisecond)
	b.savePlan(ctx, req, plan, label)
	if err != nil {
		return nil, fmt.Errorf("plan %s (%s): %w", label, duration, err)
	}
	b.logger.Infof("planned %s in %s", label, duration)
	return plan, nil
}

func (b *bartender) savePlan(ctx context.Context, req *armplanning.PlanRequest, plan motionplan.Plan, label string) {
	orderID := orderIDFromCtx(ctx)
	if orderID == "" {
		return
	}
	b.filesaver.SaveAsync(ctx, orderID, fmt.Sprintf("%s_move.json", label), func(path string) error {
		return req.WriteRequestAndResponseToFile(path, plan)
	})
}

func (b *bartender) moveToResolvedPose(ctx context.Context, pd *poseData, label string, constraints *motionplan.Constraints, opts *arm.MoveOptions) (time.Duration, error) {
	start := time.Now()
	fs, fsInputs, err := b.currentInputs(ctx)
	if err != nil {
		return 0, err
	}
	goalPIF := referenceframe.NewPoseInFrame(pd.refFrame, pd.pose)
	worldTF, err := fs.Transform(fsInputs.ToLinearInputs(), goalPIF, referenceframe.World)
	if err != nil {
		return 0, fmt.Errorf("transform goal to world: %w", err)
	}
	componentName := pd.componentName
	if componentName == "" {
		componentName = b.cfg.ArmName
	}
	goal := armplanning.NewPlanState(
		referenceframe.FrameSystemPoses{componentName: worldTF.(*referenceframe.PoseInFrame)},
		nil,
	)
	req := &armplanning.PlanRequest{
		FrameSystem: fs,
		StartState:  armplanning.NewPlanState(nil, fsInputs),
		Goals:       []*armplanning.PlanState{goal},
		Constraints: constraints,
	}
	plan, err := b.planMotion(ctx, req, label)
	if err != nil {
		return 0, err
	}
	positions, err := plan.Trajectory().GetFrameInputs(b.cfg.ArmName)
	if err != nil {
		return 0, fmt.Errorf("extract trajectory: %w", err)
	}
	if err := b.arm.MoveThroughJointPositions(ctx, positions, opts, nil); err != nil {
		return 0, fmt.Errorf("execute %s: %w", label, err)
	}
	return time.Since(start).Round(time.Millisecond), nil
}

func (b *bartender) moveArmToPose(ctx context.Context, poseName string) (time.Duration, error) {
	pd, _, err := b.resolvePose(ctx, poseName)
	if err != nil {
		return 0, err
	}
	return b.moveToResolvedPose(ctx, pd, poseName, nil, nil)
}

func (b *bartender) moveArmToPoseWithOpts(ctx context.Context, poseName string, opts *arm.MoveOptions) (time.Duration, error) {
	pd, _, err := b.resolvePose(ctx, poseName)
	if err != nil {
		return 0, err
	}
	return b.moveToResolvedPose(ctx, pd, poseName, nil, opts)
}

func (b *bartender) moveArmToPoseOnSwitch(ctx context.Context, sw toggleswitch.Switch, poseName string) (time.Duration, error) {
	pd, err := fetchPose(ctx, sw, poseName)
	if err != nil {
		return 0, err
	}
	label := sw.Name().ShortName() + ":" + poseName
	return b.moveToResolvedPose(ctx, pd, label, nil, nil)
}

func (b *bartender) moveArmToPoseOnSwitchWithOpts(ctx context.Context, sw toggleswitch.Switch, poseName string, opts *arm.MoveOptions) (time.Duration, error) {
	pd, err := fetchPose(ctx, sw, poseName)
	if err != nil {
		return 0, err
	}
	label := sw.Name().ShortName() + ":" + poseName
	return b.moveToResolvedPose(ctx, pd, label, nil, opts)
}

// Straight-line through space; the empty-handed grab/release paths that risk
// knocking the bottle laterally. For moves WITH a bottle in hand, use
// linearCarryToPose so the no-spill orientation constraint is applied too.
func (b *bartender) linearMoveToPose(ctx context.Context, sw toggleswitch.Switch, poseName string) (time.Duration, error) {
	pd, err := fetchPose(ctx, sw, poseName)
	if err != nil {
		return 0, err
	}
	label := sw.Name().ShortName() + ":" + poseName + ":linear"
	constraints := &motionplan.Constraints{
		LinearConstraint: []motionplan.LinearConstraint{{LineToleranceMm: 1, OrientationToleranceDegs: 5}},
	}
	return b.moveToResolvedPose(ctx, pd, label, constraints, nil)
}

const noSpillOrientationToleranceDegs = 15

// Straight-line + no-spill constraint. For lifting/lowering a bottle held in the
// gripper: linear so the planner doesn't swing the bottle sideways, orientation
// so the planner doesn't rotate the gripper mid-ascent and tip the bottle.
func (b *bartender) linearCarryToPose(ctx context.Context, sw toggleswitch.Switch, poseName string) (time.Duration, error) {
	pd, err := fetchPose(ctx, sw, poseName)
	if err != nil {
		return 0, err
	}
	label := sw.Name().ShortName() + ":" + poseName + ":linear-carry"
	constraints := &motionplan.Constraints{
		LinearConstraint: []motionplan.LinearConstraint{{LineToleranceMm: 1, OrientationToleranceDegs: 5}},
		OrientationConstraint: []motionplan.OrientationConstraint{
			{OrientationToleranceDegs: noSpillOrientationToleranceDegs, IgnoreTheta: true},
		},
	}
	return b.moveHeldToResolvedPose(ctx, pd, label, constraints, nil)
}

// Carry = plan-wide orientation constraint on the held object's long axis, so
// the shaker stays within ~15° of upright for the whole move and doesn't slosh.
// IgnoreTheta so rotation about the shaker's own vertical axis is still free.
func (b *bartender) carryHeldLevel(ctx context.Context, sw toggleswitch.Switch, poseName string) (time.Duration, error) {
	pd, err := fetchPose(ctx, sw, poseName)
	if err != nil {
		return 0, err
	}
	return b.carryHeldLevelToResolved(ctx, pd, sw.Name().ShortName()+":"+poseName+":carry")
}

func (b *bartender) carryHeldLevelToResolved(ctx context.Context, pd *poseData, label string) (time.Duration, error) {
	constraints := &motionplan.Constraints{
		OrientationConstraint: []motionplan.OrientationConstraint{
			{OrientationToleranceDegs: noSpillOrientationToleranceDegs, IgnoreTheta: true},
		},
	}
	return b.moveHeldToResolvedPose(ctx, pd, label, constraints, nil)
}

// moveHeldToResolvedPose plans with the held-object frame as the goal, so any
// orientation constraint bounds the shaker's own long axis rather than the
// gripper's tool axis. Falls back to moveToResolvedPose when nothing is held.
func (b *bartender) moveHeldToResolvedPose(ctx context.Context, pd *poseData, label string, constraints *motionplan.Constraints, opts *arm.MoveOptions) (time.Duration, error) {
	if b.heldGeomFrame == nil {
		return b.moveToResolvedPose(ctx, pd, label, constraints, opts)
	}
	start := time.Now()
	fs, fsInputs, err := b.currentInputs(ctx)
	if err != nil {
		return 0, err
	}
	linearInputs := fsInputs.ToLinearInputs()

	destTF, err := fs.Transform(linearInputs, referenceframe.NewPoseInFrame(pd.refFrame, pd.pose), referenceframe.World)
	if err != nil {
		return 0, fmt.Errorf("transform carry destination to world: %w", err)
	}
	destPose := destTF.(*referenceframe.PoseInFrame).Pose()

	// Where held-object sits relative to the destination's component frame; constant
	// because both hang rigidly off the gripper.
	destComponent := pd.componentName
	if destComponent == "" {
		destComponent = b.cfg.ArmName
	}
	offTF, err := fs.Transform(linearInputs, referenceframe.NewPoseInFrame(heldObjectFrameName, spatialmath.NewZeroPose()), destComponent)
	if err != nil {
		return 0, fmt.Errorf("transform %q into %q: %w", heldObjectFrameName, destComponent, err)
	}
	heldGoalPose := spatialmath.Compose(destPose, offTF.(*referenceframe.PoseInFrame).Pose())

	goal := armplanning.NewPlanState(
		referenceframe.FrameSystemPoses{
			heldObjectFrameName: referenceframe.NewPoseInFrame(referenceframe.World, heldGoalPose),
		},
		nil,
	)
	req := &armplanning.PlanRequest{
		FrameSystem: fs,
		StartState:  armplanning.NewPlanState(nil, fsInputs),
		Goals:       []*armplanning.PlanState{goal},
		Constraints: constraints,
	}
	plan, err := b.planMotion(ctx, req, label)
	if err != nil {
		return 0, err
	}
	positions, err := plan.Trajectory().GetFrameInputs(b.cfg.ArmName)
	if err != nil {
		return 0, fmt.Errorf("extract trajectory: %w", err)
	}
	if err := b.arm.MoveThroughJointPositions(ctx, positions, opts, nil); err != nil {
		return 0, fmt.Errorf("execute %s: %w", label, err)
	}
	return time.Since(start).Round(time.Millisecond), nil
}

// Saved poses may be expressed in any frame; shifting them by a world offset needs them in world first.
func (b *bartender) poseInWorld(ctx context.Context, pd *poseData) (*poseData, error) {
	fs, fsInputs, err := b.currentInputs(ctx)
	if err != nil {
		return nil, err
	}
	tf, err := fs.Transform(fsInputs.ToLinearInputs(), referenceframe.NewPoseInFrame(pd.refFrame, pd.pose), referenceframe.World)
	if err != nil {
		return nil, fmt.Errorf("transform pose to world: %w", err)
	}
	return &poseData{
		pose:          tf.(*referenceframe.PoseInFrame).Pose(),
		refFrame:      referenceframe.World,
		componentName: pd.componentName,
	}, nil
}

func moveOptionsFromCfg(velDegsPerSec, accDegsPerSec2 float64) *arm.MoveOptions {
	if velDegsPerSec == 0 && accDegsPerSec2 == 0 {
		return nil
	}
	return &arm.MoveOptions{
		MaxVelRads: velDegsPerSec * math.Pi / 180.0,
		MaxAccRads: accDegsPerSec2 * math.Pi / 180.0,
	}
}

func (b *bartender) pourMoveOptions() *arm.MoveOptions {
	return moveOptionsFromCfg(b.cfg.PourVelDegsPerSec, b.cfg.PourAccDegsPerSec2)
}

func (b *bartender) serveMoveOptions() *arm.MoveOptions {
	return moveOptionsFromCfg(b.cfg.ServeVelDegsPerSec, b.cfg.ServeAccDegsPerSec2)
}
