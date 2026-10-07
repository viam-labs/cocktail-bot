package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"go.viam.com/rdk/services/generic"
)

type PourGlassFlags struct {
	MachineAddress string
	Bartender      string
	Bottle         string
	PourMs         int
	FindOnly       bool
	MouthOffsetMM  float64
	// Only sent when the flag is given, so the bartender's pour_mouth_offset_mm applies otherwise.
	MouthOffsetSet bool
}

var pourGlassFlags PourGlassFlags

var pourGlassCmd = &cobra.Command{
	Use:   "pour-glass",
	Short: "Find the glass with the wrist camera, then pick up a bottle and pour into it",
	Long: `Asks the bartender to find the glass and pour into it (DoCommand find_and_pour):

  1. Move to the saved glass-look pose. If no glass is visible, look a few degrees left and
     right, then lower the camera a few mm (still aimed at the same spot) and repeat, until one
     is found.
  2. Read the glass's world (x, y) from the glass finder.
  3. Pick up --bottle and replay the saved pour-approach/pour-tilt poses moved so the bottle
     mouth (--mouth-offset-mm from the gripper, toward the side the bottle leans) is above the
     glass. Pour for --pour-ms and put the bottle back.

--find-only stops after step 2 (DoCommand find_glass). The arm still moves to search, but the
bottle is never touched. Move the glass between runs to check the (x, y) at many positions.

Examples:
  cocktail-cli pour-glass --machine-address bartender-main.xxxx.viam.cloud --find-only
  cocktail-cli pour-glass --machine-address bartender-main.xxxx.viam.cloud --bottle bottle-gin --pour-ms 1500
  cocktail-cli pour-glass --machine-address bartender-main.xxxx.viam.cloud --bottle bottle-gin --pour-ms 1500 --mouth-offset-mm 80`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		pourGlassFlags.MouthOffsetSet = cmd.Flags().Changed("mouth-offset-mm")
		return runPourGlass(pourGlassFlags)
	},
}

func init() {
	f := pourGlassCmd.Flags()
	f.StringVar(&pourGlassFlags.MachineAddress, "machine-address", "", "[required] address of the machine (e.g. part-xxxx.viam.cloud)")
	f.StringVar(&pourGlassFlags.Bartender, "bartender", "bartender", "bartender service")
	f.StringVar(&pourGlassFlags.Bottle, "bottle", "", "[required unless --find-only] bottle station switch to pour from")
	f.IntVar(&pourGlassFlags.PourMs, "pour-ms", 0, "[required unless --find-only] pour duration in ms")
	f.BoolVar(&pourGlassFlags.FindOnly, "find-only", false, "search for the glass and print its position without touching the bottle")
	f.Float64Var(&pourGlassFlags.MouthOffsetMM, "mouth-offset-mm", 100,
		"horizontal distance from the gripper to the bottle mouth while pouring, toward the side the top of the bottle tips; negative flips the side (default: bartender's pour_mouth_offset_mm)")
}

func (f *PourGlassFlags) validate() error {
	if f.MachineAddress == "" {
		return errors.New("pour-glass: --machine-address is required")
	}
	if f.FindOnly {
		return nil
	}
	if f.Bottle == "" {
		return errors.New("pour-glass: --bottle is required unless --find-only")
	}
	if f.PourMs <= 0 {
		return errors.New("pour-glass: --pour-ms must be > 0 unless --find-only")
	}
	return nil
}

func (f *PourGlassFlags) command() map[string]interface{} {
	if f.FindOnly {
		return map[string]interface{}{"find_glass": map[string]interface{}{}}
	}
	args := map[string]interface{}{"bottle": f.Bottle, "pour_ms": f.PourMs}
	if f.MouthOffsetSet {
		args["mouth_offset_mm"] = f.MouthOffsetMM
	}
	return map[string]interface{}{"find_and_pour": args}
}

func runPourGlass(flags PourGlassFlags) error {
	if err := flags.validate(); err != nil {
		return err
	}
	logger := newLogger("pour-glass")
	ctx := context.Background()

	machine, err := dialMachine(ctx, flags.MachineAddress, logger)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := machine.Close(ctx); cerr != nil {
			logger.Warnf("closing machine: %v", cerr)
		}
	}()

	bartender, err := generic.FromProvider(machine, flags.Bartender)
	if err != nil {
		return fmt.Errorf("bartender %q: %w", flags.Bartender, err)
	}
	res, err := bartender.DoCommand(ctx, flags.command())
	if err != nil {
		return err
	}
	if glass, ok := res["glass"].(map[string]interface{}); ok {
		logger.Infof("glass %q at world x=%v y=%v mm (camera lowered %v mm, panned %v°)",
			glass["label"], glass["x"], glass["y"], glass["lower_mm"], glass["pan_deg"])
	}
	logger.Infof("result: %v", res)
	return nil
}
