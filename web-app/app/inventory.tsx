"use client";

import { useCallback, useEffect, useState } from "react";
import type { ViamConnection } from "./lib/viamClient";
import { getInventory, updateInventoryItem } from "./lib/viamClient";
import type { Inventory } from "./lib/inventory";
import { Nav } from "./nav";

export function InventoryPage({ conn }: { conn: ViamConnection }) {
  const [inventory, setInventory] = useState<Inventory | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [newName, setNewName] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(() => {
    getInventory(conn)
      .then(setInventory)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : String(err)));
  }, [conn]);

  useEffect(() => {
    load();
  }, [load]);

  async function toggle(ingredient: string, inStock: boolean) {
    setBusy(true);
    setError(null);
    try {
      await updateInventoryItem(conn, ingredient, inStock);
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function addIngredient() {
    const name = newName.trim();
    if (!name) return;
    setBusy(true);
    setError(null);
    try {
      await updateInventoryItem(conn, name, true);
      setNewName("");
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  const items = inventory ? Object.entries(inventory.ingredients).sort(([a], [b]) => a.localeCompare(b)) : [];

  return (
    <>
      <Nav current="inventory" />
      <main className="max-w-3xl mx-auto p-6 flex flex-col gap-6">
        <h1 className="text-3xl font-semibold tracking-tight">Inventory</h1>

        <section className="flex gap-2">
          <input
            type="text"
            placeholder="New ingredient name"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") addIngredient();
            }}
            className="flex-1 h-11 border border-gray-300 rounded-lg px-4"
          />
          <button
            type="button"
            onClick={addIngredient}
            disabled={busy || !newName.trim()}
            className="h-11 px-5 rounded-lg bg-black text-white font-medium disabled:opacity-50"
          >
            Add
          </button>
        </section>

        {error && <p className="text-red-600 text-sm">{error}</p>}

        {!inventory ? (
          <p className="text-gray-600">Loading…</p>
        ) : items.length === 0 ? (
          <p className="text-gray-600">No ingredients yet. Add one above.</p>
        ) : (
          <ul className="divide-y divide-gray-200 border border-gray-200 rounded-xl overflow-hidden bg-white">
            {items.map(([name, stock]) => (
              <li key={name} className="flex items-center justify-between px-5 h-16">
                <div className="flex items-center gap-3">
                  <span
                    className={`h-2.5 w-2.5 rounded-full ${
                      stock.in_stock ? "bg-green-600" : "bg-gray-300"
                    }`}
                  />
                  <span className="font-medium">{name}</span>
                </div>
                <button
                  type="button"
                  onClick={() => toggle(name, !stock.in_stock)}
                  disabled={busy}
                  className={`h-9 px-4 rounded-lg text-sm font-medium border ${
                    stock.in_stock
                      ? "border-gray-300 text-gray-700 hover:bg-gray-50"
                      : "border-black bg-black text-white"
                  } disabled:opacity-50`}
                >
                  {stock.in_stock ? "In stock" : "Out of stock"}
                </button>
              </li>
            ))}
          </ul>
        )}
      </main>
    </>
  );
}
