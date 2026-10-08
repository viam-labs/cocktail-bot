import { test } from "node:test";
import assert from "node:assert/strict";
import { isAvailable, type Inventory } from "./inventory";
import type { Recipe } from "./recipes";

const inv = (overrides: Record<string, boolean>): Inventory => ({
  ingredients: Object.fromEntries(
    Object.entries(overrides).map(([k, v]) => [k, { in_stock: v }]),
  ),
});

const recipe = (steps: Recipe["steps"]): Recipe => ({
  id: "test",
  name: "Test",
  steps,
});

test("isAvailable is true when every bottle in the recipe is in stock", () => {
  const r = recipe([
    { verb: "pour_into_shaker", bottle: "vodka", oz: 2 },
    { verb: "pour_into_shaker", bottle: "coffee-liquor", oz: 1 },
  ]);
  assert.equal(isAvailable(r, inv({ vodka: true, "coffee-liquor": true })), true);
});

test("isAvailable is false when any bottle is out of stock", () => {
  const r = recipe([
    { verb: "pour_into_shaker", bottle: "vodka", oz: 2 },
    { verb: "pour_into_shaker", bottle: "coffee-liquor", oz: 1 },
  ]);
  assert.equal(isAvailable(r, inv({ vodka: true, "coffee-liquor": false })), false);
});

test("isAvailable is false when a bottle is missing from inventory entirely", () => {
  const r = recipe([{ verb: "pour_into_shaker", bottle: "vodka", oz: 2 }]);
  assert.equal(isAvailable(r, inv({})), false);
});

test("isAvailable is true for a recipe with no ingredient-requiring steps", () => {
  const r = recipe([
    { verb: "dispense_ice", station: "ice-station", dwell_ms: 3000 },
    { verb: "mix", station: "mixer", dwell_ms: 10000 },
  ]);
  assert.equal(isAvailable(r, inv({})), true);
});

test("isAvailable deduplicates bottles that appear in multiple steps", () => {
  const r = recipe([
    { verb: "pour_into_shaker", bottle: "vodka", oz: 1 },
    { verb: "pour_into_shaker", bottle: "vodka", oz: 1 },
  ]);
  assert.equal(isAvailable(r, inv({ vodka: true })), true);
  assert.equal(isAvailable(r, inv({ vodka: false })), false);
});

test("isAvailable ignores extra bottles in inventory not used by the recipe", () => {
  const r = recipe([{ verb: "pour_into_shaker", bottle: "vodka", oz: 1 }]);
  assert.equal(isAvailable(r, inv({ vodka: true, gin: false })), true);
});
