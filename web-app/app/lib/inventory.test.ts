import { test } from "node:test";
import assert from "node:assert/strict";
import { isAvailable, type Inventory } from "./inventory";
import type { Recipe } from "./recipes";

const inv = (overrides: Record<string, boolean>): Inventory => ({
  ingredients: Object.fromEntries(
    Object.entries(overrides).map(([k, v]) => [k, { in_stock: v }]),
  ),
});

const recipe = (pours: Recipe["pours"]): Recipe => ({
  id: "test",
  name: "Test",
  pours,
});

test("isAvailable is true when every ingredient in the recipe is in stock", () => {
  const r = recipe([
    { ingredient: "vodka", oz: 2 },
    { ingredient: "coffee-liquor", oz: 1 },
  ]);
  assert.equal(isAvailable(r, inv({ vodka: true, "coffee-liquor": true })), true);
});

test("isAvailable is false when any ingredient is out of stock", () => {
  const r = recipe([
    { ingredient: "vodka", oz: 2 },
    { ingredient: "coffee-liquor", oz: 1 },
  ]);
  assert.equal(isAvailable(r, inv({ vodka: true, "coffee-liquor": false })), false);
});

test("isAvailable is false when an ingredient is missing from inventory entirely", () => {
  const r = recipe([{ ingredient: "vodka", oz: 2 }]);
  assert.equal(isAvailable(r, inv({})), false);
});

test("isAvailable is true for a recipe with no pours", () => {
  const r = recipe([]);
  assert.equal(isAvailable(r, inv({})), true);
});

test("isAvailable deduplicates ingredients that appear in multiple pours", () => {
  const r = recipe([
    { ingredient: "vodka", oz: 1 },
    { ingredient: "vodka", oz: 1 },
  ]);
  assert.equal(isAvailable(r, inv({ vodka: true })), true);
  assert.equal(isAvailable(r, inv({ vodka: false })), false);
});

test("isAvailable ignores extra ingredients in inventory not used by the recipe", () => {
  const r = recipe([{ ingredient: "vodka", oz: 1 }]);
  assert.equal(isAvailable(r, inv({ vodka: true, gin: false })), true);
});
