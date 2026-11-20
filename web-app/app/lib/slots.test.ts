import { test } from "node:test";
import assert from "node:assert/strict";
import { groupByShelf, parseSlotId, placeName } from "./slots";

test("parseSlotId: rack slot", () => {
  assert.deepEqual(parseSlotId("slot-j-bottom-back"), { place: "slot", letter: "J", shelf: "bottom", depth: "back" });
});

test("parseSlotId: fridge slot", () => {
  assert.deepEqual(parseSlotId("fridge-a-top-front"), { place: "fridge", letter: "A", shelf: "top", depth: "front" });
});

test("placeName: blank on the rack", () => {
  assert.equal(placeName("slot"), "");
  assert.equal(placeName("fridge"), "Fridge");
});

test("groupByShelf: fridge slots get their own shelf after the rack's", () => {
  const ids = ["fridge-a-top-front", "slot-a-bottom-back", "slot-b-top-front", "slot-a-top-front", "slot-a-top-back"];
  const groups = groupByShelf(ids.map((id) => ({ id })));
  assert.deepEqual(
    groups.map((g) => [g.place, g.shelf, Object.fromEntries([...g.rows].map(([depth, slots]) => [depth, slots.map((s) => s.id)]))]),
    [
      ["slot", "top", { front: ["slot-b-top-front", "slot-a-top-front"], back: ["slot-a-top-back"] }],
      ["slot", "bottom", { back: ["slot-a-bottom-back"] }],
      ["fridge", "top", { front: ["fridge-a-top-front"] }],
    ],
  );
});
