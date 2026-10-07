package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang/geo/r3"
	"github.com/spf13/cobra"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/generic"
	"go.viam.com/rdk/services/vision"

	"github.com/viam-labs/cocktail-bot/glassfinder"
)

type PourGlassesFlags struct {
	MachineAddress  string
	Bartender       string
	Camera          string
	Detector        string
	Labels          []string
	MinConfidence   float64
	LookPose        string
	Bottle          string
	PourMs          int
	MaxGlasses      int
	MinSeparationMM float64
	DryRun          bool
}

var pourGlassesFlags PourGlassesFlags

var pourGlassesCmd = &cobra.Command{
	Use:   "pour-glasses",
	Short: "Detect every glass in view and pour into each one",
	Long: `Moves the arm to --look-pose through the bartender, detects the glasses with the glass finder
(world-frame (x, y) centroid of each glass's depth points), then asks the bartender to pick up
--bottle once and pour into each glass.

The bartender pours by translating its saved pour-approach/pour-tilt poses in world x, y from
its pour_reference_glass to each detected glass, so every pour keeps the saved height and tilt.

--dry-run detects from wherever the arm currently is and prints the glasses without moving
the arm or calling the bartender. Use it with one glass placed where the saved pour lands to
read the value for pour_reference_glass.

Examples:
  cocktail-cli pour-glasses --machine-address bartender-main.xxxx.viam.cloud --camera cam --dry-run
  cocktail-cli pour-glasses --machine-address bartender-main.xxxx.viam.cloud --camera cam \
    --bottle bottle-gin --pour-ms 1500`,
	RunE: func(_ *cobra.Command, _ []string) error {
		return runPourGlasses(pourGlassesFlags)
	},
}

func init() {
	f := pourGlassesCmd.Flags()
	f.StringVar(&pourGlassesFlags.MachineAddress, "machine-address", "", "[required] address of the machine (e.g. part-xxxx.viam.cloud)")
	f.StringVar(&pourGlassesFlags.Bartender, "bartender", "bartender", "bartender service")
	f.StringVar(&pourGlassesFlags.Camera, "camera", "", "[required] RGB-D (wrist) camera the detector runs on; must be in the frame system")
	f.StringVar(&pourGlassesFlags.Detector, "detector", "yolov8", "2D detector vision service")
	f.StringSliceVar(&pourGlassesFlags.Labels, "labels", nil, "detector labels counted as a glass (default: wine glass, cup)")
	f.Float64Var(&pourGlassesFlags.MinConfidence, "min-confidence", 0, "minimum detection score (default 0.5)")
	f.StringVar(&pourGlassesFlags.LookPose, "look-pose", "home", "saved pose the arm moves to before detecting")
	f.StringVar(&pourGlassesFlags.Bottle, "bottle", "", "[required unless --dry-run] bottle station switch to pour from")
	f.IntVar(&pourGlassesFlags.PourMs, "pour-ms", 0, "[required unless --dry-run] pour duration per glass in ms")
	f.IntVar(&pourGlassesFlags.MaxGlasses, "max-glasses", 0, "pour into at most this many glasses, highest score first (0 = all)")
	f.Float64Var(&pourGlassesFlags.MinSeparationMM, "min-separation-mm", 50, "treat detections closer than this as the same glass")
	f.BoolVar(&pourGlassesFlags.DryRun, "dry-run", false, "detect and print the glasses without moving the arm")
}

func (f *PourGlassesFlags) validate() error {
	if f.MachineAddress == "" {
		return errors.New("pour-glasses: --machine-address is required")
	}
	if f.Camera == "" {
		return errors.New("pour-glasses: --camera is required")
	}
	if f.MaxGlasses < 0 || f.MinSeparationMM < 0 {
		return errors.New("pour-glasses: --max-glasses and --min-separation-mm must be >= 0")
	}
	if f.DryRun {
		return nil
	}
	if f.Bottle == "" {
		return errors.New("pour-glasses: --bottle is required unless --dry-run")
	}
	if f.PourMs <= 0 {
		return errors.New("pour-glasses: --pour-ms must be > 0 unless --dry-run")
	}
	return nil
}

// pickGlassCenters keeps centers (already sorted best first) that are at least minSepMM apart in
// x, y from every kept one, so a glass detected as both "cup" and "wine glass" gets one pour.
func pickGlassCenters(centers []r3.Vector, minSepMM float64, maxGlasses int) []r3.Vector {
	var kept []r3.Vector
	for _, c := range centers {
		if maxGlasses > 0 && len(kept) == maxGlasses {
			break
		}
		duplicate := false
		for _, k := range kept {
			if (r3.Vector{X: c.X - k.X, Y: c.Y - k.Y}).Norm() < minSepMM {
				duplicate = true
				break
			}
		}
		if !duplicate {
			kept = append(kept, c)
		}
	}
	return kept
}

func runPourGlasses(flags PourGlassesFlags) error {
	if err := flags.validate(); err != nil {
		return err
	}
	logger := newLogger("pour-glasses")
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

	var bartender resource.Resource
	if !flags.DryRun {
		bartender, err = generic.FromProvider(machine, flags.Bartender)
		if err != nil {
			return fmt.Errorf("bartender %q: %w", flags.Bartender, err)
		}
		logger.Infof("moving to %q to look for glasses", flags.LookPose)
		if _, err := bartender.DoCommand(ctx, map[string]interface{}{"execute_action": flags.LookPose}); err != nil {
			return fmt.Errorf("move to look pose %q: %w", flags.LookPose, err)
		}
	} else {
		logger.Warnf("dry run: detecting from the arm's current pose, not %q", flags.LookPose)
	}

	conf := &glassfinder.Config{
		CameraName:    flags.Camera,
		DetectorName:  flags.Detector,
		Labels:        flags.Labels,
		MinConfidence: flags.MinConfidence,
	}
	if _, _, err := conf.Validate(""); err != nil {
		return err
	}
	finder, err := glassfinder.New(vision.Named("cli-glass-finder"), conf, machine, logger)
	if err != nil {
		return err
	}
	glasses, _, err := finder.FindGlasses(ctx)
	if err != nil {
		return err
	}
	centers := make([]r3.Vector, len(glasses))
	for i, g := range glasses {
		centers[i] = g.Center
		logger.Infof("detection %d: label=%q score=%.2f box=%v points=%d center_world_mm=(%.1f, %.1f, %.1f)",
			i, g.Detection.Label(), g.Detection.Score(), *g.Detection.BoundingBox(), g.Cloud.Size(), g.Center.X, g.Center.Y, g.Center.Z)
	}
	targets := pickGlassCenters(centers, flags.MinSeparationMM, flags.MaxGlasses)
	if len(targets) == 0 {
		return errors.New("pour-glasses: no glass found")
	}
	payload := make([]interface{}, len(targets))
	for i, c := range targets {
		logger.Infof("glass %d: world x=%.1f y=%.1f mm", i, c.X, c.Y)
		payload[i] = map[string]interface{}{"x": c.X, "y": c.Y}
	}
	if flags.DryRun {
		return nil
	}

	res, err := bartender.DoCommand(ctx, map[string]interface{}{
		"pour_into_glasses": map[string]interface{}{
			"bottle":  flags.Bottle,
			"pour_ms": flags.PourMs,
			"glasses": payload,
		},
	})
	if err != nil {
		return fmt.Errorf("pour_into_glasses: %w", err)
	}
	logger.Infof("poured into %d glasses: %v", len(targets), res)
	return nil
}
