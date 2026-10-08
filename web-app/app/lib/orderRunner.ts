import type { Recipe, RecipeStep } from "./recipes";
import type { ViamConnection } from "./viamClient";
import {
  pourIntoShaker,
  dispenseIce,
  mix,
  pourFromShaker,
} from "./viamClient";

export type StepDispatcher = (step: RecipeStep) => Promise<void>;

export type ProgressCallback = (progress: {
  index: number;
  total: number;
  step: RecipeStep;
}) => void;

export async function runOrder(
  recipe: Recipe,
  dispatch: StepDispatcher,
  onProgress: ProgressCallback,
): Promise<void> {
  for (let i = 0; i < recipe.steps.length; i++) {
    const step = recipe.steps[i];
    onProgress({ index: i, total: recipe.steps.length, step });
    await dispatch(step);
  }
}

export function makeDispatcher(conn: ViamConnection): StepDispatcher {
  return async (step) => {
    switch (step.verb) {
      case "pour_into_shaker":
        if (!step.bottle || step.oz == null) {
          throw new Error("pour_into_shaker requires bottle and oz");
        }
        await pourIntoShaker(conn, step.bottle, step.oz);
        return;
      case "dispense_ice":
        if (!step.station || step.dwell_ms == null) {
          throw new Error("dispense_ice requires station and dwell_ms");
        }
        await dispenseIce(conn, step.station, step.dwell_ms);
        return;
      case "mix":
        if (!step.station || step.dwell_ms == null) {
          throw new Error("mix requires station and dwell_ms");
        }
        await mix(conn, step.station, step.dwell_ms);
        return;
      case "pour_from_shaker":
        if (!step.station || step.pour_ms == null) {
          throw new Error("pour_from_shaker requires station and pour_ms");
        }
        await pourFromShaker(conn, step.station, step.pour_ms);
        return;
      default:
        throw new Error(`unknown verb: ${step.verb}`);
    }
  };
}

export function stepLabel(step: RecipeStep): string {
  switch (step.verb) {
    case "pour_into_shaker":
      return `Pouring ${step.oz} oz ${step.bottle}`;
    case "dispense_ice":
      return "Dispensing ice";
    case "mix":
      return "Mixing";
    case "pour_from_shaker":
      return "Pouring into glass";
    default:
      return step.verb;
  }
}
