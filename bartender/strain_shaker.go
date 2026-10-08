package bartender

import (
	"context"
	"fmt"
	"time"
)

const (
	poseStrainApproach    = "strain-approach"
	poseStrainTilt        = "strain-tilt"
	poseFilterHover       = "filter-hover"
	poseFilterGrab        = "filter-grab"
	poseGarbageHover      = "garbage-hover"
	poseGarbageApproach   = "garbage-approach"
	poseGarbageTilt       = "garbage-tilt"
	poseParkingHover      = "parking-hover"
	poseParkingDrop       = "parking-drop"
	poseParkingLift       = "parking-lift"
	poseParkingCarryHover = "parking-carry-hover"
	poseFilterApproach    = "filter-approach"
	poseFilterPlace       = "filter-place"
)

type strainShakerRequest struct {
	source       string
	strain       string
	garbage      string
	parking      string
	drainDwellMs int
	dumpDwellMs  int
}

func (b *bartender) strainShaker(ctx context.Context, req strainShakerRequest) error {
	sourceSw, err := b.findSwitch(req.source)
	if err != nil {
		return fmt.Errorf("source switch: %w", err)
	}
	strainSw, err := b.findSwitch(req.strain)
	if err != nil {
		return fmt.Errorf("strain switch: %w", err)
	}
	garbageSw, err := b.findSwitch(req.garbage)
	if err != nil {
		return fmt.Errorf("garbage switch: %w", err)
	}
	parkingSw, err := b.findSwitch(req.parking)
	if err != nil {
		return fmt.Errorf("parking switch: %w", err)
	}

	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("start-home: %w", err)
	}

	if _, err := b.moveArmToPoseOnSwitch(ctx, sourceSw, poseShakerHover); err != nil {
		return fmt.Errorf("source shaker-hover: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sourceSw, poseShakerDrop); err != nil {
		return fmt.Errorf("linear-into source shaker cradle: %w", err)
	}
	if err := b.attachHeld(ctx, b.cfg.HeldShakerGeometry); err != nil {
		return fmt.Errorf("attach held shaker A: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sourceSw, poseShakerLift); err != nil {
		return fmt.Errorf("linear-lift source shaker: %w", err)
	}
	if _, err := b.carryHeldLevel(ctx, sourceSw, poseShakerCarryHover); err != nil {
		return fmt.Errorf("carry to source shaker-carry-hover: %w", err)
	}

	if _, err := b.carryHeldLevel(ctx, strainSw, poseStrainApproach); err != nil {
		return fmt.Errorf("carry to strain-approach: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, strainSw, poseStrainTilt); err != nil {
		return fmt.Errorf("tilt A over filter: %w", err)
	}
	if err := sleepCtx(ctx, time.Duration(req.drainDwellMs)*time.Millisecond); err != nil {
		return fmt.Errorf("strain dwell: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, strainSw, poseStrainApproach); err != nil {
		return fmt.Errorf("untilt A at strain-approach: %w", err)
	}

	if _, err := b.carryHeldLevel(ctx, parkingSw, poseParkingCarryHover); err != nil {
		return fmt.Errorf("carry A to parking carry-hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, parkingSw, poseParkingLift); err != nil {
		return fmt.Errorf("linear-down to parking-lift: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, parkingSw, poseParkingDrop); err != nil {
		return fmt.Errorf("linear-drop A at parking: %w", err)
	}
	b.detachHeld()
	if _, err := b.linearMoveToPose(ctx, parkingSw, poseParkingHover); err != nil {
		return fmt.Errorf("linear-retreat from parked A: %w", err)
	}

	if _, err := b.moveArmToPoseOnSwitch(ctx, strainSw, poseFilterHover); err != nil {
		return fmt.Errorf("move to filter-hover: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("open gripper before filter grab: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, strainSw, poseFilterGrab); err != nil {
		return fmt.Errorf("linear-descend to filter-grab: %w", err)
	}
	if _, err := b.gripper.Grab(ctx, nil); err != nil {
		return fmt.Errorf("close gripper on filter: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, strainSw, poseFilterHover); err != nil {
		return fmt.Errorf("linear-lift filter: %w", err)
	}

	if _, err := b.moveArmToPoseOnSwitch(ctx, garbageSw, poseGarbageHover); err != nil {
		return fmt.Errorf("carry filter to garbage-hover: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, garbageSw, poseGarbageApproach); err != nil {
		return fmt.Errorf("linear to garbage-approach: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, garbageSw, poseGarbageTilt); err != nil {
		return fmt.Errorf("tilt filter over garbage: %w", err)
	}
	if err := sleepCtx(ctx, time.Duration(req.dumpDwellMs)*time.Millisecond); err != nil {
		return fmt.Errorf("dump dwell: %w", err)
	}
	if _, err := b.moveArmToPoseOnSwitch(ctx, garbageSw, poseGarbageApproach); err != nil {
		return fmt.Errorf("untilt filter at garbage-approach: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, garbageSw, poseGarbageHover); err != nil {
		return fmt.Errorf("linear-retreat from garbage: %w", err)
	}

	if _, err := b.moveArmToPoseOnSwitch(ctx, parkingSw, poseFilterApproach); err != nil {
		return fmt.Errorf("carry filter to parking filter-approach: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, parkingSw, poseFilterPlace); err != nil {
		return fmt.Errorf("linear-place filter on parked A: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("release filter on parked A: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, parkingSw, poseFilterApproach); err != nil {
		return fmt.Errorf("linear-retreat from parked filter: %w", err)
	}

	if _, err := b.moveArmToPoseOnSwitch(ctx, strainSw, poseShakerHover); err != nil {
		return fmt.Errorf("move to strain shaker-hover: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, strainSw, poseShakerDrop); err != nil {
		return fmt.Errorf("linear-into strain shaker cradle: %w", err)
	}
	if err := b.attachHeld(ctx, b.cfg.HeldShakerGeometry); err != nil {
		return fmt.Errorf("attach held shaker B: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, strainSw, poseShakerLift); err != nil {
		return fmt.Errorf("linear-lift B: %w", err)
	}
	if _, err := b.carryHeldLevel(ctx, strainSw, poseShakerCarryHover); err != nil {
		return fmt.Errorf("carry B to strain carry-hover: %w", err)
	}

	if _, err := b.carryHeldLevel(ctx, sourceSw, poseShakerCarryHover); err != nil {
		return fmt.Errorf("carry B to source carry-hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sourceSw, poseShakerLift); err != nil {
		return fmt.Errorf("linear-down to source shaker-lift: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sourceSw, poseShakerDrop); err != nil {
		return fmt.Errorf("linear-drop B at source: %w", err)
	}
	b.detachHeld()
	if _, err := b.linearMoveToPose(ctx, sourceSw, poseShakerHover); err != nil {
		return fmt.Errorf("linear-retreat from B at source: %w", err)
	}

	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("end-home: %w", err)
	}
	return nil
}
