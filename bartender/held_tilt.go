package bartender

import (
	"fmt"
	"maps"
	"math"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	heldTiltSlackDegs   = 15.0
	heldTiltCheckStepRd = math.Pi / 180
)

// heldTiltPastEndpoints walks the joint-space path the arm will execute and fails if the held
// object tips more than slackDegs past the larger of its start and end tilts. Unconstrained held
// moves (pour tilts, strains, flips) only bound the endpoints, so a plan that unwinds a wrist
// joint a full turn can swing a bottle upside down mid-move while both endpoints look fine.
func heldTiltPastEndpoints(
	fs *referenceframe.FrameSystem,
	start referenceframe.FrameSystemInputs,
	armName string,
	path [][]referenceframe.Input,
	slackDegs float64,
) error {
	if len(path) == 0 {
		return nil
	}
	inputs := maps.Clone(start)
	tiltAt := func(q []referenceframe.Input) (float64, error) {
		inputs[armName] = q
		tf, err := fs.Transform(inputs.ToLinearInputs(), referenceframe.NewPoseInFrame(heldObjectFrameName, spatialmath.NewZeroPose()), referenceframe.World)
		if err != nil {
			return 0, fmt.Errorf("transform %q to world: %w", heldObjectFrameName, err)
		}
		oz := tf.(*referenceframe.PoseInFrame).Pose().Orientation().OrientationVectorRadians().OZ
		return math.Acos(math.Max(-1, math.Min(1, oz))) * 180 / math.Pi, nil
	}

	full := append([][]referenceframe.Input{start[armName]}, path...)
	first, err := tiltAt(full[0])
	if err != nil {
		return err
	}
	last, err := tiltAt(full[len(full)-1])
	if err != nil {
		return err
	}
	limit := math.Max(first, last) + slackDegs

	for i := 1; i < len(full); i++ {
		a, b := full[i-1], full[i]
		if len(a) != len(b) {
			return fmt.Errorf("trajectory step %d has %d joints, previous has %d", i, len(b), len(a))
		}
		maxDelta := 0.0
		for j := range a {
			maxDelta = math.Max(maxDelta, math.Abs(b[j]-a[j]))
		}
		steps := max(1, int(math.Ceil(maxDelta/heldTiltCheckStepRd)))
		q := make([]referenceframe.Input, len(a))
		for s := 1; s <= steps; s++ {
			f := float64(s) / float64(steps)
			for j := range q {
				q[j] = a[j] + f*(b[j]-a[j])
			}
			tilt, err := tiltAt(q)
			if err != nil {
				return err
			}
			if tilt > limit {
				return fmt.Errorf("held object would tip to %.0f° from upright mid-move but only %.0f° at the start and %.0f° at the end; the plan likely unwinds a wrist joint",
					tilt, first, last)
			}
		}
	}
	return nil
}
