package bartender

import (
	"context"
	"errors"
	"fmt"

	"go.viam.com/rdk/referenceframe"
)

const (
	heldShakerAttachmentFrameName   = "held-shaker-attachment"
	parkedShakerAttachmentFrameName = "parked-shaker-attachment"
	shakerPickupKey                 = "shaker"
)

// attachHeldShaker models the shaker just gripped: its capsule, and the attachment clamped to its rim,
// picked up from wherever it stands so it keeps its real bearing around the shaker in every grip.
func (b *bartender) attachHeldShaker(ctx context.Context, g *HeldObjectGeometry) error {
	if err := b.attachHeld(ctx, g); err != nil {
		return err
	}
	if b.cfg.ShakerAttachmentObstacle == "" || g == nil {
		return nil
	}
	if b.heldShakerAttachmentFrame != nil {
		return errors.New("attach shaker attachment: already carrying it")
	}
	standing := b.cfg.ShakerAttachmentObstacle
	if b.parkedShakerAttachmentFrame != nil {
		standing = parkedShakerAttachmentFrameName
	}
	fs, inputs, err := b.currentInputs(ctx)
	if err != nil {
		return err
	}
	parent := fs.Frame(gripPointFrameName)
	if parent == nil {
		parent = fs.Frame(b.cfg.GripperName)
	}
	if parent == nil {
		return fmt.Errorf("attach shaker attachment: no %q or %q frame to carry it from", gripPointFrameName, b.cfg.GripperName)
	}
	frame, err := carriedObstacleFrame(fs, inputs, parent.Name(), standing, heldShakerAttachmentFrameName)
	if err != nil {
		return fmt.Errorf("attach shaker attachment: %w", err)
	}
	b.heldShakerAttachmentFrame = frame
	b.parkedShakerAttachmentFrame = nil
	return nil
}

// setShakerDownAway releases the held shaker somewhere other than shaker-home, leaving its attachment
// modeled where it now stands until the shaker is gripped again.
func (b *bartender) setShakerDownAway(ctx context.Context) error {
	var parked referenceframe.Frame
	if b.heldShakerAttachmentFrame != nil {
		fs, inputs, err := b.currentInputs(ctx)
		if err != nil {
			return err
		}
		parked, err = carriedObstacleFrame(fs, inputs, referenceframe.World, heldShakerAttachmentFrameName, parkedShakerAttachmentFrameName)
		if err != nil {
			return fmt.Errorf("park shaker attachment: %w", err)
		}
	}
	if err := b.gripperOpen(ctx); err != nil {
		return err
	}
	b.parkedShakerAttachmentFrame = parked
	return nil
}

func (b *bartender) detachParkedShakerAttachment() {
	b.parkedShakerAttachmentFrame = nil
}
