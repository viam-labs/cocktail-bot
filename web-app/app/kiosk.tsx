"use client";

import { useCallback, useEffect, useState } from "react";
import type { ViamConnection } from "./lib/viamClient";
import {
  getInventory,
  getRecipes,
  getMachineName,
} from "./lib/viamClient";
import type { Recipe, RecipeStep } from "./lib/recipes";
import type { Inventory } from "./lib/inventory";
import { isAvailable } from "./lib/inventory";
import { makeDispatcher, runOrder, stepLabel } from "./lib/orderRunner";
import { Nav } from "./nav";

type OrderState =
  | { kind: "idle" }
  | { kind: "running"; recipe: Recipe; step: RecipeStep; index: number; total: number }
  | { kind: "done"; recipe: Recipe }
  | { kind: "error"; recipe: Recipe; message: string };

export function Kiosk({ conn, connected }: { conn: ViamConnection; connected: boolean }) {
  const [machineName, setMachineName] = useState<string>("");
  const [recipes, setRecipes] = useState<Recipe[] | null>(null);
  const [inventory, setInventory] = useState<Inventory | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [order, setOrder] = useState<OrderState>({ kind: "idle" });

  useEffect(() => {
    let cancelled = false;
    getMachineName(conn)
      .then((n) => !cancelled && setMachineName(n))
      .catch(() => {});
    Promise.all([getRecipes(conn), getInventory(conn)])
      .then(([r, i]) => {
        if (cancelled) return;
        setRecipes(r);
        setInventory(i);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setLoadError(err instanceof Error ? err.message : String(err));
      });
    return () => {
      cancelled = true;
    };
  }, [conn]);

  const startOrder = useCallback(
    async (recipe: Recipe) => {
      const dispatch = makeDispatcher(conn);
      try {
        await runOrder(recipe, dispatch, ({ index, total, step }) => {
          setOrder({ kind: "running", recipe, step, index, total });
        });
        setOrder({ kind: "done", recipe });
      } catch (err) {
        setOrder({
          kind: "error",
          recipe,
          message: err instanceof Error ? err.message : String(err),
        });
      }
    },
    [conn],
  );

  const resetOrder = useCallback(() => setOrder({ kind: "idle" }), []);

  return (
    <>
      <Nav current="kiosk" />
      <main className="min-h-screen flex flex-col items-center p-8 gap-8">
      <header className="flex flex-col items-center gap-2">
        <h1 className="text-4xl font-semibold tracking-tight">Cocktails</h1>
        <p className={`text-sm ${connected ? "text-green-700" : "text-amber-700"}`}>
          {connected ? "Connected" : "Reconnecting"} to {machineName || conn.hostname}
          {conn.isDev && " (dev mock)"}
        </p>
      </header>

      {loadError ? (
        <p className="text-red-600">Could not load menu: {loadError}</p>
      ) : !recipes || !inventory ? (
        <p className="text-gray-600">Loading menu…</p>
      ) : recipes.length === 0 ? (
        <p className="text-gray-600">No recipes configured.</p>
      ) : (
        <DrinkGrid
          recipes={recipes}
          inventory={inventory}
          disabled={order.kind === "running"}
          onOrder={startOrder}
        />
      )}

      {order.kind === "running" && <OrderProgress order={order} />}
      {order.kind === "done" && <OrderDone recipe={order.recipe} onDismiss={resetOrder} />}
      {order.kind === "error" && (
        <OrderError recipe={order.recipe} message={order.message} onDismiss={resetOrder} />
      )}
      </main>
    </>
  );
}

function DrinkGrid({
  recipes,
  inventory,
  disabled,
  onOrder,
}: {
  recipes: Recipe[];
  inventory: Inventory;
  disabled: boolean;
  onOrder: (recipe: Recipe) => void;
}) {
  return (
    <ul className="grid gap-4 w-full max-w-3xl grid-cols-[repeat(auto-fill,minmax(260px,1fr))]">
      {recipes.map((recipe) => {
        const available = isAvailable(recipe, inventory);
        return (
          <li
            key={recipe.id}
            className="border border-gray-200 rounded-xl p-6 flex flex-col gap-4 bg-white"
          >
            <h2 className="text-xl font-semibold">{recipe.name}</h2>
            <button
              type="button"
              onClick={() => onOrder(recipe)}
              disabled={disabled || !available}
              className="h-12 rounded-lg bg-black text-white font-medium disabled:bg-gray-200 disabled:text-gray-400"
            >
              {available ? "Order" : "Out of stock"}
            </button>
          </li>
        );
      })}
    </ul>
  );
}

function OrderProgress({
  order,
}: {
  order: { recipe: Recipe; step: RecipeStep; index: number; total: number };
}) {
  return (
    <section className="fixed inset-x-0 bottom-0 bg-black text-white px-6 py-5 flex items-center justify-between gap-6">
      <div className="flex flex-col gap-1">
        <strong className="text-sm uppercase tracking-wide text-gray-400">
          Making {order.recipe.name}
        </strong>
        <span className="text-lg">{stepLabel(order.step)}</span>
      </div>
      <span className="text-sm text-gray-400">
        Step {order.index + 1} of {order.total}
      </span>
    </section>
  );
}

function OrderDone({ recipe, onDismiss }: { recipe: Recipe; onDismiss: () => void }) {
  return (
    <section className="fixed inset-0 bg-black/60 flex items-center justify-center p-8">
      <div className="bg-white rounded-xl p-8 flex flex-col gap-4 max-w-sm w-full">
        <h2 className="text-2xl font-semibold">{recipe.name} is ready</h2>
        <button
          type="button"
          onClick={onDismiss}
          className="h-12 rounded-lg bg-black text-white font-medium"
        >
          Done
        </button>
      </div>
    </section>
  );
}

function OrderError({
  recipe,
  message,
  onDismiss,
}: {
  recipe: Recipe;
  message: string;
  onDismiss: () => void;
}) {
  return (
    <section className="fixed inset-0 bg-black/60 flex items-center justify-center p-8">
      <div className="bg-white rounded-xl p-8 flex flex-col gap-4 max-w-sm w-full">
        <h2 className="text-2xl font-semibold text-red-700">
          {recipe.name} failed
        </h2>
        <p className="text-sm text-gray-700 break-words">{message}</p>
        <button
          type="button"
          onClick={onDismiss}
          className="h-12 rounded-lg bg-black text-white font-medium"
        >
          Dismiss
        </button>
      </div>
    </section>
  );
}
