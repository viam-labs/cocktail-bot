package bartender

import (
	"context"
	"errors"
	"fmt"
)

// makeCocktail walks a recipe end-to-end: for each pour, pours the configured
// ounces of the ingredient into the shaker; then runs the fixed suffix of
// dispense_ice → mix → strain → serve → rotate using the stations and dwell
// times from Config.RecipeDefaults. Fails fast with the phase and underlying
// error; the arm stays wherever it ended up and the operator recovers manually.
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
	displayName := recipe.Name
	if displayName == "" {
		displayName = recipe.ID
	}
	b.status.begin(displayName)
	defer b.status.end()

	for i, pour := range recipe.Pours {
		if pour.Ingredient == "" {
			return fmt.Errorf("pour %d: ingredient is required", i+1)
		}
		b.status.setPhase(fmt.Sprintf("Pouring %s (%d of %d)", pour.Ingredient, i+1, len(recipe.Pours)))
		if err := b.pourIntoShaker(ctx, pour.Ingredient, pour.Oz); err != nil {
			return fmt.Errorf("pour %d (%s): %w", i+1, pour.Ingredient, err)
		}
	}
	b.status.setPhase("Dispensing ice")
	if err := b.dispenseIce(ctx, d.IceStation, d.IceDwellMs); err != nil {
		return fmt.Errorf("dispense ice: %w", err)
	}
	b.status.setPhase("Mixing")
	if err := b.mix(ctx, d.MixerStation, d.MixDwellMs); err != nil {
		return fmt.Errorf("mix: %w", err)
	}
	b.status.setPhase("Straining")
	if err := b.strainShaker(ctx, strainShakerRequest{
		source:       d.ShakerSource,
		strainFlow:   d.StrainFlow,
		drainDwellMs: d.StrainDrainMs,
		dumpDwellMs:  d.StrainDumpMs,
	}); err != nil {
		return fmt.Errorf("strain: %w", err)
	}
	b.status.setPhase("Serving")
	if err := b.pourFromShaker(ctx, d.ServeStation, d.ServeMs); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	b.status.setPhase("Resetting")
	if err := b.rotateShakers(ctx, d.StrainFlow); err != nil {
		return fmt.Errorf("rotate: %w", err)
	}
	return nil
}
