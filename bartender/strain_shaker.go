package bartender

import (
	"context"
	"fmt"
	"time"
)

const (
	poseStrainApproach          = "strain-approach"
	poseStrainTilt              = "strain-tilt"
	poseFilterPickupHover       = "filter-pickup-hover"
	poseFilterPickupGrab        = "filter-pickup-grab"
	poseGarbageHover            = "garbage-hover"
	poseGarbageApproach         = "garbage-approach"
	poseGarbageTilt             = "garbage-tilt"
	poseParkingHover            = "parking-hover"
	poseParkingGrab             = "parking-grab"
	poseParkingCarryHover       = "parking-carry-hover"
	poseParkingFilterHover      = "parking-filter-hover"
	poseParkingFilterPlace      = "parking-filter-place"
	poseReceiveShakerHover      = "receive-shaker-hover"
	poseReceiveShakerGrab       = "receive-shaker-grab"
	poseReceiveShakerCarryHover = "receive-shaker-carry-hover"
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
	shakerAllow := b.pickupAllowedCollisions("shaker")
	parkingAllow := b.pickupAllowedCollisions("parking")
	receiveAllow := b.pickupAllowedCollisions("receive-shaker")

	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("start-home: %w", err)
	}

	if _, err := b.moveArmToPoseOnSwitch(ctx, sourceSw, poseShakerHover); err != nil {
		return fmt.Errorf("source shaker-hover: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("open gripper before source shaker grab: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, sourceSw, poseShakerGrab, shakerAllow...); err != nil {
		return fmt.Errorf("linear-to source shaker grab: %w", err)
	}
	if _, err := b.gripper.Grab(ctx, nil); err != nil {
		return fmt.Errorf("close gripper on source shaker: %w", err)
	}
	if err := b.attachHeld(ctx, b.cfg.HeldShakerGeometry); err != nil {
		return fmt.Errorf("attach held shaker A: %w", err)
	}
	if _, err := b.carryHeldLevel(ctx, sourceSw, poseShakerCarryHover, shakerAllow...); err != nil {
		return fmt.Errorf("carry to source shaker-carry-hover: %w", err)
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

	if _, err := b.carryHeldLevel(ctx, flowSw, poseParkingCarryHover, parkingAllow...); err != nil {
		return fmt.Errorf("carry A to parking carry-hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, flowSw, poseParkingGrab, parkingAllow...); err != nil {
		return fmt.Errorf("linear-to parking grab: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("release A at parking: %w", err)
	}
	b.detachHeld()
	if _, err := b.linearMoveToPose(ctx, flowSw, poseParkingHover, parkingAllow...); err != nil {
		return fmt.Errorf("linear-retreat from parked A: %w", err)
	}

	if _, err := b.moveArmToPoseOnSwitch(ctx, flowSw, poseFilterPickupHover); err != nil {
		return fmt.Errorf("move to filter-pickup-hover: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("open gripper before filter grab: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, flowSw, poseFilterPickupGrab); err != nil {
		return fmt.Errorf("linear-descend to filter-pickup-grab: %w", err)
	}
	if _, err := b.gripper.Grab(ctx, nil); err != nil {
		return fmt.Errorf("close gripper on filter: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, flowSw, poseFilterPickupHover); err != nil {
		return fmt.Errorf("linear-lift filter: %w", err)
	}

	if _, err := b.moveArmToPoseOnSwitch(ctx, flowSw, poseGarbageHover); err != nil {
		return fmt.Errorf("carry filter to garbage-hover: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, flowSw, poseGarbageApproach); err != nil {
		return fmt.Errorf("linear to garbage-approach: %w", err)
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
	if _, err := b.linearMoveToPose(ctx, flowSw, poseGarbageHover); err != nil {
		return fmt.Errorf("linear-retreat from garbage: %w", err)
	}

	if _, err := b.moveArmToPoseOnSwitch(ctx, flowSw, poseParkingFilterHover); err != nil {
		return fmt.Errorf("carry filter to parking-filter-hover: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, flowSw, poseParkingFilterPlace); err != nil {
		return fmt.Errorf("linear-place filter on parked A: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("release filter on parked A: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, flowSw, poseParkingFilterHover); err != nil {
		return fmt.Errorf("linear-retreat from parked filter: %w", err)
	}

	if _, err := b.moveArmToPoseOnSwitch(ctx, flowSw, poseReceiveShakerHover); err != nil {
		return fmt.Errorf("move to receive-shaker-hover: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("open gripper before receive-shaker grab: %w", err)
	}
	if _, err := b.linearMoveToPose(ctx, flowSw, poseReceiveShakerGrab, receiveAllow...); err != nil {
		return fmt.Errorf("linear-to receive-shaker grab: %w", err)
	}
	if _, err := b.gripper.Grab(ctx, nil); err != nil {
		return fmt.Errorf("close gripper on receive-shaker: %w", err)
	}
	if err := b.attachHeld(ctx, b.cfg.HeldShakerGeometry); err != nil {
		return fmt.Errorf("attach held shaker B: %w", err)
	}
	if _, err := b.carryHeldLevel(ctx, flowSw, poseReceiveShakerCarryHover, receiveAllow...); err != nil {
		return fmt.Errorf("carry B to receive-shaker-carry-hover: %w", err)
	}

	if _, err := b.carryHeldLevel(ctx, sourceSw, poseShakerCarryHover, shakerAllow...); err != nil {
		return fmt.Errorf("carry B to source carry-hover: %w", err)
	}
	if _, err := b.linearCarryToPose(ctx, sourceSw, poseShakerGrab, shakerAllow...); err != nil {
		return fmt.Errorf("linear-to source shaker grab: %w", err)
	}
	if err := b.gripper.Open(ctx, nil); err != nil {
		return fmt.Errorf("release B at source: %w", err)
	}
	b.detachHeld()
	if _, err := b.linearMoveToPose(ctx, sourceSw, poseShakerHover, shakerAllow...); err != nil {
		return fmt.Errorf("linear-retreat from B at source: %w", err)
	}

	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("end-home: %w", err)
	}
	return nil
}
