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
	ParkedBottleGeometry  *HeldObjectGeometry `json:"parked_bottle_geometry,omitempty"`
	HeldShakerGeometry    *HeldObjectGeometry `json:"held_shaker_geometry,omitempty"`
	HeldStirrerGeometry   *HeldObjectGeometry `json:"held_stirrer_geometry,omitempty"`
	OrderSensorName       string              `json:"order_sensor_name,omitempty"`
	SaveMotionRequestsDir string              `json:"save_motion_requests_dir,omitempty"`
	PourVelDegsPerSec     float64             `json:"pour_vel_degs_per_sec,omitempty"`
	PourAccDegsPerSec2    float64             `json:"pour_acc_degs_per_sec2,omitempty"`
	ServeVelDegsPerSec    float64             `json:"serve_vel_degs_per_sec,omitempty"`
	ServeAccDegsPerSec2   float64             `json:"serve_acc_degs_per_sec2,omitempty"`
	// Horizontal distance (mm) from the gripper to the bottle mouth while pouring, toward the side the
	// top of the bottle tips (default 100). Negative flips the side.
	PourMouthOffsetMM *float64 `json:"pour_mouth_offset_mm,omitempty"`
	MaxPourOffsetMM   float64  `json:"max_pour_offset_mm,omitempty"`
	// Switch holding the shaker and serve-approach/serve-tilt poses used by find_and_pour_from_shaker
	// (default "serving-glass-center").
	ServingStation string `json:"serving_station,omitempty"`
	// Vision service returning world-frame glass point clouds, e.g. viam:cocktail-bot:glass-finder on the wrist camera.
	GlassFinderName string `json:"glass_finder_name,omitempty"`
	// World z (mm) of the table surface (default 0); the search keeps the camera aimed where its glass-look view meets it.
	GlassTableZMM       float64   `json:"glass_table_z_mm,omitempty"`
	GlassSearchLowerMM  []float64 `json:"glass_search_lower_mm,omitempty"`
	GlassSearchPanDeg   []float64 `json:"glass_search_pan_deg,omitempty"`
	GlassSearchSettleMs int       `json:"glass_search_settle_ms,omitempty"`
	// Detections whose centroids are closer than this in x, y count as one glass (default 50).
	GlassMinSeparationMM   float64           `json:"glass_min_separation_mm,omitempty"`
	DataDir                string            `json:"data_dir,omitempty"`
	PickupObstacles        map[string]string `json:"pickup_obstacles,omitempty"`
	RecipeDefaults         *RecipeDefaults   `json:"recipe_defaults,omitempty"`
	Glasses                []Glass           `json:"glasses,omitempty"`
	DefaultGlassObstacle   string            `json:"default_glass_obstacle,omitempty"`
	PourIntoShakerDwellMs  int               `json:"pour_into_shaker_dwell_ms,omitempty"`
	MixerProtectorObstacle string            `json:"mixer_protector_obstacle,omitempty"`
	// Switch powering the mixer (e.g. a smart plug); mix turns it on only while the shaker is
	// seated. Empty leaves the mixer to be switched by hand.
	MixerPowerSwitchName     string   `json:"mixer_power_switch_name,omitempty"`
	BottlesProtectorObstacle string   `json:"bottles_protector_obstacle,omitempty"`
	IceProtectorObstacle     string   `json:"ice_protector_obstacle,omitempty"`
	LoadingProtectorObstacle string   `json:"loading_protector_obstacle,omitempty"`
	SlotNames                []string `json:"slot_names,omitempty"`
	ParkingSwitchName        string   `json:"parking_switch_name,omitempty"`
	LoadingSpotSwitchName    string   `json:"loading_spot_switch_name,omitempty"`
	MixPrestagePose          string   `json:"mix_prestage_pose,omitempty"`
	// The hand strainer, which rides on the shaker from strainer-seat until it is back on its mount.
	StrainerObstacle string `json:"strainer_obstacle,omitempty"`
	// Only the carried strainer may touch the mount, while it seats into or leaves it.
	StrainerMountObstacle string `json:"strainer_mount_obstacle,omitempty"`
	// Arm, gripper, shaker and strainer may enter the shield only on the strainer-* moves.
	StrainerMountShieldObstacle string `json:"strainer_mount_shield_obstacle,omitempty"`
	// Joint-space configurations the arm snaps to before each bottle pickup or
	// return, split by which third of the rack the slot is on and by shelf.
	// Pins the elbow branch so hover/grab/lift plan from a known-good config
	// instead of whatever branch IK picked up from the prior move — cbirrt
	// otherwise fails on mid-array slots when the previous move left the arm
	// in the wrong IK family. Letter strings assign each slot letter to a
	// zone; a letter in none of the strings, or a zone with an empty joint
	// array, is a no-op for that slot. Top-shelf slots prefer the *Top
	// joints when set and fall back to the plain (bottom) ones otherwise.
	//
	// TODO(bandaid): remove once Dan G's motion planning improvements land —
	// those should pick the correct IK branch itself without the caller
	// pre-pinning the config.
	// Kill-switch for the whole side-prestage helper: true no-ops it for every slot
	// pickup and return. Defaults to false so existing behavior is unchanged. Flip to
	// true when prestage is timing out often enough to be worse than the cbirrt
	// failures it was supposed to avoid.
	BottlePrestageDisabled        bool      `json:"bottle_prestage_disabled,omitempty"`
	BottlePrestageLeftJoints      []float64 `json:"bottle_prestage_left_joints_degs,omitempty"`
	BottlePrestageMiddleJoints    []float64 `json:"bottle_prestage_middle_joints_degs,omitempty"`
	BottlePrestageRightJoints     []float64 `json:"bottle_prestage_right_joints_degs,omitempty"`
	BottlePrestageLeftTopJoints   []float64 `json:"bottle_prestage_left_top_joints_degs,omitempty"`
	BottlePrestageMiddleTopJoints []float64 `json:"bottle_prestage_middle_top_joints_degs,omitempty"`
	BottlePrestageRightTopJoints  []float64 `json:"bottle_prestage_right_top_joints_degs,omitempty"`
	BottlePrestageLeftLetters     string    `json:"bottle_prestage_left_letters,omitempty"`
	BottlePrestageMiddleLetters   string    `json:"bottle_prestage_middle_letters,omitempty"`
	BottlePrestageRightLetters    string    `json:"bottle_prestage_right_letters,omitempty"`
	// The attachment clamped to the shaker's rim, as it stands at shaker-home. It rides with the shaker.
	ShakerAttachmentObstacle string `json:"shaker_attachment_obstacle,omitempty"`
	// Incoming webhook URL for order outcome alerts; empty disables.
	SlackWebhookURL string `json:"slack_webhook_url,omitempty"`
	// Viam machine id; when set, failure alerts include a deep-link to app.viam.com machine logs.
	MachineID      string          `json:"machine_id,omitempty"`
	ShakerCleaning *ShakerCleaning `json:"shaker_cleaning,omitempty"`
	// Pourer oz (string key, e.g. "1.5") → dwell ms at the tilt; sizes not listed use pour_into_shaker_dwell_ms.
	PourDwellMsByPourerOz map[string]int `json:"pour_dwell_ms_by_pourer_oz,omitempty"`
	Fridge                *Fridge        `json:"fridge,omitempty"`
}

