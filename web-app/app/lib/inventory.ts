import type { Recipe } from "./recipes";
import { recipeIngredients } from "./recipes";

export interface Inventory {
  ingredients: Record<string, IngredientStock>;
}

export interface IngredientStock {
  in_stock: boolean;
}

export function isAvailable(recipe: Recipe, inventory: Inventory): boolean {
  for (const ingredient of recipeIngredients(recipe)) {
    if (!inventory.ingredients[ingredient]?.in_stock) return false;
  }
  return true;
}
