package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/golang/geo/r3"
	"github.com/spf13/cobra"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"

	"github.com/viam-labs/cocktail-bot/glassfinder"
)

type HoverGlassFlags struct {
	MachineAddress string
	Camera         string
	Detector       string
	Labels         []string
	MinConfidence  float64
	Component      string
	Motion         string
	HoverZMM       float64
	DryRun         bool
	SavePCD        string
}

var hoverGlassFlags HoverGlassFlags

var hoverGlassCmd = &cobra.Command{
	Use:   "hover-glass",
	Short: "Locate the glass with the glass finder and hover the tool above it",
	Long: `Runs the glass finder against the live camera, takes the world-frame (x, y) centroid of the
highest-confidence glass, and moves --component straight down-facing to (x, y, --hover-z-mm).
Use --dry-run first to print the target without moving the arm.

Examples:
  cocktail-cli hover-glass --machine-address bartender-main.xxxx.viam.cloud \
    --camera cam --component gripper --dry-run --save-pcd glass.pcd`,
	RunE: func(_ *cobra.Command, _ []string) error {
		return runHoverGlass(hoverGlassFlags)
	},
}

func init() {
	f := hoverGlassCmd.Flags()
	f.StringVar(&hoverGlassFlags.MachineAddress, "machine-address", "", "[required] address of the machine (e.g. part-xxxx.viam.cloud)")
	f.StringVar(&hoverGlassFlags.Camera, "camera", "", "[required] RGB-D camera the detector runs on; must be in the frame system")
	f.StringVar(&hoverGlassFlags.Detector, "detector", "yolov8", "2D detector vision service")
	f.StringSliceVar(&hoverGlassFlags.Labels, "labels", nil, "detector labels counted as a glass (default: wine glass, cup)")
	f.Float64Var(&hoverGlassFlags.MinConfidence, "min-confidence", 0, "minimum detection score (default 0.5)")
	f.StringVar(&hoverGlassFlags.Component, "component", "", "[required] component/frame to hover above the glass")
	f.StringVar(&hoverGlassFlags.Motion, "motion", "builtin", "motion service")
	f.Float64Var(&hoverGlassFlags.HoverZMM, "hover-z-mm", 300, "world-frame z of the hover pose in mm")
	f.BoolVar(&hoverGlassFlags.DryRun, "dry-run", false, "print the target pose without moving")
	f.StringVar(&hoverGlassFlags.SavePCD, "save-pcd", "", "write the glass's world-frame point cloud to this path")
}

func (f *HoverGlassFlags) validate() error {
	if f.MachineAddress == "" {
		return errors.New("hover-glass: --machine-address is required")
	}
	if f.Camera == "" {
		return errors.New("hover-glass: --camera is required")
	}
	if f.Component == "" && !f.DryRun {
		return errors.New("hover-glass: --component is required unless --dry-run")
	}
	return nil
}

func runHoverGlass(flags HoverGlassFlags) error {
	if err := flags.validate(); err != nil {
		return err
	}
	logger := newLogger("hover-glass")
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
	if len(glasses) == 0 {
		return errors.New("hover-glass: no glass found")
	}
	for i, g := range glasses {
		logger.Infof("glass %d: label=%q score=%.2f box=%v points=%d center_world_mm=%v",
			i, g.Detection.Label(), g.Detection.Score(), *g.Detection.BoundingBox(), g.Cloud.Size(), g.Center)
	}
	glass := glasses[0]

	if flags.SavePCD != "" {
		if err := writePCD(flags.SavePCD, glass.Cloud); err != nil {
			return err
		}
		logger.Infof("wrote world-frame glass cloud to %s", flags.SavePCD)
	}

	target := spatialmath.NewPose(
		r3.Vector{X: glass.Center.X, Y: glass.Center.Y, Z: flags.HoverZMM},
		&spatialmath.OrientationVectorDegrees{OZ: -1},
	)
	logger.Infof("hover target (world): %v", spatialmath.PoseToProtobuf(target))
	if flags.DryRun {
		return nil
	}

	mot, err := motion.FromProvider(machine, flags.Motion)
	if err != nil {
		return fmt.Errorf("motion %q: %w", flags.Motion, err)
	}
	if _, err := mot.Move(ctx, motion.MoveReq{
		ComponentName: flags.Component,
		Destination:   referenceframe.NewPoseInFrame(referenceframe.World, target),
	}); err != nil {
		return fmt.Errorf("move %s to %v: %w", flags.Component, spatialmath.PoseToProtobuf(target), err)
	}
	logger.Infof("%s is hovering above the glass", flags.Component)
	return nil
}

func writePCD(path string, pc pointcloud.PointCloud) error {
	f, err := os.Create(path) //nolint:gosec
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	if err := pointcloud.ToPCD(pc, f, pointcloud.PCDBinary); err != nil {
		_ = f.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	return f.Close()
}
