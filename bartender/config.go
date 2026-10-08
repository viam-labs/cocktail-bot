package bartender

import (
	"errors"
	"fmt"
	"os"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/sensor"
	toggleswitch "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/vision"
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
	// Horizontal distance (mm) from the gripper to the bottle mouth while pouring, toward the side the
	// top of the bottle tips (default 100). Negative flips the side.
	PourMouthOffsetMM *float64 `json:"pour_mouth_offset_mm,omitempty"`
	MaxPourOffsetMM   float64  `json:"max_pour_offset_mm,omitempty"`
	// Vision service returning world-frame glass point clouds, e.g. viam:cocktail-bot:glass-finder on the wrist camera.
	GlassFinderName string `json:"glass_finder_name,omitempty"`
	// World z (mm) of the table surface (default 0); the search keeps the camera aimed where its glass-look view meets it.
	GlassTableZMM       float64            `json:"glass_table_z_mm,omitempty"`
	GlassSearchLowerMM  []float64          `json:"glass_search_lower_mm,omitempty"`
	GlassSearchPanDeg   []float64          `json:"glass_search_pan_deg,omitempty"`
	GlassSearchSettleMs int                `json:"glass_search_settle_ms,omitempty"`
	BottlePourerOz      map[string]float64 `json:"bottle_pourer_oz,omitempty"`
	DataDir             string             `json:"data_dir,omitempty"`
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
	if c.GlassSearchSettleMs < 0 {
		return nil, nil, errors.New(path + ": glass_search_settle_ms must be >= 0")
	}
	for _, l := range c.GlassSearchLowerMM {
		if l < 0 {
			return nil, nil, errors.New(path + ": glass_search_lower_mm entries must be >= 0 (mm below glass-look)")
		}
	}
	for name, oz := range c.BottlePourerOz {
		if oz <= 0 {
			return nil, nil, fmt.Errorf("%s: bottle_pourer_oz[%q] must be > 0", path, name)
		}
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
	if c.GlassFinderName != "" {
		optional = append(optional, vision.Named(c.GlassFinderName).String())
	}
	return deps, optional, nil
}

func (c *Config) dataDir() string {
	if c.DataDir != "" {
		return c.DataDir
	}
	return os.Getenv("VIAM_MODULE_DATA")
}

func (c *Config) pourMouthOffsetMM() float64 {
	if c.PourMouthOffsetMM == nil {
		return defaultPourMouthOffsetMM
	}
	return *c.PourMouthOffsetMM
}

func (c *Config) maxPourOffsetMM() float64 {
	if c.MaxPourOffsetMM == 0 {
		return defaultMaxPourOffsetMM
	}
	return c.MaxPourOffsetMM
}

func (c *Config) glassSearchLowerMM() []float64 {
	if len(c.GlassSearchLowerMM) == 0 {
		return defaultGlassSearchLowerMM
	}
	return c.GlassSearchLowerMM
}

func (c *Config) glassSearchPanDeg() []float64 {
	if len(c.GlassSearchPanDeg) == 0 {
		return defaultGlassSearchPanDeg
	}
	return c.GlassSearchPanDeg
}

func (c *Config) glassSearchSettleMs() int {
	if c.GlassSearchSettleMs == 0 {
		return defaultGlassSearchSettleMs
	}
	return c.GlassSearchSettleMs
}
