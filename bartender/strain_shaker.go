package bartender

import (
	"context"
	"fmt"
	"time"
)

const (
	poseStrainApproach    = "strain-approach"
	poseStrainTilt        = "strain-tilt"
	poseShakerAHover      = "shaker-a-hover"
	poseShakerAApproach   = "shaker-a-approach"
	poseShakerALift       = "shaker-a-lift"
	poseShakerACarryHover = "shaker-a-carry-hover"
	poseShakerBHover      = "shaker-b-hover"
	poseShakerBPlace      = "shaker-b-place"
	poseShakerBDescend    = "shaker-b-descend"
	poseFilterALift       = "filter-a-lift"
	poseFilterAHoverAbove = "filter-a-hover-above"
	poseFilterBHoverAbove = "filter-b-hover-above"
	poseFilterBPlace      = "filter-b-place"
	poseGarbageApproach   = "garbage-approach"
	poseGarbageTilt       = "garbage-tilt"
)

type strainShakerRequest struct {
	source       string
	strainFlow   string
	drainDwellMs int
	dumpDwellMs  int
}

func (b *bartender) strainShaker(ctx context.Context, req strainShakerRequest) error {
	sourceSw, err := b.findSwitch(req.source)
	if err != nil {
		return fmt.Errorf("source switch: %w", err)
	}
	flowSw, err := b.findSwitch(req.strainFlow)
	if err != nil {
		return fmt.Errorf("strain_flow switch: %w", err)
	}
	homeAllow := b.pickupAllowedCollisions("shaker")
	shakerAAllow := b.pickupAllowedCollisions("shaker-a")
	shakerBAllow := b.pickupAllowedCollisions("shaker-b")

	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("start-home: %w", err)
	}

	if _, err := b.moveArmToPoseOnSwitch(ctx, sourceSw, poseShakerHomeHover); err != nil {
		return fmt.Errorf("shaker-home-hover: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("open gripper before shaker-home-approach: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sourceSw, poseShakerHomeApproach, homeAllow...); err != nil {
		return fmt.Errorf("linear-to shaker-home-approach: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sourceSw, poseShakerHomeLift, homeAllow...); err != nil {
		return fmt.Errorf("linear-lift at shaker-home: %w", err)
	}
	if _, err := b.gripper.Grab(ctx, nil); err != nil {
		return fmt.Errorf("close gripper on shaker A: %w", err)
	}
	if err := b.attachHeld(ctx, b.cfg.HeldShakerGeometry); err != nil {
		return fmt.Errorf("attach held shaker A: %w", err)
	}
	if _, err := b.carryHeldLevel(ctx, sourceSw, poseShakerHomeCarryHover, homeAllow...); err != nil {
		return fmt.Errorf("carry to shaker-home-carry-hover: %w", err)
	}

	if _, err := b.carryHeldLevel(ctx, flowSw, poseStrainApproach); err != nil {
		return fmt.Errorf("carry to strain-approach: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, flowSw, poseStrainTilt); err != nil {
		return fmt.Errorf("tilt A over filter: %w", err)
	}
	if err := sleepCtx(ctx, time.Duration(req.drainDwellMs)*time.Millisecond); err != nil {
		return fmt.Errorf("strain dwell: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, flowSw, poseStrainApproach); err != nil {
		return fmt.Errorf("untilt A at strain-approach: %w", err)
	}

	if _, err := b.carryHeldLevel(ctx, flowSw, poseShakerBHover, shakerBAllow...); err != nil {
		return fmt.Errorf("carry A to shaker-b-hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, flowSw, poseShakerBPlace, shakerBAllow...); err != nil {
		return fmt.Errorf("linear-descend to shaker-b-place: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("release A at shaker-b-place: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, flowSw, poseShakerBDescend, shakerBAllow...); err != nil {
		return fmt.Errorf("linear-descend further to shaker-b-descend: %w", err)
	}
	b.detachHeld()
	if _, err := b.linearMoveToPose(ctx, flowSw, poseShakerBHover, shakerBAllow...); err != nil {
		return fmt.Errorf("linear-retreat to shaker-b-hover: %w", err)
	}

	if _, err := b.moveArmToPoseOnSwitch(ctx, flowSw, poseShakerAHover); err != nil {
		return fmt.Errorf("shaker-a-hover before filter grab: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("open gripper before filter grab: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, flowSw, poseShakerAApproach); err != nil {
		return fmt.Errorf("linear-to shaker-a-approach before filter grab: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, flowSw, poseFilterALift); err != nil {
		return fmt.Errorf("linear-to filter-a-lift: %w", err)
	}
	if _, err := b.gripper.Grab(ctx, nil); err != nil {
		return fmt.Errorf("close gripper on filter: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, flowSw, poseFilterAHoverAbove); err != nil {
		return fmt.Errorf("linear-lift to filter-a-hover-above: %w", err)
	}

	if _, err := b.moveArmToPoseOnSwitch(ctx, flowSw, poseGarbageApproach); err != nil {
		return fmt.Errorf("carry filter to garbage-approach: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, flowSw, poseGarbageTilt); err != nil {
		return fmt.Errorf("tilt filter over garbage: %w", err)
	}
	if err := sleepCtx(ctx, time.Duration(req.dumpDwellMs)*time.Millisecond); err != nil {
		return fmt.Errorf("dump dwell: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, flowSw, poseGarbageApproach); err != nil {
		return fmt.Errorf("untilt filter at garbage-approach: %w", err)
	}

	if _, err := b.moveArmToPoseOnSwitch(ctx, flowSw, poseFilterBHoverAbove); err != nil {
		return fmt.Errorf("carry filter to filter-b-hover-above: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, flowSw, poseFilterBPlace); err != nil {
		return fmt.Errorf("linear-descend to filter-b-place: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("release filter at filter-b-place: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, flowSw, poseShakerBDescend); err != nil {
		return fmt.Errorf("linear-descend to shaker-b-descend after filter place: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, flowSw, poseShakerBHover); err != nil {
		return fmt.Errorf("linear-retreat to shaker-b-hover after filter place: %w", err)
	}

	if _, err := b.moveArmToPoseOnSwitch(ctx, flowSw, poseShakerAHover); err != nil {
		return fmt.Errorf("shaker-a-hover before B pickup: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("open gripper before shaker B grab: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, flowSw, poseShakerAApproach, shakerAAllow...); err != nil {
		return fmt.Errorf("linear-to shaker-a-approach for B grab: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, flowSw, poseShakerALift, shakerAAllow...); err != nil {
		return fmt.Errorf("linear-lift at shaker-a: %w", err)
	}
	if _, err := b.gripper.Grab(ctx, nil); err != nil {
		return fmt.Errorf("close gripper on shaker B: %w", err)
	}
	if err := b.attachHeld(ctx, b.cfg.HeldShakerGeometry); err != nil {
		return fmt.Errorf("attach held shaker B: %w", err)
	}
	if _, err := b.carryHeldLevel(ctx, flowSw, poseShakerACarryHover, shakerAAllow...); err != nil {
		return fmt.Errorf("carry B to shaker-a-carry-hover: %w", err)
	}

	if _, err := b.carryHeldLevel(ctx, sourceSw, poseShakerHomeCarryHover, homeAllow...); err != nil {
		return fmt.Errorf("carry B to shaker-home-carry-hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sourceSw, poseShakerHomeLift, homeAllow...); err != nil {
		return fmt.Errorf("linear-descend to shaker-home-lift: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("release B at shaker-home-lift: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sourceSw, poseShakerHomeApproach, homeAllow...); err != nil {
		return fmt.Errorf("linear-descend further to shaker-home-approach: %w", err)
	}
	b.detachHeld()
	if _, err := b.linearMoveToPose(ctx, sourceSw, poseShakerHomeHover, homeAllow...); err != nil {
		return fmt.Errorf("linear-retreat to shaker-home-hover: %w", err)
	}

	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("end-home: %w", err)
	}
	return nil
}
