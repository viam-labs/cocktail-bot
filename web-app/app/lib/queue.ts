export interface QueueOrder {
  id: string;
  drink: string;
  double: boolean;
  quantity: number;
  glass?: string;
  enqueued_at: string;
  started_at: string;
  raw_step: string;
  completed_at: string;
  error?: string;
}

export interface QueueState {
  is_busy: boolean;
  count: number;
  current?: QueueOrder;
  awaiting_pickup?: QueueOrder;
  awaiting_glass?: QueueOrder;
  queue: QueueOrder[];
  recent: QueueOrder[];
}

export interface PlacedOrder {
  order_id: string;
  title: string;
  glass?: string;
  ahead: number;
}

export type OrderProgress =
  | { kind: "waiting"; position: number }
  | { kind: "making"; step: string }
  | { kind: "done" }
  | { kind: "failed"; error: string }
  | { kind: "unknown" };

// position is 1 for the order that starts next.
export function orderProgress(state: QueueState, id: string): OrderProgress {
  if (state.current?.id === id) return { kind: "making", step: state.current.raw_step };
  const idx = state.queue.findIndex((o) => o.id === id);
  if (idx >= 0) return { kind: "waiting", position: idx + 1 };
  const finished =
    state.awaiting_pickup?.id === id ? state.awaiting_pickup : state.recent.find((o) => o.id === id);
  if (finished) return finished.error ? { kind: "failed", error: finished.error } : { kind: "done" };
  return { kind: "unknown" };
}

// Orders still to be made, including the one in progress.
export function ordersInLine(state: QueueState): number {
  return state.queue.length + (state.current ? 1 : 0);
}

// Raw motion errors are for operators; guests get a short explanation.
export function guestErrorMessage(error: string): string {
  if (/context canceled/i.test(error)) return "A staff member stopped this drink.";
  if (/busy/i.test(error)) return "The bartender is busy with a staff task. Try again in a minute.";
  return "Something went wrong while making it. Ask a staff member for help.";
}

export class DevQueue {
  private pending: { order: QueueOrder; ms: number }[] = [];
  private current: { order: QueueOrder; endsAt: number } | null = null;
  private held: QueueOrder | null = null;
  private glassReadyFor = "";
  private recent: QueueOrder[] = [];
  private seq = 0;

  enqueue(title: string, double: boolean, ms: number, glass = "", now = Date.now()): PlacedOrder {
    this.advance(now);
    const ahead = this.pending.length + (this.current ? 1 : 0);
    const order: QueueOrder = {
      id: `dev-${++this.seq}`,
      drink: title,
      double,
      quantity: 1,
      glass: glass || undefined,
      enqueued_at: new Date(now).toISOString(),
      started_at: "",
      raw_step: "",
      completed_at: "",
    };
    this.pending.push({ order, ms });
    this.advance(now);
    return { order_id: order.id, title, glass: order.glass, ahead };
  }

  startNext(now = Date.now()): boolean {
    const released = this.held !== null || this.awaitingGlass() !== null;
    if (released) {
      this.held = null;
      if (this.pending.length > 0) this.glassReadyFor = this.pending[0].order.id;
    }
    this.advance(now);
    return released;
  }

  remove(id: string): boolean {
    const before = this.pending.length;
    this.pending = this.pending.filter((p) => p.order.id !== id);
    return this.pending.length !== before;
  }

  state(now = Date.now()): QueueState {
    this.advance(now);
    return {
      is_busy: this.current !== null,
      count: this.pending.length + (this.current ? 1 : 0),
      current: this.current ? { ...this.current.order, raw_step: "Pouring" } : undefined,
      awaiting_pickup: this.held ?? undefined,
      awaiting_glass: this.awaitingGlass() ?? undefined,
      queue: this.pending.map((p) => p.order),
      recent: [...this.recent],
    };
  }

  private awaitingGlass(): QueueOrder | null {
    if (this.current || this.held || this.pending.length === 0) return null;
    const next = this.pending[0].order;
    return next.glass && next.id !== this.glassReadyFor ? next : null;
  }

  private advance(now: number) {
    if (this.current && now >= this.current.endsAt) {
      const done = { ...this.current.order, completed_at: new Date(this.current.endsAt).toISOString() };
      this.recent = [...this.recent, done].slice(-10);
      this.held = done;
      this.current = null;
    }
    if (!this.current && !this.held && this.pending.length > 0 && !this.awaitingGlass()) {
      const next = this.pending.shift()!;
      this.glassReadyFor = "";
      this.current = {
        order: { ...next.order, started_at: new Date(now).toISOString() },
        endsAt: now + next.ms,
      };
    }
  }
}
