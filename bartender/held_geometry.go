package bartender

import (
	"errors"
	"fmt"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

type HeldBottleGeometry struct {
	Type      string  `json:"type"`
	RadiusMM  float64 `json:"radius_mm"`
	LengthMM  float64 `json:"length_mm"`
	ZOffsetMM float64 `json:"z_offset_mm"`
}

func (g *HeldBottleGeometry) Validate(path string) error {
	if g == nil {
		return nil
	}
	if g.Type != "cylinder" {
		return fmt.Errorf("%s.type must be \"cylinder\" (only cylinder supported now), got %q", path, g.Type)
	}
	if g.RadiusMM <= 0 || g.LengthMM <= 0 {
		return fmt.Errorf("%s.radius_mm and length_mm must be > 0", path)
	}
	return nil
}

const heldBottleFrameName = "held-bottle"

// Returns (nil, nil) when g is nil so callers can call unconditionally.
func buildHeldBottleFrame(g *HeldBottleGeometry) (referenceframe.Frame, error) {
	if g == nil {
		return nil, nil
	}
	geom, err := spatialmath.NewCapsule(
		spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: -g.ZOffsetMM}, &spatialmath.OrientationVectorDegrees{OZ: 1}),
		g.RadiusMM, g.LengthMM, heldBottleFrameName,
	)
	if err != nil {
		return nil, fmt.Errorf("build held-bottle geometry: %w", err)
	}
	return referenceframe.NewStaticFrameWithGeometry(heldBottleFrameName, spatialmath.NewZeroPose(), geom)
}

// No-op when HeldBottleGeometry is nil so pickup code doesn't branch on config.
func (b *bartender) attachHeldBottle() error {
	if b.cfg.HeldBottleGeometry == nil {
		return nil
	}
	if b.heldGeomFrame != nil {
		return errors.New("attachHeldBottle: already holding a bottle; detach first")
	}
	frame, err := buildHeldBottleFrame(b.cfg.HeldBottleGeometry)
	if err != nil {
		return err
	}
	b.heldGeomFrame = frame
	return nil
}

// Idempotent so release-the-bottle paths don't branch on held-state.
func (b *bartender) detachHeldBottle() {
	b.heldGeomFrame = nil
}