// ShakerCleaning is the v1 feature flag for rinsing the shaker on the cleaner
// after make_cocktail serves it.
type ShakerCleaning struct {
	Enabled         bool   `json:"enabled"`
	Station         string `json:"station,omitempty"`
	DwellMs         int    `json:"dwell_ms,omitempty"`
	Obstacle        string `json:"obstacle,omitempty"`
	PowerSwitchName string `json:"power_switch_name,omitempty"`
	// Pause after pressing the shaker onto the cleaner before turning cleaner power on.
	PowerOnDelayMs int `json:"power_on_delay_ms,omitempty"`
	// Pause after cutting cleaner power before lifting the shaker, so water can drip off.
	DrainMs int `json:"drain_ms,omitempty"`
	// Joint-space pose on station passed through before the cleaner grab, like mix_prestage_pose.
	PrestagePose string `json:"prestage_pose,omitempty"`
	// Held shaker cylinder for the cleaner's lower grip; defaults to held_shaker_geometry.
	HeldShakerGeometry *HeldObjectGeometry `json:"held_shaker_geometry,omitempty"`
}

func (s *ShakerCleaning) enabled() bool {
	return s != nil && s.Enabled
}

type RecipeDefaults struct {
	ShakerSource string `json:"shaker_source,omitempty"`
	IceStation   string `json:"ice_station,omitempty"`
	MixerStation string `json:"mixer_station,omitempty"`
	StrainFlow   string `json:"strain_flow,omitempty"`
	ServeStation string `json:"serve_station,omitempty"`
	IceDwellMs   int    `json:"ice_dwell_ms,omitempty"`
	MixDwellMs   int    `json:"mix_dwell_ms,omitempty"`
	StirStation  string `json:"stir_station,omitempty"`
	StirDwellMs  int    `json:"stir_dwell_ms,omitempty"`
	ServeMs      int    `json:"serve_ms,omitempty"`
	StrainDumpMs int    `json:"strain_dump_ms,omitempty"`
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
	if err := c.ParkedBottleGeometry.Validate(path + ".parked_bottle_geometry"); err != nil {
		return nil, nil, err
	}
	if err := c.HeldShakerGeometry.Validate(path + ".held_shaker_geometry"); err != nil {
		return nil, nil, err
	}
	if err := c.HeldStirrerGeometry.Validate(path + ".held_stirrer_geometry"); err != nil {
		return nil, nil, err
	}
	if c.PourVelDegsPerSec < 0 {
		return nil, nil, errors.New(path + ": pour_vel_degs_per_sec must be > 0 if set")
	}
	if c.PourAccDegsPerSec2 < 0 {
		return nil, nil, errors.New(path + ": pour_acc_degs_per_sec2 must be > 0 if set")
	}
	if c.ServeVelDegsPerSec < 0 {
		return nil, nil, errors.New(path + ": serve_vel_degs_per_sec must be > 0 if set")
	}
	if c.ServeAccDegsPerSec2 < 0 {
		return nil, nil, errors.New(path + ": serve_acc_degs_per_sec2 must be > 0 if set")
	}
	if _, err := parsePourDwells(c.PourDwellMsByPourerOz); err != nil {
		return nil, nil, fmt.Errorf("%s: pour_dwell_ms_by_pourer_oz: %w", path, err)
	}
	if c.MaxPourOffsetMM < 0 {
		return nil, nil, errors.New(path + ": max_pour_offset_mm must be > 0 if set")
	}
	if c.GlassMinSeparationMM < 0 {
		return nil, nil, errors.New(path + ": glass_min_separation_mm must be >= 0")
	}
	if c.GlassSearchSettleMs < 0 {
		return nil, nil, errors.New(path + ": glass_search_settle_ms must be >= 0")
	}
	for _, l := range c.GlassSearchLowerMM {
		if l < 0 {
			return nil, nil, errors.New(path + ": glass_search_lower_mm entries must be >= 0 (mm below glass-look)")
		}
	}
	switcherSet := make(map[string]struct{}, len(c.PoseSwitcherNames))
	for _, n := range c.PoseSwitcherNames {
		switcherSet[n] = struct{}{}
	}
	seenSlot := make(map[string]struct{}, len(c.SlotNames))
	for _, slot := range c.SlotNames {
		if slot == "" {
			return nil, nil, errors.New(path + ": slot_names contains an empty entry")
		}
		if _, dup := seenSlot[slot]; dup {
			return nil, nil, fmt.Errorf("%s: slot_names has duplicate %q", path, slot)
		}
		if _, ok := switcherSet[slot]; !ok {
			return nil, nil, fmt.Errorf("%s: slot_names[%q] is not in pose_switcher_names", path, slot)
		}
		seenSlot[slot] = struct{}{}
	}
	if err := validateGlasses(path, c.Glasses, switcherSet); err != nil {
		return nil, nil, err
	}
	if sc := c.ShakerCleaning; sc != nil {
		if err := sc.HeldShakerGeometry.Validate(path + ".shaker_cleaning.held_shaker_geometry"); err != nil {
			return nil, nil, err
		}
	}
	if sc := c.ShakerCleaning; sc.enabled() {
		if sc.Station == "" {
			return nil, nil, errors.New(path + ": shaker_cleaning.station is required when enabled")
		}
		if _, ok := switcherSet[sc.Station]; !ok {
			return nil, nil, fmt.Errorf("%s: shaker_cleaning.station %q is not in pose_switcher_names", path, sc.Station)
		}
		if sc.DwellMs < 0 {
			return nil, nil, errors.New(path + ": shaker_cleaning.dwell_ms must be >= 0")
		}
		if sc.PowerOnDelayMs < 0 {
			return nil, nil, errors.New(path + ": shaker_cleaning.power_on_delay_ms must be >= 0")
		}
		if sc.DrainMs < 0 {
			return nil, nil, errors.New(path + ": shaker_cleaning.drain_ms must be >= 0")
		}
	}
	if c.Fridge != nil {
		if err := c.Fridge.validate(path+".fridge", switcherSet, seenSlot); err != nil {
			return nil, nil, err
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
	if c.MixerPowerSwitchName != "" {
		deps = append(deps, toggleswitch.Named(c.MixerPowerSwitchName).String())
	}
	if sc := c.ShakerCleaning; sc != nil && sc.PowerSwitchName != "" {
		deps = append(deps, toggleswitch.Named(sc.PowerSwitchName).String())
	}
	for _, name := range c.glassObstacleNames() {
		deps = append(deps, gripper.Named(name).String())
	}
	if c.Fridge != nil {
		deps = append(deps, gripper.Named(c.Fridge.DoorObstacle).String())
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

func (c *Config) servingStation() string {
	if c.ServingStation == "" {
		return defaultServingStation
	}
	return c.ServingStation
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

func (c *Config) glassMinSeparationMM() float64 {
	if c.GlassMinSeparationMM == 0 {
		return defaultGlassMinSeparationMM
	}
	return c.GlassMinSeparationMM
}

func (c *Config) glassSearchSettleMs() int {
	if c.GlassSearchSettleMs == 0 {
		return defaultGlassSearchSettleMs
	}
	return c.GlassSearchSettleMs
}

func (d *RecipeDefaults) stirStation() string {
	if d.StirStation == "" {
		return defaultStirStation
	}
	return d.StirStation
}

func (d *RecipeDefaults) stirDwellMs() int {
	if d.StirDwellMs == 0 {
		return d.MixDwellMs
	}
	return d.StirDwellMs
}
