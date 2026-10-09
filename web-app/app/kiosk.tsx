"use client";

import { useCallback, useEffect, useState } from "react";
import type { ViamConnection } from "./lib/viamClient";
import {
  getInventory,
  getRecipes,
  getMachineName,
  getStatus,
  makeCocktail,
} from "./lib/viamClient";
import type { OrderStatus } from "./lib/viamClient";
import type { Recipe } from "./lib/recipes";
import type { Inventory } from "./lib/inventory";
import { isAvailable } from "./lib/inventory";
import { Nav } from "./nav";
import styles from "./kiosk.module.css";

type OrderState =
  | { kind: "idle" }
  | { kind: "running"; recipe: Recipe }
  | { kind: "done"; recipe: Recipe; at: number }
  | { kind: "error"; recipe: Recipe; message: string };

const READY_CARD_MS = 60_000;

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
      setOrder({ kind: "running", recipe });
      try {
        await makeCocktail(conn, recipe.id);
        setOrder({ kind: "done", recipe, at: Date.now() });
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

  const dismissError = useCallback(() => setOrder({ kind: "idle" }), []);

  // Auto-fade the "ready" state after READY_CARD_MS so the sidebar returns to idle.
  useEffect(() => {
    if (order.kind !== "done") return;
    const remaining = Math.max(0, READY_CARD_MS - (Date.now() - order.at));
    const t = window.setTimeout(() => setOrder({ kind: "idle" }), remaining);
    return () => window.clearTimeout(t);
  }, [order]);

  const visible = recipes ? recipes.filter((r) => r.on_menu !== false) : [];

  return (
    <>
      <Nav current="kiosk" />
      <div className={styles.scope}>
        <main className={styles.main}>
          <h1 className={styles.title}>What can we make you?</h1>
          <p className={`${styles.sub} ${connected ? styles.ok : styles.warn}`}>
            {connected ? "Connected" : "Reconnecting"} to {machineName || conn.hostname}
            {conn.isDev && " (dev mock)"}
          </p>

          <div className={styles.kBody}>
            <div className={styles.menuWrap}>
              {loadError ? (
                <p className={styles.sub}>Could not load menu: {loadError}</p>
              ) : !recipes || !inventory ? (
                <p className={styles.loading}>Loading menu…</p>
              ) : recipes.length === 0 ? (
                <p className={styles.loading}>No recipes configured.</p>
              ) : visible.length === 0 ? (
                <p className={styles.loading}>No drinks on the menu right now.</p>
              ) : (
                <DrinkGrid
                  recipes={visible}
                  inventory={inventory}
                  disabled={order.kind === "running"}
                  onOrder={startOrder}
                />
              )}
            </div>

            <KioskQueue conn={conn} order={order} />
          </div>
        </main>

        {order.kind === "error" && (
          <OrderError recipe={order.recipe} message={order.message} onDismiss={dismissError} />
        )}
      </div>
    </>
  );
}

function DrinkArt({ slug, name }: { slug?: string; name: string }) {
  if (!slug) return <div className={styles.art} aria-hidden />;
  return (
    <div className={styles.art}>
      <picture>
        <source srcSet={`/drinks/png/dark/${slug}.png`} media="(prefers-color-scheme: dark)" />
        <img src={`/drinks/png/light/${slug}.png`} alt="" width={150} height={150} aria-label={name} />
      </picture>
    </div>
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
    <ul className={styles.menu}>
      {recipes.map((recipe) => {
        const available = isAvailable(recipe, inventory);
        return (
          <li key={recipe.id}>
            <button
              type="button"
              className={`${styles.drink} ${!available ? styles.out : ""}`}
              onClick={() => onOrder(recipe)}
              disabled={disabled || !available}
            >
              <DrinkArt slug={recipe.image} name={recipe.name} />
              <h2>{recipe.name}</h2>
              <p>{available ? `${recipe.pours.length} ingredients` : "Out of stock"}</p>
            </button>
          </li>
        );
      })}
    </ul>
  );
}

function KioskQueue({
  conn,
  order,
}: {
  conn: ViamConnection;
  order: OrderState;
}) {
  const [status, setStatus] = useState<OrderStatus | null>(null);
  const pollActive = order.kind === "running";

  useEffect(() => {
    if (!pollActive) return;
    let cancelled = false;
    const poll = () => {
      getStatus(conn)
        .then((s) => {
          if (!cancelled) setStatus(s);
        })
        .catch(() => {});
    };
    poll();
    const id = window.setInterval(poll, 1000);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [conn, pollActive]);

  const currentPhaseForRunning = pollActive ? status?.current_step : undefined;

  return (
    <aside className={styles.kq} aria-label="Queue">
      <h2>In line</h2>

      {order.kind === "done" && (
        <div className={styles.kqReady} role="status">
          {order.recipe.name} is ready
        </div>
      )}

      {order.kind === "running" ? (
        <div className={styles.rows}>
          <div className={styles.kqRow}>
            <span className={styles.kqName}>{order.recipe.name}</span>
            <span className={styles.kqTag}>Making now</span>
          </div>
          {currentPhaseForRunning && <p className={styles.kqNote}>{currentPhaseForRunning}</p>}
        </div>
      ) : (
        order.kind !== "done" && <p className={styles.kqNote}>Nobody in line.</p>
      )}
    </aside>
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
    <div className={styles.scrim}>
      <div className={styles.sheet}>
        <h3 className={styles.err}>{recipe.name} failed</h3>
        <p>{message}</p>
        <div className={styles.btns}>
          <button type="button" className={`${styles.btn} ${styles.btnMain}`} onClick={onDismiss}>
            Dismiss
          </button>
        </div>
      </div>
    </div>
  );
}
