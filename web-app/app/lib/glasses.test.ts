import { test } from "node:test";
import assert from "node:assert/strict";
import { fits, glassName, glassNeed, ICE_IN_GLASS_OZ, MELT_FRACTION } from "./glasses";
import type { RecipeStep } from "./recipes";

const pours: RecipeStep[] = [
  { kind: "pour", ingredient: "vodka", oz: 2 },
  { kind: "pour", ingredient: "coffee-liquor", oz: 1 },
];

test("strained shaken drink adds melt but no ice", () => {
  const need = glassNeed([{ kind: "ice" }, ...pours, { kind: "mix" }, { kind: "strain" }]);
  assert.equal(need.pouredOz, 3);
  assert.equal(need.meltOz, 3 * MELT_FRACTION);
  assert.equal(need.iceOz, 0);
  assert.equal(need.totalOz, 3 * (1 + MELT_FRACTION));
});

test("unstrained iced drink keeps the ice in the glass", () => {
  const need = glassNeed([{ kind: "ice" }, ...pours, { kind: "mix" }]);
  assert.equal(need.iceOz, ICE_IN_GLASS_OZ);
  assert.equal(need.totalOz, 3 * (1 + MELT_FRACTION) + ICE_IN_GLASS_OZ);
});

test("built drink is just the pours", () => {
  assert.equal(glassNeed(pours).totalOz, 3);
});

test("double doubles the pours but not the ice or its melt", () => {
  const need = glassNeed([{ kind: "ice" }, ...pours, { kind: "mix" }], true);
  assert.equal(need.pouredOz, 6);
  assert.equal(need.meltOz, 3 * MELT_FRACTION);
  assert.equal(need.iceOz, ICE_IN_GLASS_OZ);
  assert.equal(need.totalOz, 6 + 3 * MELT_FRACTION + ICE_IN_GLASS_OZ);
});

test("fits compares capacity to the total need", () => {
  const need = glassNeed(pours);
  assert.equal(fits({ id: "a", name: "A", capacity_oz: 3 }, need), true);
  assert.equal(fits({ id: "b", name: "B", capacity_oz: 2.9 }, need), false);
});

test("glassName falls back to the id", () => {
  const glasses = [{ id: "rocks", name: "Rocks", capacity_oz: 10 }];
  assert.equal(glassName(glasses, "rocks"), "Rocks");
  assert.equal(glassName(glasses, "tiki"), "tiki");
  assert.equal(glassName(glasses, undefined), "");
});
