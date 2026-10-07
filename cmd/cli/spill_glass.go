package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
)

// spillGlass is one entry of the spill-glass-finder's {"find_glasses": {}} DoCommand response.
type spillGlass struct {
	Label              string
	Score              float64
	RimCenterMM        r3.Vector
	RadiusMM           float64
	HeightMM           float64
	TiltDeg            float64
	ReprojectionRMSEPx float64
	Frame              string
}

func parseFindGlassesResponse(resp map[string]interface{}) ([]spillGlass, error) {
	raw, ok := resp["glasses"]
	if !ok {
		return nil, errors.New(`find_glasses response has no "glasses" key`)
	}
	list, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf(`find_glasses "glasses" is %T, want a list`, raw)
	}
	glasses := make([]spillGlass, 0, len(list))
	for i, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("glass %d is %T, want an object", i, item)
		}
		g, err := parseSpillGlass(m)
		if err != nil {
			return nil, fmt.Errorf("glass %d: %w", i, err)
		}
		glasses = append(glasses, g)
	}
	return glasses, nil
}

func parseSpillGlass(m map[string]interface{}) (spillGlass, error) {
	var g spillGlass
	var err error
	if g.Label, err = stringField(m, "label"); err != nil {
		return g, err
	}
	if g.Frame, err = stringField(m, "frame"); err != nil {
		return g, err
	}
	for key, dst := range map[string]*float64{
		"score":                &g.Score,
		"radius_mm":            &g.RadiusMM,
		"height_mm":            &g.HeightMM,
		"tilt_deg":             &g.TiltDeg,
		"reprojection_rmse_px": &g.ReprojectionRMSEPx,
	} {
		if *dst, err = floatField(m, key); err != nil {
			return g, err
		}
	}
	raw, ok := m["rim_center_mm"].([]interface{})
	if !ok || len(raw) != 3 {
		return g, fmt.Errorf(`"rim_center_mm" is %v, want [x, y, z]`, m["rim_center_mm"])
	}
	xyz := make([]float64, 3)
	for i, v := range raw {
		f, ok := v.(float64)
		if !ok {
			return g, fmt.Errorf(`"rim_center_mm"[%d] is %T, want a number`, i, v)
		}
		xyz[i] = f
	}
	g.RimCenterMM = r3.Vector{X: xyz[0], Y: xyz[1], Z: xyz[2]}
	return g, nil
}

func stringField(m map[string]interface{}, key string) (string, error) {
	s, ok := m[key].(string)
	if !ok || s == "" {
		return "", fmt.Errorf("%q is %v, want a non-empty string", key, m[key])
	}
	return s, nil
}

func floatField(m map[string]interface{}, key string) (float64, error) {
	f, ok := m[key].(float64)
	if !ok {
		return 0, fmt.Errorf("%q is %T, want a number", key, m[key])
	}
	return f, nil
}

// findSpillGlass asks the spill-glass-finder service for glasses and returns the highest-scoring one with its rim
// center in the world frame.
func findSpillGlass(ctx context.Context, machine robot.Robot, name string) (spillGlass, r3.Vector, error) {
	svc, err := vision.FromProvider(machine, name)
	if err != nil {
		return spillGlass{}, r3.Vector{}, fmt.Errorf("vision service %q: %w", name, err)
	}
	resp, err := svc.DoCommand(ctx, map[string]interface{}{"find_glasses": map[string]interface{}{}})
	if err != nil {
		return spillGlass{}, r3.Vector{}, fmt.Errorf("%s find_glasses: %w", name, err)
	}
	glasses, err := parseFindGlassesResponse(resp)
	if err != nil {
		return spillGlass{}, r3.Vector{}, err
	}
	if len(glasses) == 0 {
		return spillGlass{}, r3.Vector{}, errors.New("hover-glass: no glass found")
	}
	glass := glasses[0]
	world, err := machine.TransformPose(
		ctx,
		referenceframe.NewPoseInFrame(glass.Frame, spatialmath.NewPoseFromPoint(glass.RimCenterMM)),
		referenceframe.World,
		nil,
	)
	if err != nil {
		return spillGlass{}, r3.Vector{}, fmt.Errorf("transform rim center from %q to world: %w", glass.Frame, err)
	}
	return glass, world.Pose().Point(), nil
}
