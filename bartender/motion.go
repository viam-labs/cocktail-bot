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
	pose                            spatialmath.Pose
	refFrame                        string
	componentName                   string
	velDegsPerSec                   float64
	accDegsPerSec2                  float64
	lineToleranceMm                 float64
	orientationToleranceDegs        float64
	noSpillOrientationToleranceDegs float64
	allowedObstacles                []string
	circleRadiusMm                  float64
	circlePointsPerRev              int
}

const (
	defaultLineToleranceMm                 = 1.0
	defaultLinearOrientationToleranceDegs  = 5.0
	defaultNoSpillOrientationToleranceDegs = 15.0
	defaultCircleRadiusMm                  = 10.0
	defaultCirclePointsPerRev              = 8
)

func (pd *poseData) linearLineToleranceMm() float64 {
	if pd.lineToleranceMm > 0 {
		return pd.lineToleranceMm
	}
	return defaultLineToleranceMm
}

func (pd *poseData) linearOrientationToleranceDegs() float64 {
	if pd.orientationToleranceDegs > 0 {
		return pd.orientationToleranceDegs
	}
	return defaultLinearOrientationToleranceDegs
}

func (pd *poseData) noSpillToleranceDegs() float64 {
	if pd.noSpillOrientationToleranceDegs > 0 {
		return pd.noSpillOrientationToleranceDegs
	}
	return defaultNoSpillOrientationToleranceDegs
}

func (pd *poseData) circleRadius() float64 {
	if pd.circleRadiusMm > 0 {
		return pd.circleRadiusMm
	}
	return defaultCircleRadiusMm
}

