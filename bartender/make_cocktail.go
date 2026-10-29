package bartender

import (
	"context"
	"errors"
	"fmt"
)

func (b *bartender) makeCocktail(ctx context.Context, drinkID string, recipe *Recipe) error {
	if recipe == nil {
		if drinkID == "" {
			return errors.New("make_cocktail: either drink_id or recipe is required")
		}
		if b.dataStore == nil {
			return errDataStoreNotConfigured
		}
		for _, r := range b.dataStore.Recipes() {
			if r.ID == drinkID {
				rCopy := r
				recipe = &rCopy
				break
			}
		}
		if recipe == nil {
			return fmt.Errorf("make_cocktail: recipe %q not found", drinkID)
		}
	}
	d := b.cfg.RecipeDefaults
	if d == nil {
		return errors.New("make_cocktail: recipe_defaults not configured on bartender service")
	}
	if d.ShakerSource == "" || d.IceStation == "" || d.MixerStation == "" || d.StrainFlow == "" || d.ServeStation == "" {
		return errors.New("make_cocktail: recipe_defaults is missing one of shaker_source, ice_station, mixer_station, strain_flow, serve_station")
	}

	steps := stepsFor(*recipe)
	// A drink that never involves the shaker (no ice/mix/strain) implies every
	// pour goes straight into the serving glass. Otherwise all pours fill the
	// shaker and the shaker gets served at the end.
	usesShaker := false
	mixed := false
	stirred := false
	strained := false
	for _, s := range steps {
		switch s.Kind {
		case "ice", "mix", "strain":
			usesShaker = true
		}
		if s.Kind == "mix" {
			if s.MixMode == "stir" {
				stirred = true
			} else {
				mixed = true
			}
		}
		if s.Kind == "strain" {
			strained = true
		}
	}
	for i, step := range steps {
		switch step.Kind {
		case "pour":
			if step.Ingredient == "" {
				return fmt.Errorf("step %d: pour ingredient is required", i+1)
			}
			b.status.setPhase(fmt.Sprintf("Pouring %s (step %d of %d)", step.Ingredient, i+1, len(steps)))
			if usesShaker {
				if err := b.pourIntoShaker(ctx, step.Ingredient, step.Oz); err != nil {
					return fmt.Errorf("step %d pour %s into shaker: %w", i+1, step.Ingredient, err)
				}
			} else {
				if err := b.pourIntoGlass(ctx, step.Ingredient, step.Oz); err != nil {
					return fmt.Errorf("step %d pour %s into glass: %w", i+1, step.Ingredient, err)
				}
			}
		case "ice":
			iceType := step.IceType
			if iceType == "" {
				iceType = "crushed"
			}
			if iceType != "crushed" {
				return fmt.Errorf("step %d ice type %q: not yet supported", i+1, iceType)
			}
			b.status.setPhase("Dispensing ice")
			if err := b.dispenseIce(ctx, d.IceStation, d.IceDwellMs); err != nil {
				return fmt.Errorf("step %d dispense ice: %w", i+1, err)
			}
		case "mix":
			mode := step.MixMode
			if mode == "" {
				mode = "shake"
			}
			switch mode {
			case "shake":
				b.status.setPhase("Shaking")
				if err := b.mix(ctx, d.MixerStation, d.MixDwellMs); err != nil {
					return fmt.Errorf("step %d shake: %w", i+1, err)
				}
			case "stir":
				b.status.setPhase("Stirring")
				if err := b.stir(ctx, d.stirStation(), d.stirDwellMs()); err != nil {
					return fmt.Errorf("step %d stir: %w", i+1, err)
				}
			default:
				return fmt.Errorf("step %d mix mode %q is not yet supported", i+1, mode)
			}
		case "strain":
			b.status.setPhase("Straining")
			if err := b.strainShaker(ctx, strainShakerRequest{
				source:       d.ShakerSource,
				strainFlow:   d.StrainFlow,
				drainDwellMs: d.StrainDrainMs,
				dumpDwellMs:  d.StrainDumpMs,
			}); err != nil {
				return fmt.Errorf("step %d strain: %w", i+1, err)
			}
		default:
			return fmt.Errorf("step %d: unknown kind %q", i+1, step.Kind)
		}
	}

	// A stirred drink stays in the shaker instead of going through shaker pickup and serve.
	if usesShaker && !stirred {
		// strain leaves shaker-A held and mix ends still holding the shaker
		// above the mixer, so in both cases pourFromShaker gets a shaker
		// already in the gripper. Only when neither ran is the shaker still
		// sitting at shaker-home with contents, and only then do we go pick
		// it up before serving.
		if !mixed && !strained {
			shakerSw, err := b.findSwitch(d.ShakerSource)
			if err != nil {
				return fmt.Errorf("shaker source switch %q: %w", d.ShakerSource, err)
			}
			b.status.setPhase("Picking up shaker")
			if err := b.pickupShaker(ctx, shakerSw, "shaker-home"); err != nil {
				return fmt.Errorf("pickup shaker before serve: %w", err)
			}
		}
		b.status.setPhase("Serving")
		if err := b.pourFromShaker(ctx, d.ServeStation, d.ServeMs); err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		// rotateShakers resets the strain-flow bookkeeping (shaker-B back to
		// shaker-A); only relevant when a strain actually ran.
		if strained {
			b.status.setPhase("Resetting")
			if err := b.rotateShakers(ctx, d.StrainFlow); err != nil {
				return fmt.Errorf("rotate: %w", err)
			}
		}
	}
	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("park at home: %w", err)
	}
	return nil
}
