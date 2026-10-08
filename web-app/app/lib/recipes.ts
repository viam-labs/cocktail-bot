export interface Recipe {
  id: string;
  name: string;
  pours: Pour[];
  on_menu?: boolean;
}

export interface Pour {
  ingredient: string;
  oz: number;
  tol_oz?: number;
}

export function recipeIngredients(recipe: Recipe): string[] {
  const seen = new Set<string>();
  for (const pour of recipe.pours) {
    if (pour.ingredient) seen.add(pour.ingredient);
  }
  return [...seen];
}
