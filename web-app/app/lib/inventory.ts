import type { Recipe } from "./recipes";
import { recipeIngredients } from "./recipes";

export interface Inventory {
  ingredients: Record<string, IngredientStock>;
  slots?: Slot[];
}

export interface IngredientStock {
  in_stock: boolean;
  unopened?: number;
  refrigerated?: boolean;
  slot?: string;
  pourer_oz?: number;
  category?: string;
  whiskey_profile?: WhiskeyProfile;
}

export interface WhiskeyProfile {
  abv?: number;
  region?: string;
  story?: string;
  notes?: string;
  profile?: number[];
  us?: string;
  style?: string;
}

export const WHISKEY_TRAITS = [
  "Peat",
  "Smoke",
  "Smoothness",
  "Sweetness",
  "Fruitiness",
  "Heat",
  "Body",
] as const;

export interface Slot {
  id: string;
  ingredient?: string;
}

export function isAvailable(recipe: Recipe, inventory: Inventory): boolean {
  for (const ingredient of recipeIngredients(recipe)) {
    if (!inventory.ingredients[ingredient]?.in_stock) return false;
  }
  return true;
}

// Mirrors the bartender's normalizeIngredientName: names that titleCase the same are the same ingredient.
export function normalizeIngredientName(name: string): string {
  return name.toLowerCase().split(/[-_\s]+/).filter(Boolean).join(" ");
}

export function findEquivalentIngredient(name: string, existing: Iterable<string>): string | undefined {
  const want = normalizeIngredientName(name);
  for (const candidate of existing) {
    if (normalizeIngredientName(candidate) === want) return candidate;
  }
  return undefined;
}
