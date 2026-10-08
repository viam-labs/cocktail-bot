package bartender

import (
	"context"
	"errors"
	"fmt"
)

// makeCocktail walks a full recipe end-to-end, dispatching each step to the
// corresponding primitive. Fails fast with the step index and underlying error
// if any step fails; the arm stays wherever it ended up and the operator
// recovers manually. Fetches the recipe from the data store by id, or runs
// the inline recipe when provided.
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
	for i, step := range recipe.Steps {
		if err := b.runRecipeStep(ctx, step); err != nil {
			return fmt.Errorf("step %d (%s): %w", i+1, step.Verb, err)
		}
	}
	return nil
}

func (b *bartender) runRecipeStep(ctx context.Context, step RecipeStep) error {
	switch step.Verb {
	case "pour_into_shaker":
		return b.pourIntoShaker(ctx, step.Bottle, step.Oz)
	case "dispense_ice":
		return b.dispenseIce(ctx, step.Station, step.DwellMs)
	case "mix":
		return b.mix(ctx, step.Station, step.DwellMs)
	case "strain_shaker":
		return b.strainShaker(ctx, strainShakerRequest{
			source:       step.Source,
			strainFlow:   step.StrainFlow,
			drainDwellMs: step.DrainDwellMs,
			dumpDwellMs:  step.DumpDwellMs,
		})
	case "pour_from_shaker":
		return b.pourFromShaker(ctx, step.Station, step.PourMs)
	case "rotate_shakers":
		return b.rotateShakers(ctx, step.StrainFlow)
	default:
		return fmt.Errorf("unknown verb %q", step.Verb)
	}
}
