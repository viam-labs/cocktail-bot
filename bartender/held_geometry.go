package bartender

import (
	"errors"
	"fmt"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

type HeldObjectGeometry struct {
	Type      string  `json:"type"`
	RadiusMM  float64 `json:"radius_mm"`
	LengthMM  float64 `json:"length_mm"`
	ZOffsetMM float64 `json:"z_offset_mm"`
}

func (g *HeldObjectGeometry) Validate(path string) error {
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

const heldObjectFrameName = "held-object"

// Returns (nil, nil) when g is nil so callers can call unconditionally.
func buildHeldObjectFrame(g *HeldObjectGeometry) (referenceframe.Frame, error) {
	if g == nil {
		return nil, nil
	}
	geom, err := spatialmath.NewCapsule(
		spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: -g.ZOffsetMM}, &spatialmath.OrientationVectorDegrees{OZ: 1}),
		g.RadiusMM, g.LengthMM, heldObjectFrameName,
	)
	if err != nil {
		return nil, fmt.Errorf("build held-object geometry: %w", err)
	}
	return referenceframe.NewStaticFrameWithGeometry(heldObjectFrameName, spatialmath.NewZeroPose(), geom)
}

// No-op when g is nil so caller code doesn't branch on config.
func (b *bartender) attachHeld(g *HeldObjectGeometry) error {
	if g == nil {
		return nil
	}
	if b.heldGeomFrame != nil {
		return errors.New("attachHeld: already holding an object; detach first")
	}
	frame, err := buildHeldObjectFrame(g)
	if err != nil {
		return err
	}
	b.heldGeomFrame = frame
	return nil
}

// Idempotent so release paths don't branch on held-state.
func (b *bartender) detachHeld() {
	b.heldGeomFrame = nil
}
