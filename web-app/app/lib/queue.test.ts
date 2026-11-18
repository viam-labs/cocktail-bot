import { test } from "node:test";
import assert from "node:assert/strict";
import { DevQueue, guestErrorMessage, orderProgress, ordersInLine, type QueueOrder, type QueueState } from "./queue";

const order = (id: string, extra: Partial<QueueOrder> = {}): QueueOrder => ({
  id,
  drink: id,
  double: false,
  quantity: 1,
  enqueued_at: "",
  started_at: "",
  raw_step: "",
  completed_at: "",
  ...extra,
});

const state = (s: Partial<QueueState>): QueueState => ({ is_busy: false, count: 0, queue: [], recent: [], ...s });

test("orderProgress reports position among waiting orders", () => {
  const s = state({ current: order("a", { raw_step: "Shaking" }), queue: [order("b"), order("c")] });
  assert.deepEqual(orderProgress(s, "a"), { kind: "making", step: "Shaking" });
  assert.deepEqual(orderProgress(s, "c"), { kind: "waiting", position: 2 });
  assert.equal(ordersInLine(s), 3);
});

test("orderProgress reads outcome from awaiting_pickup or recent", () => {
  const s = state({ awaiting_pickup: order("a", { error: "boom" }), recent: [order("b"), order("a", { error: "boom" })] });
  assert.deepEqual(orderProgress(s, "a"), { kind: "failed", error: "boom" });
  assert.deepEqual(orderProgress(s, "b"), { kind: "done" });
  assert.deepEqual(orderProgress(s, "zzz"), { kind: "unknown" });
});

test("guestErrorMessage hides raw errors", () => {
  assert.match(guestErrorMessage("step 2: context canceled"), /staff member stopped/);
  assert.doesNotMatch(guestErrorMessage("motion plan failed at pose 7"), /pose/);
});

test("DevQueue makes one drink at a time and holds until startNext", () => {
  const q = new DevQueue();
  const a = q.enqueue("A", false, 100, "Martini", 0);
  const b = q.enqueue("B", false, 100, "", 0);
  assert.equal(a.ahead, 0);
  assert.equal(b.ahead, 1);
  assert.equal(q.state(50).current?.id, a.order_id);
  assert.equal(q.state(50).current?.glass, "Martini");
  assert.equal(q.state(50).queue[0].glass, undefined);

  const held = q.state(150);
  assert.equal(held.current, undefined);
  assert.equal(held.awaiting_pickup?.id, a.order_id);
  assert.equal(held.queue[0].id, b.order_id);

  assert.equal(q.startNext(160), true);
  assert.equal(q.state(160).current?.id, b.order_id);
});
