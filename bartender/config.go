package bartender

import (
	"errors"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/sensor"
	toggleswitch "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/robot/framesystem"
)

type Config struct {
	ArmName               string              `json:"arm_name"`
	GripperName           string              `json:"gripper_name"`
	PoseSwitcherNames     []string            `json:"pose_switcher_names"`
	HeldBottleGeometry    *HeldObjectGeometry `json:"held_bottle_geometry,omitempty"`
	HeldShakerGeometry    *HeldObjectGeometry `json:"held_shaker_geometry,omitempty"`
	OrderSensorName       string              `json:"order_sensor_name,omitempty"`
	SaveMotionRequestsDir string              `json:"save_motion_requests_dir,omitempty"`
	PourVelDegsPerSec     float64             `json:"pour_vel_degs_per_sec,omitempty"`
	PourAccDegsPerSec2    float64             `json:"pour_acc_degs_per_sec2,omitempty"`
	// World XY (mm) of the glass the saved pour-approach/pour-tilt poses pour into.
	PourReferenceGlass *GlassXY `json:"pour_reference_glass,omitempty"`
	MaxPourOffsetMM    float64  `json:"max_pour_offset_mm,omitempty"`
}

func (c *Config) Validate(path string) ([]string, []string, error) {
	if c.ArmName == "" {
		return nil, nil, errors.New(path + ": arm_name is required")
	}
	if c.GripperName == "" {
		return nil, nil, errors.New(path + ": gripper_name is required")
	}
	if len(c.PoseSwitcherNames) == 0 {
		return nil, nil, errors.New(path + ": pose_switcher_names must list at least one switch")
	}
	if err := c.HeldBottleGeometry.Validate(path + ".held_bottle_geometry"); err != nil {
		return nil, nil, err
	}
	if err := c.HeldShakerGeometry.Validate(path + ".held_shaker_geometry"); err != nil {
		return nil, nil, err
	}
	if c.PourVelDegsPerSec < 0 {
		return nil, nil, errors.New(path + ": pour_vel_degs_per_sec must be > 0 if set")
	}
	if c.PourAccDegsPerSec2 < 0 {
		return nil, nil, errors.New(path + ": pour_acc_degs_per_sec2 must be > 0 if set")
	}
	if c.MaxPourOffsetMM < 0 {
		return nil, nil, errors.New(path + ": max_pour_offset_mm must be > 0 if set")
	}
	deps := []string{
		framesystem.PublicServiceName.String(),
		arm.Named(c.ArmName).String(),
		gripper.Named(c.GripperName).String(),
	}
	for _, name := range c.PoseSwitcherNames {
		if name == "" {
			return nil, nil, errors.New(path + ": pose_switcher_names contains an empty entry")
		}
		deps = append(deps, toggleswitch.Named(name).String())
	}

	var optional []string
	if c.OrderSensorName != "" {
		optional = append(optional, sensor.Named(c.OrderSensorName).String())
	}
	return deps, optional, nil
}

func (c *Config) maxPourOffsetMM() float64 {
	if c.MaxPourOffsetMM == 0 {
		return defaultMaxPourOffsetMM
	}
	return c.MaxPourOffsetMM
}
