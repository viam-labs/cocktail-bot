import { test } from "node:test";
import assert from "node:assert/strict";
import { planStockingAction, slotLabel } from "./stocking";
import type { Inventory, Slot } from "./inventory";

function inv(ingredients: Record<string, { slot?: string }>): Inventory {
  const full: Inventory["ingredients"] = {};
  for (const [name, { slot }] of Object.entries(ingredients)) {
    full[name] = { in_stock: true, slot };
  }
  return { ingredients: full };
}

const emptySlot: Slot = { id: "slot-g-bottom-front" };
const occupiedSlot: Slot = { id: "slot-g-bottom-front", ingredient: "aged-rum" };

test("planStockingAction: place (empty slot, new bottle not on rack)", () => {
  const plan = planStockingAction(emptySlot, "midori", inv({ midori: {} }));
  assert.deepEqual(plan, {
    action: "place",
    targetSlot: "slot-g-bottom-front",
    ingredient: "midori",
  });
});

test("planStockingAction: move (empty slot, bottle already at another slot)", () => {
  const plan = planStockingAction(
    emptySlot,
    "midori",
    inv({ midori: { slot: "slot-a-bottom-front" } }),
  );
  assert.deepEqual(plan, {
    action: "move",
    targetSlot: "slot-g-bottom-front",
    ingredient: "midori",
    sourceSlot: "slot-a-bottom-front",
  });
});

test("planStockingAction: retrieve (clear an occupied slot)", () => {
  const plan = planStockingAction(
    occupiedSlot,
    "",
    inv({ "aged-rum": { slot: "slot-g-bottom-front" } }),
  );
  assert.deepEqual(plan, {
    action: "retrieve",
    targetSlot: "slot-g-bottom-front",
    removedIngredient: "aged-rum",
  });
});

test("planStockingAction: swap (occupied slot, new bottle not on rack)", () => {
  const plan = planStockingAction(
    occupiedSlot,
    "midori",
    inv({ midori: {}, "aged-rum": { slot: "slot-g-bottom-front" } }),
  );
  assert.deepEqual(plan, {
    action: "swap",
    targetSlot: "slot-g-bottom-front",
    ingredient: "midori",
    sourceSlot: "slot-g-bottom-front",
    removedIngredient: "aged-rum",
  });
});

test("planStockingAction: errors when clearing an already-empty slot", () => {
  assert.throws(() => planStockingAction(emptySlot, "", inv({})), /already empty/);
});

test("planStockingAction: errors when the chosen bottle is already in this slot", () => {
  assert.throws(
    () =>
      planStockingAction(
        occupiedSlot,
        "aged-rum",
        inv({ "aged-rum": { slot: "slot-g-bottom-front" } }),
      ),
    /already in/,
  );
});

test("planStockingAction: errors when slot is occupied AND new bottle is already elsewhere", () => {
  assert.throws(
    () =>
      planStockingAction(
        occupiedSlot,
        "midori",
        inv({
          midori: { slot: "slot-a-bottom-front" },
          "aged-rum": { slot: "slot-g-bottom-front" },
        }),
      ),
    /already at slot-a-bottom-front/,
  );
});

test("slotLabel: bottom-back", () => {
  assert.equal(slotLabel("slot-j-bottom-back"), "Bottom back J");
});

test("slotLabel: top-front", () => {
  assert.equal(slotLabel("slot-a-top-front"), "Top front A");
});

test("slotLabel: single-row", () => {
  assert.equal(slotLabel("slot-e-front"), "Front E");
});

test("slotLabel: no row suffix", () => {
  assert.equal(slotLabel("slot-e"), "E");
});
