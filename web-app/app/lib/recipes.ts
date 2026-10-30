export interface Recipe {
  id: string;
  name: string;
  steps?: RecipeStep[];
  on_menu?: boolean;
  image?: string;
  section?: string;
  tag?: string;
  base_spirit?: BaseSpirit | "";
  flavors?: Flavor[];
  sort_order?: number;
  pours?: Pour[];
  crushed_ice?: boolean;
  shake?: boolean;
  strain?: boolean;
}

export const BASE_SPIRITS = [
  "Vodka",
  "Gin",
  "Tequila & Mezcal",
  "Rum",
  "Whiskey",
  "Wine",
  "Liqueurs",
  "No alcohol",
] as const;
export type BaseSpirit = (typeof BASE_SPIRITS)[number];

export const FLAVORS = [
  "Fruity",
  "Sour",
  "Sweet",
  "Strong",
  "Bitter",
  "Coffee",
  "Smoky",
  "Floral",
  "Refreshing",
] as const;
export type Flavor = (typeof FLAVORS)[number];

export const MAX_FLAVORS_PER_RECIPE = 2;

export type RecipeStep =
  | { kind: "pour"; ingredient: string; oz: number }
  | { kind: "ice"; type?: IceType }
  | { kind: "mix"; mode?: MixMode }
  | { kind: "strain" };

export type IceType = "crushed" | "cubed";
export type MixMode = "shake" | "stir";

export const COMMON_RECIPE_SECTIONS = ["Classics", "Fruity", "Halloween specials"];
export const COMMON_RECIPE_TAGS = ["Halloween", "New", "Nicolas"];

export const DRINK_IMAGE_SLUGS = [
  "amaretto-sour",
  "aperol-sour",
  "aperol-spritz",
  "aviation",
  "black-russian",
  "blood-moon",
  "boulevardier",
  "burn-through",
  "cherry-limeade",
  "cosmopolitan",
  "daiquiri",
  "draculas-blood",
  "elderflower-martini",
  "espresso-martini",
  "espresso-shakerato",
  "gimlet",
  "gin-and-tonic",
  "godfather",
  "kamikaze",
  "lemon-drop",
  "manhattan",
  "mango-margarita",
  "margarita",
  "martini",
  "mocktail",
  "moscow-mule",
  "negroni",
  "old-fashioned",
  "old-pal",
  "paloma",
  "passion-fruit-martini",
  "whiskey-neat",
  "whiskey-sour",
  "witchs-brew",
  "witchs-brew-empress",
] as const;

export interface Pour {
  ingredient: string;
  oz: number;
}

export function stepsFor(recipe: Recipe): RecipeStep[] {
  if (recipe.steps && recipe.steps.length > 0) return recipe.steps;
  const out: RecipeStep[] = [];
  if (recipe.crushed_ice ?? true) {
    out.push({ kind: "ice", type: "crushed" });
  }
  for (const pour of recipe.pours ?? []) {
    out.push({ kind: "pour", ingredient: pour.ingredient, oz: pour.oz });
  }
  if (recipe.shake ?? true) {
    out.push({ kind: "mix", mode: "shake" });
  }
  if (recipe.strain ?? true) {
    out.push({ kind: "strain" });
  }
  return out;
}

export function recipePours(recipe: Recipe): { ingredient: string; oz: number }[] {
  return stepsFor(recipe)
    .filter((s): s is Extract<RecipeStep, { kind: "pour" }> => s.kind === "pour")
    .map((s) => ({ ingredient: s.ingredient, oz: s.oz }));
}

export function recipeIngredients(recipe: Recipe): string[] {
  const seen = new Set<string>();
  for (const pour of recipePours(recipe)) {
    if (pour.ingredient) seen.add(pour.ingredient);
  }
  return [...seen];
}
