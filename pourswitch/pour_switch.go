// Package pourswitch registers viam:cocktail-bot:pour-switch, a two-position switch (upright, pour) that
// tilts the held cup about its lip through the bartender, so the lip stays put while the cup turns.
package pourswitch

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	toggleswitch "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module/trace"
	"go.viam.com/rdk/resource"
	generic "go.viam.com/rdk/services/generic"
)

var Model = resource.NewModel("viam", "cocktail-bot", "pour-switch")

func init() {
	resource.RegisterComponent(toggleswitch.API, Model,
		resource.Registration[toggleswitch.Switch, *Config]{
			Constructor: newPourSwitch,
		},
	)
}

const (
	positionUpright uint32 = iota
	positionPour
)

var positionNames = []string{"upright", "pour"}

type Vec struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type Config struct {
	Bartender   string `json:"bartender"`
	PrePourPose string `json:"pre_pour_pose,omitempty"`
	// Gripper frame, mm: the rim point the liquid leaves over, and the centre of the cup's opening.
	LipMM          *Vec    `json:"lip_mm"`
	RimCenterMM    *Vec    `json:"rim_center_mm"`
	TiltDegs       float64 `json:"tilt_degs,omitempty"`
	StepDegs       float64 `json:"step_degs,omitempty"`
	VelDegsPerSec  float64 `json:"vel_degs_per_sec,omitempty"`
	AccDegsPerSec2 float64 `json:"acc_degs_per_sec2,omitempty"`
}

func (cfg *Config) Validate(path string) ([]string, []string, error) {
	if cfg.Bartender == "" {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, "bartender")
	}
	if cfg.LipMM == nil {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, "lip_mm")
	}
	if cfg.RimCenterMM == nil {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, "rim_center_mm")
	}
	if cfg.TiltDegs < 0 || cfg.TiltDegs > 180 {
		return nil, nil, fmt.Errorf("%s: tilt_degs must be in (0, 180] if set", path)
	}
	if cfg.StepDegs < 0 || cfg.VelDegsPerSec < 0 || cfg.AccDegsPerSec2 < 0 {
		return nil, nil, fmt.Errorf("%s: step_degs, vel_degs_per_sec and acc_degs_per_sec2 must be > 0 if set", path)
	}
	return []string{resource.NewName(generic.API, cfg.Bartender).String()}, nil, nil
}

type pourSwitch struct {
	resource.AlwaysRebuild
	resource.TriviallyCloseable

	name      resource.Name
	logger    logging.Logger
	pourCmd   map[string]any
	bartender resource.Resource
	executing atomic.Bool
}

func newPourSwitch(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (toggleswitch.Switch, error) {
	conf, err := resource.NativeConfig[*Config](rawConf)
	if err != nil {
		return nil, err
	}
	bartender, ok := deps[resource.NewName(generic.API, conf.Bartender)]
	if !ok {
		return nil, fmt.Errorf("bartender service %q not found in dependencies", conf.Bartender)
	}
	return &pourSwitch{
		name:      rawConf.ResourceName(),
		logger:    logger,
		pourCmd:   pourCommand(conf),
		bartender: bartender,
	}, nil
}

// Omitted optionals fall through to the bartender's defaults.
func pourCommand(conf *Config) map[string]any {
	vec := func(v *Vec) map[string]any { return map[string]any{"x": v.X, "y": v.Y, "z": v.Z} }
	args := map[string]any{
		"lip_mm":        vec(conf.LipMM),
		"rim_center_mm": vec(conf.RimCenterMM),
	}
	if conf.PrePourPose != "" {
		args["pre_pour_pose"] = conf.PrePourPose
	}
	for name, v := range map[string]float64{
		"tilt_degs":         conf.TiltDegs,
		"step_degs":         conf.StepDegs,
		"vel_degs_per_sec":  conf.VelDegsPerSec,
		"acc_degs_per_sec2": conf.AccDegsPerSec2,
	} {
		if v > 0 {
			args[name] = v
		}
	}
	return map[string]any{"pour_about_lip": args}
}

func (s *pourSwitch) Name() resource.Name {
	return s.name
}

func (s *pourSwitch) Status(ctx context.Context) (map[string]any, error) {
	return map[string]any{}, nil
}

func (s *pourSwitch) DoCommand(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	return nil, errors.New("pour-switch has no DoCommands; use SetPosition")
}

func (s *pourSwitch) GetNumberOfPositions(ctx context.Context, extra map[string]any) (uint32, []string, error) {
	return uint32(len(positionNames)), positionNames, nil
}

// Asks the bartender rather than tracking locally, so a failed, cancelled or restarted pour still reads right.
func (s *pourSwitch) GetPosition(ctx context.Context, extra map[string]any) (uint32, error) {
	ctx, span := trace.StartSpan(ctx, "pour-switch::GetPosition")
	defer span.End()
	resp, err := s.bartender.DoCommand(ctx, map[string]any{"get_lip_pour": true})
	if err != nil {
		return 0, fmt.Errorf("query bartender: %w", err)
	}
	if tilted, _ := resp["tilted"].(bool); tilted {
		return positionPour, nil
	}
	return positionUpright, nil
}

func (s *pourSwitch) SetPosition(ctx context.Context, position uint32, extra map[string]any) error {
	ctx, span := trace.StartSpan(ctx, "pour-switch::SetPosition")
	defer span.End()
	if position > positionPour {
		return fmt.Errorf("requested position %d is greater than highest possible position %d", position, positionPour)
	}
	if !s.executing.CompareAndSwap(false, true) {
		return errors.New("switch is currently executing")
	}
	defer s.executing.Store(false)

	current, err := s.GetPosition(ctx, nil)
	if err != nil {
		return err
	}
	if current == position {
		return nil
	}
	cmd := s.pourCmd
	if position == positionUpright {
		cmd = map[string]any{"upright_about_lip": true}
	}
	s.logger.Infof("moving to %q", positionNames[position])
	if _, err := s.bartender.DoCommand(ctx, cmd); err != nil {
		return fmt.Errorf("%s: %w", positionNames[position], err)
	}
	return nil
}
