"use client";

import { useCallback, useEffect, useState } from "react";
import type { ViamConnection } from "./lib/viamClient";
import { getInventory, updateInventoryItem } from "./lib/viamClient";
import type { Inventory } from "./lib/inventory";
import { Nav } from "./nav";
import styles from "./inventory.module.css";

function cap(s: string): string {
  return s ? s.charAt(0).toUpperCase() + s.slice(1) : s;
}

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
  const inStock = items.filter(([, s]) => s.in_stock);
  const outOfStock = items.filter(([, s]) => !s.in_stock);

  return (
    <>
      <Nav current="inventory" />
      <div className={styles.scope}>
        <main className={styles.page}>
          <div className={styles.toolbar}>
            <h1 className={styles.pageTitle}>Inventory</h1>
            <div className={styles.addRow}>
              <input
                type="text"
                className={styles.input}
                placeholder="New ingredient"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") addIngredient();
                }}
              />
              <button
                type="button"
                className={`${styles.btn} ${styles.btnMain}`}
                onClick={addIngredient}
                disabled={busy || !newName.trim()}
              >
                Add
              </button>
            </div>
          </div>

          {error && <p className={styles.error}>{error}</p>}

          {!inventory ? (
            <p className={styles.empty}>Loading…</p>
          ) : items.length === 0 ? (
            <p className={styles.empty}>No ingredients yet. Add one above.</p>
          ) : (
            <>
              <StockSection title="In stock" count={inStock.length} items={inStock} inStock busy={busy} onToggle={toggle} />
              <StockSection title="Out of stock" count={outOfStock.length} items={outOfStock} inStock={false} busy={busy} onToggle={toggle} />
            </>
          )}
        </main>
      </div>
    </>
  );
}

function StockSection({
  title,
  count,
  items,
  inStock,
  busy,
  onToggle,
}: {
  title: string;
  count: number;
  items: [string, { in_stock: boolean }][];
  inStock: boolean;
  busy: boolean;
  onToggle: (name: string, next: boolean) => void;
}) {
  if (items.length === 0) return null;
  return (
    <section className={styles.section}>
      <h2>
        {title}
        <span className={styles.sectionCount}>{count}</span>
      </h2>
      <div className={styles.rows}>
        {items.map(([name]) => (
          <div key={name} className={styles.row}>
            <span
              className={styles.dot}
              style={{ background: inStock ? "var(--ok)" : "var(--smoke)" }}
              aria-hidden
            />
            <span className={styles.name}>{cap(name)}</span>
            <button
              type="button"
              className={styles.btn}
              onClick={() => onToggle(name, !inStock)}
              disabled={busy}
            >
              Mark {inStock ? "out of stock" : "in stock"}
            </button>
          </div>
        ))}
      </div>
    </section>
  );
}
