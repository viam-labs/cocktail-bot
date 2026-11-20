package bartender

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// validateSteps catches step problems that would otherwise surface partway
// through a drink, after earlier steps have already poured or iced.
func (b *bartender) validateSteps(steps []RecipeStep) error {
	var problems []string
	for i, step := range steps {
		switch step.Kind {
		case "pour":
			if step.Ingredient == "" {
				problems = append(problems, fmt.Sprintf("step %d: pour ingredient is required", i+1))
				continue
			}
			if _, err := b.resolvePour(step.Ingredient, step.Oz); err != nil {
				problems = append(problems, fmt.Sprintf("%s: %v", step.Ingredient, err))
			}
		case "ice":
			if step.IceType != "" && step.IceType != "crushed" {
				problems = append(problems, fmt.Sprintf("step %d ice type %q: not yet supported", i+1, step.IceType))
			}
		case "mix":
			if step.MixMode != "" && step.MixMode != "shake" && step.MixMode != "stir" {
				problems = append(problems, fmt.Sprintf("step %d mix mode %q is not yet supported", i+1, step.MixMode))
			}
		case "strain":
			if b.cfg.StrainerObstacle == "" {
				problems = append(problems, fmt.Sprintf("step %d strain: strainer_obstacle is not configured", i+1))
			}
			for j, later := range steps[i+1:] {
				if later.Kind != "strain" {
					problems = append(problems, fmt.Sprintf("step %d %s: must come before the strain step, which fits the strainer for serving", i+j+2, later.Kind))
				}
			}
		default:
			problems = append(problems, fmt.Sprintf("step %d: unknown kind %q", i+1, step.Kind))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return errors.New(strings.Join(problems, "; "))
}

func (b *bartender) makeCocktail(ctx context.Context, plan drinkPlan) error {
	d := b.cfg.RecipeDefaults
	if d == nil {
		return errors.New("make_cocktail: recipe_defaults not configured on bartender service")
	}
	if d.ShakerSource == "" || d.IceStation == "" || d.MixerStation == "" || d.StrainFlow == "" || d.ServeStation == "" {
		return errors.New("make_cocktail: recipe_defaults is missing one of shaker_source, ice_station, mixer_station, strain_flow, serve_station")
	}

	steps := plan.steps
	// A drink that never involves the shaker (no ice/mix/strain) implies every
	// pour goes straight into the serving glass. Otherwise all pours fill the
	// shaker and the shaker gets served at the end.
	shaker := usesShaker(steps)
	mixed := false
	strained := false
	for _, s := range steps {
		// A stir leaves the shaker at shaker-home with the gripper empty, so
		// only a shake counts as ending with the shaker held.
		if s.Kind == "mix" && s.MixMode != "stir" {
			mixed = true
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
			b.setPhase(fmt.Sprintf("Pouring %s (step %d of %d)", step.Ingredient, i+1, len(steps)))
			if shaker {
				if err := b.pourIntoShaker(ctx, step.Ingredient, step.Oz); err != nil {
					return fmt.Errorf("step %d pour %s into shaker: %w", i+1, step.Ingredient, err)
				}
			} else {
				if err := b.pourIntoGlass(ctx, step.Ingredient, step.Oz, plan.glass); err != nil {
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
			b.setPhase("Dispensing ice")
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
				b.setPhase("Shaking")
				if err := b.mix(ctx, d.MixerStation, d.MixDwellMs); err != nil {
					return fmt.Errorf("step %d shake: %w", i+1, err)
				}
			case "stir":
				b.setPhase("Stirring")
				if err := b.stir(ctx, d.stirStation(), d.stirDwellMs()); err != nil {
					return fmt.Errorf("step %d stir: %w", i+1, err)
				}
			default:
				return fmt.Errorf("step %d mix mode %q is not yet supported", i+1, mode)
			}
		case "strain":
			b.setPhase("Fitting the strainer")
			if err := b.strain(ctx, d); err != nil {
				return fmt.Errorf("step %d strain: %w", i+1, err)
			}
		default:
			return fmt.Errorf("step %d: unknown kind %q", i+1, step.Kind)
		}
	}

	// Every drink made in the shaker gets served, including stirred ones.
	if shaker {
		// strain leaves the shaker held with the strainer on and mix ends
		// still holding the shaker above the mixer, so in both cases
		// pourFromShaker gets a shaker already in the gripper. Only when
		// neither ran (ice-only or stirred drinks) is the shaker still sitting
		// at shaker-home with contents, and only then do we go pick it up
		// before serving.
		if !mixed && !strained {
			shakerSw, err := b.findSwitch(d.ShakerSource)
			if err != nil {
				return fmt.Errorf("shaker source switch %q: %w", d.ShakerSource, err)
			}
			b.setPhase("Picking up shaker")
			if err := b.pickupShaker(ctx, shakerSw, "shaker-home"); err != nil {
				return fmt.Errorf("pickup shaker before serve: %w", err)
			}
		}
		serveStation, glassAllow := b.glassServe(plan.glass, d.ServeStation)
		serveSw, err := b.findSwitch(serveStation)
		if err != nil {
			return fmt.Errorf("serve station switch %q: %w", serveStation, err)
		}
		b.setPhase(plan.servePhase)
		if err := b.serveFromShaker(ctx, serveSw, d.ServeMs, glassAllow...); err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		if strained {
			if err := b.unstrain(ctx, d); err != nil {
				return err
			}
		}
		if err := b.putShakerAway(ctx, d, strained); err != nil {
			return err
		}
	}
	if _, err := b.moveArmToPose(ctx, poseHome); err != nil {
		return fmt.Errorf("park at home: %w", err)
	}
	return nil
}

// unstrain puts the strainer back on its mount and dumps the ice it held back.
func (b *bartender) unstrain(ctx context.Context, d *RecipeDefaults) error {
	flowSw, err := b.findSwitch(d.StrainFlow)
	if err != nil {
		return fmt.Errorf("strain_flow switch %q: %w", d.StrainFlow, err)
	}
	b.setPhase("Returning the strainer")
	if err := b.returnStrainer(ctx, flowSw); err != nil {
		return fmt.Errorf("return strainer: %w", err)
	}
	b.setPhase("Dumping the ice")
	if err := b.dumpShakerIce(ctx, flowSw, d.StrainDumpMs); err != nil {
		return fmt.Errorf("dump ice: %w", err)
	}
	return nil
}

// putShakerAway carries the served shaker straight to the cleaner when shaker
// cleaning is on, since serving grips it low enough to flip, then returns it home.
// fromGarbage goes through the cleaner prestage first, since the cleaner may not
// be reachable straight from where the ice was dumped. The shaker-home poses live
// on the default serve station, not on a glass's own serve switch.
func (b *bartender) putShakerAway(ctx context.Context, d *RecipeDefaults, fromGarbage bool) error {
	homeSw, err := b.findSwitch(d.ServeStation)
	if err != nil {
		return fmt.Errorf("serve station switch %q: %w", d.ServeStation, err)
	}
	if sc := b.cfg.ShakerCleaning; sc.enabled() {
		cleanerSw, err := b.findSwitch(sc.Station)
		if err != nil {
			return fmt.Errorf("clean shaker: %w", err)
		}
		b.setPhase("Cleaning shaker")
		if fromGarbage && sc.PrestagePose != "" {
			if _, err := b.carryHeld(ctx, cleanerSw, sc.PrestagePose); err != nil {
				return fmt.Errorf("clean shaker: prestage after dumping the ice: %w", err)
			}
		}
		if err := b.cleanHeldShaker(ctx, cleanerSw, cleanShakerRequest{
			powerOnDelayMs: sc.PowerOnDelayMs,
			dwellMs:        sc.DwellMs,
			drainMs:        sc.DrainMs,
		}); err != nil {
			return fmt.Errorf("clean shaker: %w", err)
		}
	}
	if err := b.returnShakerHome(ctx, homeSw); err != nil {
		return fmt.Errorf("return shaker home: %w", err)
	}
	return nil
}
