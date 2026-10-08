export interface Recipe {
  id: string;
  name: string;
  steps: RecipeStep[];
}

export interface RecipeStep {
  verb: string;
  bottle?: string;
  station?: string;
  source?: string;
  strain_flow?: string;
  oz?: number;
  pour_ms?: number;
  dwell_ms?: number;
  drain_dwell_ms?: number;
  dump_dwell_ms?: number;
}

export function recipeIngredients(recipe: Recipe): string[] {
  const seen = new Set<string>();
  for (const step of recipe.steps) {
    if (step.bottle) seen.add(step.bottle);
  }
  return [...seen];
}
