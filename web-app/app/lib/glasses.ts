import type { RecipeStep } from "./recipes";

export type GlassImage = "highball" | "martini" | "rocks" | "coupe" | "wine";

// Defined in the bartender config's `glasses`; read-only here.
export interface Glass {
  id: string;
  name: string;
  capacity_oz: number;
  image?: GlassImage;
}

// Rough planning numbers, not measurements: shaking or stirring over ice adds melt water, and a drink
// served unstrained keeps the shaker's ice, which takes up about the same room whatever the drink.
export const MELT_FRACTION = 0.25;
export const ICE_IN_GLASS_OZ = 3;

export interface GlassNeed {
  pouredOz: number;
  meltOz: number;
  iceOz: number;
  totalOz: number;
}

// Volume the serving glass has to hold. A double doubles the pours but not the ice, matching the bartender.
export function glassNeed(steps: RecipeStep[], double = false): GlassNeed {
  const poured = steps.reduce(
    (sum, s) => sum + (s.kind === "pour" && Number.isFinite(s.oz) ? s.oz : 0),
    0,
  ) * (double ? 2 : 1);
  const iced = steps.some((s) => s.kind === "ice");
  const mixed = steps.some((s) => s.kind === "mix");
  const strained = steps.some((s) => s.kind === "strain");
  const meltOz = iced && mixed ? poured * MELT_FRACTION : 0;
  const iceOz = iced && !strained ? ICE_IN_GLASS_OZ : 0;
  return { pouredOz: poured, meltOz, iceOz, totalOz: poured + meltOz + iceOz };
}

export function fits(glass: Glass, need: GlassNeed): boolean {
  return glass.capacity_oz + 1e-9 >= need.totalOz;
}

// Mirrors the bartender: only drinks made in the shaker can be doubled.
export function usesShaker(steps: RecipeStep[]): boolean {
  return steps.some((s) => s.kind === "ice" || s.kind === "mix" || s.kind === "strain");
}

export function glassName(glasses: Glass[], id: string | undefined): string {
  if (!id) return "";
  return glasses.find((g) => g.id === id)?.name ?? id;
}
