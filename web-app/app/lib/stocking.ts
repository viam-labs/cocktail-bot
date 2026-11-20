import type { Inventory, Slot } from "./inventory";
import { RACK, parseSlotId } from "./slots";

export type StockingAction = "place" | "move" | "retrieve" | "swap";

// slotLabel formats "slot-j-bottom-back" as "Bottom back J" to match how slots
// are titled on the inventory page, and "fridge-a-top-front" as "Fridge top front A".
export function slotLabel(id: string): string {
  const { place, letter, shelf, depth } = parseSlotId(id);
  const row = [place === RACK ? "" : place, shelf, ...depth.split("-")]
    .filter(Boolean)
    .join(" ")
    .replace(/^./, (c) => c.toUpperCase());
  return row ? `${row} ${letter}` : letter;
}

export interface StockingPlan {
  action: StockingAction;
  targetSlot: string;
  // ingredient being placed in targetSlot at the end of this plan.
  // undefined for retrieve.
  ingredient?: string;
  // for move: the source slot the bottle currently lives in.
  // for swap: the slot the old bottle currently occupies (same as targetSlot).
  sourceSlot?: string;
  // for swap: the old ingredient being removed to make room.
  // for retrieve: the ingredient being taken off the rack.
  removedIngredient?: string;
}

// planStockingAction decides which of the four cases the UI should run when
// a user picks `choice` in the slot panel. `choice` is either an ingredient
// name or "" for the "Empty slot" / "Clear slot" selection.
export function planStockingAction(
  slot: Slot,
  choice: string,
  inventory: Inventory,
): StockingPlan {
  const current = slot.ingredient;

  if (choice === "") {
    if (!current) {
      throw new Error(`slot ${slot.id} is already empty`);
    }
    return {
      action: "retrieve",
      targetSlot: slot.id,
      removedIngredient: current,
    };
  }

  if (choice === current) {
    throw new Error(`${choice} is already in ${slot.id}`);
  }

  const existingSlot = inventory.ingredients[choice]?.slot;

  if (!current && !existingSlot) {
    return { action: "place", targetSlot: slot.id, ingredient: choice };
  }

  if (!current && existingSlot) {
    return {
      action: "move",
      targetSlot: slot.id,
      ingredient: choice,
      sourceSlot: existingSlot,
    };
  }

  if (current && !existingSlot) {
    return {
      action: "swap",
      targetSlot: slot.id,
      ingredient: choice,
      sourceSlot: slot.id,
      removedIngredient: current,
    };
  }

  // Both the destination is occupied AND the new bottle is already on the rack.
  // Not supported in v1 — the UI should steer the user to retrieve one first.
  throw new Error(
    `${choice} is already at ${existingSlot}; retrieve it or clear ${slot.id} first`,
  );
}
