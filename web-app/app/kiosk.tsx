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
      setOrder({ kind: "running", recipe });
      try {
        await makeCocktail(conn, recipe.id);
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

  const visible = recipes ? recipes.filter((r) => r.on_menu !== false) : [];

  return (
    <>
      <Nav current="kiosk" />
      <div className={styles.scope}>
        <main className={styles.main}>
          <h1 className={styles.title}>Cocktails</h1>
          <p className={`${styles.sub} ${connected ? styles.ok : styles.warn}`}>
            {connected ? "Connected" : "Reconnecting"} to {machineName || conn.hostname}
            {conn.isDev && " (dev mock)"}
          </p>

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
        </main>

        {order.kind === "running" && <OrderProgress recipe={order.recipe} conn={conn} />}
        {order.kind === "done" && <OrderDone recipe={order.recipe} onDismiss={resetOrder} />}
        {order.kind === "error" && (
          <OrderError recipe={order.recipe} message={order.message} onDismiss={resetOrder} />
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

function fmtElapsed(ms: number): string {
  const total = Math.max(0, Math.round(ms / 1000));
  const m = Math.floor(total / 60);
  const s = total % 60;
  return `${m}m ${String(s).padStart(2, "0")}s`;
}

function OrderProgress({ recipe, conn }: { recipe: Recipe; conn: ViamConnection }) {
  const [status, setStatus] = useState<OrderStatus | null>(null);
  const [now, setNow] = useState<number>(0);

  useEffect(() => {
    let cancelled = false;
    const poll = () => {
      getStatus(conn)
        .then((s) => {
          if (!cancelled) setStatus(s);
        })
        .catch(() => {});
    };
    poll();
    const statusId = window.setInterval(poll, 1000);
    const tickId = window.setInterval(() => setNow(Date.now()), 500);
    return () => {
      cancelled = true;
      window.clearInterval(statusId);
      window.clearInterval(tickId);
    };
  }, [conn]);

  const started = status?.started_at ? new Date(status.started_at).getTime() : null;
  const elapsedMs = started ? Math.max(0, now - started) : status?.elapsed_ms ?? 0;
  const phase = status?.current_step || "Starting up";

  return (
    <div className={styles.scrim}>
      <div className={styles.sheet}>
        <h3>
          <span className={styles.spinner} /> Making {recipe.name}
        </h3>
        <p>{phase}</p>
        <p className={styles.elapsed}>{fmtElapsed(elapsedMs)}</p>
      </div>
    </div>
  );
}

function OrderDone({ recipe, onDismiss }: { recipe: Recipe; onDismiss: () => void }) {
  return (
    <div className={styles.scrim}>
      <div className={styles.sheet}>
        <h3>{recipe.name} is ready</h3>
        <p>Enjoy.</p>
        <div className={styles.btns}>
          <button type="button" className={`${styles.btn} ${styles.btnMain}`} onClick={onDismiss}>
            Done
          </button>
        </div>
      </div>
    </div>
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
