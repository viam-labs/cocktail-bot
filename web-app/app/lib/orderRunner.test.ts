import { test } from "node:test";
import assert from "node:assert/strict";
import { runOrder, stepLabel } from "./orderRunner";
import type { Recipe, RecipeStep } from "./recipes";

const recipe: Recipe = {
  id: "em",
  name: "Espresso Martini",
  steps: [
    { verb: "pour_into_shaker", bottle: "vodka", oz: 2 },
    { verb: "dispense_ice", station: "ice-station", dwell_ms: 3000 },
    { verb: "mix", station: "mixer", dwell_ms: 10000 },
  ],
};

test("runOrder dispatches every step in order", async () => {
  const dispatched: RecipeStep[] = [];
  await runOrder(
    recipe,
    async (step) => {
      dispatched.push(step);
    },
    () => {},
  );
  assert.deepEqual(
    dispatched.map((s) => s.verb),
    ["pour_into_shaker", "dispense_ice", "mix"],
  );
});

test("runOrder reports progress before each step", async () => {
  const progress: Array<{ index: number; total: number; verb: string }> = [];
  await runOrder(
    recipe,
    async () => {},
    (p) => progress.push({ index: p.index, total: p.total, verb: p.step.verb }),
  );
  assert.deepEqual(progress, [
    { index: 0, total: 3, verb: "pour_into_shaker" },
    { index: 1, total: 3, verb: "dispense_ice" },
    { index: 2, total: 3, verb: "mix" },
  ]);
});

test("runOrder stops on the first failing step and rethrows", async () => {
  const dispatched: RecipeStep[] = [];
  await assert.rejects(
    () =>
      runOrder(
        recipe,
        async (step) => {
          dispatched.push(step);
          if (step.verb === "dispense_ice") throw new Error("ice jam");
        },
        () => {},
      ),
    /ice jam/,
  );
  assert.equal(dispatched.length, 2, "stops after the failing step");
});

test("runOrder with an empty recipe is a no-op", async () => {
  let called = false;
  await runOrder(
    { id: "e", name: "Empty", steps: [] },
    async () => {
      called = true;
    },
    () => {
      called = true;
    },
  );
  assert.equal(called, false);
});

test("stepLabel formats each verb for the UI", () => {
  assert.equal(
    stepLabel({ verb: "pour_into_shaker", bottle: "vodka", oz: 2 }),
    "Pouring 2 oz vodka",
  );
  assert.equal(
    stepLabel({ verb: "dispense_ice", station: "ice-station", dwell_ms: 3000 }),
    "Dispensing ice",
  );
  assert.equal(
    stepLabel({ verb: "mix", station: "mixer", dwell_ms: 10000 }),
    "Mixing",
  );
  assert.equal(
    stepLabel({ verb: "pour_from_shaker", station: "serving", pour_ms: 5000 }),
    "Pouring into glass",
  );
});

test("stepLabel falls back to the raw verb for unknown steps", () => {
  assert.equal(stepLabel({ verb: "garnish" }), "garnish");
});