func (pd *poseData) circlePoints() int {
	if pd.circlePointsPerRev > 0 {
		return pd.circlePointsPerRev
	}
	return defaultCirclePointsPerRev
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
	vel, _ := resp["vel_degs_per_sec"].(float64)
	acc, _ := resp["acc_degs_per_sec2"].(float64)
	lineTol, _ := resp["line_tolerance_mm"].(float64)
	oriTol, _ := resp["orientation_tolerance_degs"].(float64)
	noSpillTol, _ := resp["no_spill_orientation_tolerance_degs"].(float64)
	circleRadius, _ := resp["circle_radius_mm"].(float64)
	var circlePoints int
	switch v := resp["circle_points_per_rev"].(type) {
	case float64:
		circlePoints = int(v)
	case int:
		circlePoints = v
	}
	var allowedObstacles []string
	if raw, ok := resp["allowed_obstacles"].([]any); ok {
		for _, v := range raw {
			if s, ok := v.(string); ok && s != "" {
				allowedObstacles = append(allowedObstacles, s)
			}
		}
	}
	return &poseData{
		pose: spatialmath.NewPose(
			r3.Vector{X: x, Y: y, Z: z},
			&spatialmath.OrientationVectorDegrees{OX: oX, OY: oY, OZ: oZ, Theta: theta},
		),
		refFrame:                        refFrame,
		componentName:                   componentName,
		velDegsPerSec:                   vel,
		accDegsPerSec2:                  acc,
		lineToleranceMm:                 lineTol,
		orientationToleranceDegs:        oriTol,
		noSpillOrientationToleranceDegs: noSpillTol,
		allowedObstacles:                allowedObstacles,
		circleRadiusMm:                  circleRadius,
		circlePointsPerRev:              circlePoints,
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
		parent := fs.Frame(gripPointFrameName)
		if parent == nil {
			parent = fs.Frame(b.cfg.GripperName)
			b.logger.Warnf("no %q frame in config; attaching held object to %q origin instead. The virtual capsule will sit at the gripper mount, not where the claws close, so collision checks will be off by the gripper tool length. Add a grip-point component parented to the gripper to fix.", gripPointFrameName, b.cfg.GripperName)
		}
		if err := fs.AddFrame(b.heldGeomFrame, parent); err != nil {
			return nil, nil, fmt.Errorf("attach held geometry frame to %q: %w", parent.Name(), err)
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
	hasHeld := false
	for _, name := range req.FrameSystem.FrameNames() {
		if name == heldObjectFrameName {
			hasHeld = true
			break
		}
	}
	b.logger.Infof("plan %s: held attached=%v, frames=%v", label, hasHeld, req.FrameSystem.FrameNames())
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
	if opts == nil {
		opts = moveOptionsFromCfg(pd.velDegsPerSec, pd.accDegsPerSec2)
	}
	if poseAllowed := b.poseAllowedCollisions(pd); len(poseAllowed) > 0 {
		constraints = b.withAllowedCollisions(constraints, poseAllowed)
	}
	if heldAllowed := b.armAndGripperVsHeldAllowed(); len(heldAllowed) > 0 {
		constraints = b.withAllowedCollisions(constraints, heldAllowed)
	}
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

func (b *bartender) moveArmToPoseOnSwitch(ctx context.Context, sw toggleswitch.Switch, poseName string, allowed ...AllowedCollision) (time.Duration, error) {
	pd, err := fetchPose(ctx, sw, poseName)
	if err != nil {
		return 0, err
	}
	label := sw.Name().ShortName() + ":" + poseName
	constraints := b.withAllowedCollisions(nil, allowed)
	return b.moveToResolvedPose(ctx, pd, label, constraints, nil)
}

// AllowedCollision names a pair of frames whose collision the planner should
// skip for one move; the two frames can be given in either order. Used by
// pickup/release moves where the gripper is intentionally entering the volume
// of an obstacle that models the object about to be picked up.
type AllowedCollision struct {
	Frame1 string
	Frame2 string
}

// withAllowedCollisions returns constraints with ac added to its collision-spec
// allow list. Nil constraints becomes a fresh one; empty ac is a no-op. Allow
// entries that reference the held-object frame are dropped when nothing is
// attached, since the planner rejects allow entries whose frame doesn't exist
// in the current frame system.
func (b *bartender) withAllowedCollisions(constraints *motionplan.Constraints, ac []AllowedCollision) *motionplan.Constraints {
	filtered := ac
	if b.heldGeomFrame == nil {
		filtered = filtered[:0:0]
		for _, a := range ac {
			if a.Frame1 == heldObjectFrameName || a.Frame2 == heldObjectFrameName {
				continue
			}
			filtered = append(filtered, a)
		}
	}
	if len(filtered) == 0 {
		return constraints
	}
	if constraints == nil {
		constraints = &motionplan.Constraints{}
	}
	allows := make([]motionplan.CollisionSpecificationAllowedFrameCollisions, len(filtered))
	for i, a := range filtered {
		allows[i] = motionplan.CollisionSpecificationAllowedFrameCollisions{Frame1: a.Frame1, Frame2: a.Frame2}
	}
	constraints.CollisionSpecification = append(constraints.CollisionSpecification, motionplan.CollisionSpecification{Allows: allows})
	return constraints
}

// pickupAllowedCollisions returns the frame pairs whose collision should be
// skipped while the gripper is in or around the obstacle modeling the object
// at pickup location `key` (lookup via Config.PickupObstacles). Allows the two
// gripper-attached collision bodies and the held-object frame to overlap the
// obstacle, so the pickup, release, and lift-away moves can plan through what
// would otherwise be the obstacle's volume. Returns nil when no obstacle is
// configured — the move then plans with no override.
func (b *bartender) pickupAllowedCollisions(key string) []AllowedCollision {
	obstacle := b.cfg.PickupObstacles[key]
	if obstacle == "" {
		return nil
	}
	return []AllowedCollision{
		{Frame1: "claws-middle", Frame2: obstacle},
		{Frame1: heldObjectFrameName, Frame2: obstacle},
	}
}

// armLinkFrames enumerates every arm sub-frame that counts as the arm itself
// for collision-allowance purposes. link_base and link1 are deep inside the
// base and unlikely to collide with anything, but we list them for completeness
// so a single named-obstacle allow covers the whole arm.
var armLinkSuffixes = []string{"link_base", "link1", "link2", "link3", "link4", "link5", "link6"}

// gripperAllowedVsObstacle returns the frame pairs whose collision with the
// named obstacle the planner should skip when the gripper is near it with
// something in the claws. Covers the gripper (claws, case-gripper), the full
// arm (link_base through link6), and the held object so brush-pasts anywhere
// along the arm or gripper housing don't fail planning.
func (b *bartender) gripperAllowedVsObstacle(obstacle string) []AllowedCollision {
	out := make([]AllowedCollision, 0, 3+len(armLinkSuffixes)+1)
	out = append(out,
		AllowedCollision{Frame1: "claws-middle", Frame2: obstacle},
		AllowedCollision{Frame1: b.cfg.GripperName + ":claws", Frame2: obstacle},
		AllowedCollision{Frame1: b.cfg.GripperName + ":case-gripper", Frame2: obstacle},
	)
	for _, suffix := range armLinkSuffixes {
		out = append(out, AllowedCollision{Frame1: b.cfg.ArmName + ":" + suffix, Frame2: obstacle})
	}
	out = append(out, AllowedCollision{Frame1: heldObjectFrameName, Frame2: obstacle})
	return out
}

func (b *bartender) mixerProtectorAllowed() []AllowedCollision {
	if b.cfg.MixerProtectorObstacle == "" {
		return nil
	}
	return b.gripperAllowedVsObstacle(b.cfg.MixerProtectorObstacle)
}

func (b *bartender) bottlesProtectorAllowed() []AllowedCollision {
	if b.cfg.BottlesProtectorObstacle == "" {
		return nil
	}
	return b.gripperAllowedVsObstacle(b.cfg.BottlesProtectorObstacle)
}

func (b *bartender) iceProtectorAllowed() []AllowedCollision {
	if b.cfg.IceProtectorObstacle == "" {
		return nil
	}
	return b.gripperAllowedVsObstacle(b.cfg.IceProtectorObstacle)
}

// poseAllowedCollisions expands a pose's allowed_obstacles list into the same
// frame pairs the protector helpers use.
func (b *bartender) poseAllowedCollisions(pd *poseData) []AllowedCollision {
	if pd == nil || len(pd.allowedObstacles) == 0 {
		return nil
	}
	out := make([]AllowedCollision, 0, len(pd.allowedObstacles)*4)
	for _, obstacle := range pd.allowedObstacles {
		out = append(out, b.gripperAllowedVsObstacle(obstacle)...)
	}
	return out
}

// armAndGripperVsHeldAllowed returns the frame pairs the planner should skip
// when something is in the gripper: only the bodies the held object is clamped
// against (gripper and wrist link), which the capsule overlaps by construction.
// The rest of the arm stays checked — a long bottle can physically swing into
// the base or lower links. Returns nil when nothing is held.
func (b *bartender) armAndGripperVsHeldAllowed() []AllowedCollision {
	if b.heldGeomFrame == nil {
		return nil
	}
	return []AllowedCollision{
		{Frame1: "claws-middle", Frame2: heldObjectFrameName},
		{Frame1: b.cfg.GripperName + ":claws", Frame2: heldObjectFrameName},
		{Frame1: b.cfg.GripperName + ":case-gripper", Frame2: heldObjectFrameName},
		{Frame1: b.cfg.ArmName + ":link6", Frame2: heldObjectFrameName},
	}
}

// Straight-line through space; the empty-handed grab/release paths that risk
// knocking the bottle laterally. For moves WITH a bottle in hand, use
// linearCarryToPose so the no-spill orientation constraint is applied too.
func (b *bartender) linearMoveToPose(ctx context.Context, sw toggleswitch.Switch, poseName string, allowed ...AllowedCollision) (time.Duration, error) {
	pd, err := fetchPose(ctx, sw, poseName)
	if err != nil {
		return 0, err
	}
	label := sw.Name().ShortName() + ":" + poseName + ":linear"
	constraints := &motionplan.Constraints{
		LinearConstraint: []motionplan.LinearConstraint{{
			LineToleranceMm:          pd.linearLineToleranceMm(),
			OrientationToleranceDegs: pd.linearOrientationToleranceDegs(),
		}},
	}
	constraints = b.withAllowedCollisions(constraints, allowed)
	return b.moveToResolvedPose(ctx, pd, label, constraints, nil)
}

// Straight-line + no-spill constraint. For lifting/lowering a bottle held in the
// gripper: linear so the planner doesn't swing the bottle sideways, orientation
// so the planner doesn't rotate the gripper mid-ascent and tip the bottle.
func (b *bartender) linearCarryToPose(ctx context.Context, sw toggleswitch.Switch, poseName string, allowed ...AllowedCollision) (time.Duration, error) {
	pd, err := fetchPose(ctx, sw, poseName)
	if err != nil {
		return 0, err
	}
	label := sw.Name().ShortName() + ":" + poseName + ":linear-carry"
	constraints := &motionplan.Constraints{
		LinearConstraint: []motionplan.LinearConstraint{{
			LineToleranceMm:          pd.linearLineToleranceMm(),
			OrientationToleranceDegs: pd.linearOrientationToleranceDegs(),
		}},
		OrientationConstraint: []motionplan.OrientationConstraint{
			{OrientationToleranceDegs: pd.noSpillToleranceDegs(), IgnoreTheta: true},
		},
	}
	constraints = b.withAllowedCollisions(constraints, allowed)
	return b.moveHeldToResolvedPose(ctx, pd, label, constraints, nil)
}

// Carry = plan-wide orientation constraint on the held object's long axis, so
// the shaker stays within ~15° of upright for the whole move and doesn't slosh.
// IgnoreTheta so rotation about the shaker's own vertical axis is still free.
func (b *bartender) carryHeldLevel(ctx context.Context, sw toggleswitch.Switch, poseName string, allowed ...AllowedCollision) (time.Duration, error) {
	pd, err := fetchPose(ctx, sw, poseName)
	if err != nil {
		return 0, err
	}
	return b.carryHeldLevelToResolved(ctx, pd, sw.Name().ShortName()+":"+poseName+":carry", allowed...)
}

func (b *bartender) carryHeldLevelToResolved(ctx context.Context, pd *poseData, label string, allowed ...AllowedCollision) (time.Duration, error) {
	constraints := &motionplan.Constraints{
		OrientationConstraint: []motionplan.OrientationConstraint{
			{OrientationToleranceDegs: pd.noSpillToleranceDegs(), IgnoreTheta: true},
		},
	}
	constraints = b.withAllowedCollisions(constraints, allowed)
	return b.moveHeldToResolvedPose(ctx, pd, label, constraints, nil)
}

// moveHeldToResolvedPose plans with the held-object frame as the goal, so any
// orientation constraint bounds the shaker's own long axis rather than the
// gripper's tool axis. Falls back to moveToResolvedPose when nothing is held.
func (b *bartender) moveHeldToResolvedPose(ctx context.Context, pd *poseData, label string, constraints *motionplan.Constraints, opts *arm.MoveOptions) (time.Duration, error) {
	if opts == nil {
		opts = moveOptionsFromCfg(pd.velDegsPerSec, pd.accDegsPerSec2)
	}
	if poseAllowed := b.poseAllowedCollisions(pd); len(poseAllowed) > 0 {
		constraints = b.withAllowedCollisions(constraints, poseAllowed)
	}
	if heldAllowed := b.armAndGripperVsHeldAllowed(); len(heldAllowed) > 0 {
		constraints = b.withAllowedCollisions(constraints, heldAllowed)
	}
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

// computeCircularPoses returns pointsPerRev poses evenly spaced on a circle in the XY plane of
// center's frame, holding center's orientation. The closing 360° point is omitted.
func computeCircularPoses(center spatialmath.Pose, radiusMM float64, pointsPerRev int) []spatialmath.Pose {
	poses := make([]spatialmath.Pose, pointsPerRev)
	for i := range pointsPerRev {
		angle := 2 * math.Pi * float64(i) / float64(pointsPerRev)
		offset := r3.Vector{X: radiusMM * math.Cos(angle), Y: radiusMM * math.Sin(angle)}
		poses[i] = spatialmath.NewPose(center.Point().Add(offset), center.Orientation())
	}
	return poses
}

// circleAroundPose plans one horizontal revolution around poseName once, then replays it until
// duration elapses. The arm is expected to already be at poseName; the pose sets the circle's size.
func (b *bartender) circleAroundPose(ctx context.Context, sw toggleswitch.Switch, poseName string, duration time.Duration, allowed ...AllowedCollision) error {
	if duration <= 0 {
		return nil
	}
	pd, err := fetchPose(ctx, sw, poseName)
	if err != nil {
		return err
	}
	label := sw.Name().ShortName() + ":" + poseName + ":circle"
	constraints := &motionplan.Constraints{
		LinearConstraint: []motionplan.LinearConstraint{{
			LineToleranceMm:          pd.linearLineToleranceMm(),
			OrientationToleranceDegs: pd.linearOrientationToleranceDegs(),
		}},
	}
	constraints = b.withAllowedCollisions(constraints, allowed)
	constraints = b.withAllowedCollisions(constraints, b.poseAllowedCollisions(pd))
	constraints = b.withAllowedCollisions(constraints, b.armAndGripperVsHeldAllowed())

	fs, fsInputs, err := b.currentInputs(ctx)
	if err != nil {
		return err
	}
	// Circle in world so it stays horizontal whatever frame the pose was saved in.
	centerTF, err := fs.Transform(fsInputs.ToLinearInputs(), referenceframe.NewPoseInFrame(pd.refFrame, pd.pose), referenceframe.World)
	if err != nil {
		return fmt.Errorf("transform circle center to world: %w", err)
	}
	componentName := pd.componentName
	if componentName == "" {
		componentName = b.cfg.ArmName
	}
	waypoints := computeCircularPoses(centerTF.(*referenceframe.PoseInFrame).Pose(), pd.circleRadius(), pd.circlePoints())
	goals := make([]*armplanning.PlanState, 0, len(waypoints)+1)
	for _, p := range waypoints {
		goals = append(goals, armplanning.NewPlanState(
			referenceframe.FrameSystemPoses{componentName: referenceframe.NewPoseInFrame(referenceframe.World, p)},
			nil,
		))
	}
	// Ending on the start joints makes the trajectory a closed loop, so each replay begins where the last one ended.
	goals = append(goals, armplanning.NewPlanState(nil, fsInputs))

	plan, err := b.planMotion(ctx, &armplanning.PlanRequest{
		FrameSystem: fs,
		StartState:  armplanning.NewPlanState(nil, fsInputs),
		Goals:       goals,
		Constraints: constraints,
	}, label)
	if err != nil {
		return err
	}
	positions, err := plan.Trajectory().GetFrameInputs(b.cfg.ArmName)
	if err != nil {
		return fmt.Errorf("extract trajectory: %w", err)
	}
	opts := moveOptionsFromCfg(pd.velDegsPerSec, pd.accDegsPerSec2)
	deadline := time.Now().Add(duration)
	for rev := 1; time.Now().Before(deadline); rev++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("cancelled during %s: %w", label, err)
		}
		if err := b.arm.MoveThroughJointPositions(ctx, positions, opts, nil); err != nil {
			return fmt.Errorf("execute %s revolution %d: %w", label, rev, err)
		}
	}
	return nil
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
