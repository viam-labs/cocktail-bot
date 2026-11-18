"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import type { ViamConnection } from "./lib/viamClient";
import {
  getGlasses,
  getInventory,
  getPourMenus,
  getRecipes,
  getMachineName,
  getQueue,
  cancelQueuedOrder,
  makeCocktail,
  makeInlineOrder,
  startNext,
} from "./lib/viamClient";
import type { PlacedOrder, QueueOrder, QueueState } from "./lib/queue";
import { guestErrorMessage, orderProgress, ordersInLine } from "./lib/queue";
import type { BaseSpirit, Flavor, Recipe, RecipeStep } from "./lib/recipes";
import { BASE_SPIRITS, FLAVORS, recipePours } from "./lib/recipes";
import type { Inventory, WhiskeyProfile } from "./lib/inventory";
import { isAvailable, WHISKEY_TRAITS } from "./lib/inventory";
import type { PourMenu, PourServe, PourSize } from "./lib/pour_menus";
import type { Glass, GlassImage } from "./lib/glasses";
import { titleCase } from "./lib/display";
import { GlassIcon } from "./glass_icon";
import { Nav } from "./nav";
import styles from "./kiosk.module.css";

// Orders placed from this screen, so it can confirm them and report failures.
type MyOrder = { id: string; label: string };

type Placed = { label: string; ahead: number; at: number };

type OrderFailure = { label: string; message: string };

type PourEntry = {
  menu: PourMenu;
  ingredient: string;
  profile: WhiskeyProfile | null;
  available: boolean;
};

const WHISKEY_SLUG = "whiskey-neat";

const MINUTES_PER_DRINK = 3;
const QUEUE_POLL_MS = 1500;
const PLACED_TOAST_MS = 8_000;
const FILTER_IDLE_RESET_MS = 30_000;

function spiritForPourMenu(menu: PourMenu): BaseSpirit | null {
  const c = menu.category.trim().toLowerCase();
  if (c === "whiskey") return "Whiskey";
  if (c === "wine") return "Wine";
  return null;
}

function recipeMatchesFilters(
  recipe: Recipe,
  baseFilter: BaseSpirit | "",
  flavorFilters: ReadonlySet<Flavor>,
): boolean {
  if (baseFilter && recipe.base_spirit !== baseFilter) return false;
  if (flavorFilters.size > 0) {
    const have = new Set((recipe.flavors ?? []) as Flavor[]);
    for (const f of flavorFilters) if (!have.has(f)) return false;
  }
  return true;
}

function slugForMenu(menu: PourMenu): string {
  if (menu.illustration) return menu.illustration;
  if (menu.category.toLowerCase() === "whiskey") return WHISKEY_SLUG;
  return "";
}

function defaultSize(menu: PourMenu): PourSize | undefined {
  if (menu.default_size) {
    const match = menu.sizes.find((s) => s.label === menu.default_size);
    if (match) return match;
  }
  return menu.sizes[0];
}

export function Kiosk({ conn, connected }: { conn: ViamConnection; connected: boolean }) {
  const params = useSearchParams();
  const kioskMode = params.get("kiosk") === "1";
  const [machineName, setMachineName] = useState<string>("");
  const [recipes, setRecipes] = useState<Recipe[] | null>(null);
  const [inventory, setInventory] = useState<Inventory | null>(null);
  const [menus, setMenus] = useState<PourMenu[] | null>(null);
  const [glasses, setGlasses] = useState<Glass[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [myOrders, setMyOrdersState] = useState<MyOrder[]>([]);
  const myOrdersRef = useRef<MyOrder[]>([]);
  const setMyOrders = useCallback((next: MyOrder[]) => {
    myOrdersRef.current = next;
    setMyOrdersState(next);
  }, []);
  const [placing, setPlacing] = useState(false);
  const [placed, setPlaced] = useState<Placed | null>(null);
  const [failure, setFailure] = useState<OrderFailure | null>(null);

  // Surface failures of orders placed here; forget orders once they finish or vanish.
  const onQueueState = useCallback(
    (state: QueueState) => {
      const mine = myOrdersRef.current;
      if (mine.length === 0) return;
      const still = mine.filter((o) => {
        const p = orderProgress(state, o.id);
        if (p.kind === "failed") setFailure({ label: o.label, message: p.error });
        return p.kind === "waiting" || p.kind === "making";
      });
      if (still.length !== mine.length) setMyOrders(still);
    },
    [setMyOrders],
  );
  const queue = useQueue(conn, connected, onQueueState);
  const [confirming, setConfirming] = useState<{ recipe: Recipe; orderable: boolean } | null>(null);
  const [pourConfirming, setPourConfirming] = useState<PourEntry | null>(null);
  const [pourPreview, setPourPreview] = useState<PourEntry | null>(null);
  const [baseFilter, setBaseFilter] = useState<BaseSpirit | "">("");
  const [flavorFilters, setFlavorFilters] = useState<Set<Flavor>>(() => new Set());
  const clearFilters = useCallback(() => {
    setBaseFilter("");
    setFlavorFilters(new Set());
  }, []);
  const toggleFlavor = useCallback((f: Flavor) => {
    setFlavorFilters((prev) => {
      const next = new Set(prev);
      if (next.has(f)) next.delete(f);
      else next.add(f);
      return next;
    });
  }, []);
  const filtersActive = baseFilter !== "" || flavorFilters.size > 0;

  // Reset filters after idle in kiosk mode (same 30s rule as the drink sheet)
  useEffect(() => {
    if (!kioskMode || !filtersActive) return;
    const id = window.setTimeout(clearFilters, FILTER_IDLE_RESET_MS);
    return () => window.clearTimeout(id);
  }, [kioskMode, filtersActive, baseFilter, flavorFilters, clearFilters]);

  useEffect(() => {
    let cancelled = false;
    getMachineName(conn)
      .then((n) => !cancelled && setMachineName(n))
      .catch(() => {});
    // Only used for outlines in the queue; orders still name their glass without it.
    getGlasses(conn)
      .then((g) => !cancelled && setGlasses(g))
      .catch(() => {});
    Promise.all([getRecipes(conn), getInventory(conn), getPourMenus(conn)])
      .then(([r, i, m]) => {
        if (cancelled) return;
        setRecipes(r);
        setInventory(i);
        setMenus(m);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setLoadError(err instanceof Error ? err.message : String(err));
      });
    return () => {
      cancelled = true;
    };
  }, [conn]);

  const placeOrder = useCallback(
    async (label: string, submit: () => Promise<PlacedOrder>) => {
      setPlacing(true);
      try {
        const resp = await submit();
        const title = resp.title || label;
        setMyOrders([...myOrdersRef.current, { id: resp.order_id, label: title }]);
        setPlaced({ label: title, ahead: resp.ahead, at: Date.now() });
        queue.refresh();
      } catch (err) {
        setFailure({ label, message: err instanceof Error ? err.message : String(err) });
      } finally {
        setPlacing(false);
      }
    },
    [queue, setMyOrders],
  );

  const startOrder = useCallback(
    (recipe: Recipe, double: boolean) =>
      placeOrder(recipe.name, () => makeCocktail(conn, recipe.id, { double })),
    [conn, placeOrder],
  );

  const startPourOrder = useCallback(
    (entry: PourEntry, size: PourSize, serve: PourServe) => {
      const label = `${titleCase(entry.ingredient)} · ${size.label} · ${serve.label}`;
      const steps: RecipeStep[] = [];
      if (serve.with_ice) {
        steps.push({ kind: "ice", type: "crushed" });
      }
      steps.push({ kind: "pour", ingredient: entry.ingredient, oz: size.oz });
      // Pour menus predate the glasses config; only send a glass the bartender knows, or it refuses the order.
      const glass = glasses.some((g) => g.id === serve.glass) ? serve.glass : undefined;
      return placeOrder(label, () => makeInlineOrder(conn, { name: label, steps, glass }));
    },
    [conn, placeOrder, glasses],
  );

  useEffect(() => {
    if (!placed) return;
    const t = window.setTimeout(() => setPlaced(null), Math.max(0, PLACED_TOAST_MS - (Date.now() - placed.at)));
    return () => window.clearTimeout(t);
  }, [placed]);

  const onStartNext = useCallback(async () => {
    try {
      await startNext(conn);
    } finally {
      queue.refresh();
    }
  }, [conn, queue]);

  const onCancelOrder = useCallback(
    async (id: string) => {
      try {
        await cancelQueuedOrder(conn, id);
        setMyOrders(myOrdersRef.current.filter((o) => o.id !== id));
      } finally {
        queue.refresh();
      }
    },
    [conn, queue, setMyOrders],
  );

  const visible = useMemo(
    () => (recipes ? recipes.filter((r) => r.on_menu !== false) : []),
    [recipes],
  );

  const allPours: PourEntry[] = useMemo(() => {
    if (!menus || !inventory) return [];
    const out: PourEntry[] = [];
    for (const menu of menus) {
      if (!menu.on_menu) continue;
      const matches = Object.entries(inventory.ingredients)
        .filter(([, stock]) => stock.category === menu.category)
        .sort(([aName, aStock], [bName, bStock]) => {
          const as = aStock.slot ?? "";
          const bs = bStock.slot ?? "";
          if (as && bs && as !== bs) return as.localeCompare(bs);
          if (as && !bs) return -1;
          if (!as && bs) return 1;
          return aName.localeCompare(bName);
        });
      for (const [ingredient, stock] of matches) {
        out.push({
          menu,
          ingredient,
          profile: stock.whiskey_profile ?? null,
          available: Boolean(stock.slot && stock.in_stock),
        });
      }
    }
    return out;
  }, [menus, inventory]);

  const pourSections = useMemo(() => {
    const groups = new Map<string, { menu: PourMenu; entries: PourEntry[] }>();
    for (const p of allPours) {
      if (!p.available) continue;
      const g = groups.get(p.menu.id) ?? { menu: p.menu, entries: [] };
      g.entries.push(p);
      groups.set(p.menu.id, g);
    }
    return [...groups.values()].sort(
      (a, b) => (a.menu.sort_order ?? 0) - (b.menu.sort_order ?? 0),
    );
  }, [allPours]);

  const pendingPours = allPours.filter((p) => !p.available);

  const availableRecipes = useMemo(
    () => (inventory ? visible.filter((r) => isAvailable(r, inventory)) : []),
    [visible, inventory],
  );
  const availablePourEntries = useMemo(() => allPours.filter((p) => p.available), [allPours]);

  // Spirit chip counts combine makeable cocktails (by base_spirit) with pour entries
  // whose menu category maps to that spirit (Whiskey, Wine, etc).
  const spiritCounts = useMemo<Record<string, number>>(() => {
    const counts: Record<string, number> = {};
    for (const r of availableRecipes) {
      if (!r.base_spirit) continue;
      counts[r.base_spirit] = (counts[r.base_spirit] ?? 0) + 1;
    }
    for (const entry of availablePourEntries) {
      const s = spiritForPourMenu(entry.menu);
      if (!s) continue;
      counts[s] = (counts[s] ?? 0) + 1;
    }
    return counts;
  }, [availableRecipes, availablePourEntries]);
  const totalCount = availableRecipes.length + availablePourEntries.length;

  const filteredRecipes = useMemo(
    () => availableRecipes.filter((r) => recipeMatchesFilters(r, baseFilter, flavorFilters)),
    [availableRecipes, baseFilter, flavorFilters],
  );
  const filteredPourEntries = useMemo(() => {
    if (flavorFilters.size > 0) return []; // pour entries have no flavor tags
    if (!baseFilter) return availablePourEntries;
    return availablePourEntries.filter((entry) => spiritForPourMenu(entry.menu) === baseFilter);
  }, [availablePourEntries, baseFilter, flavorFilters]);

  const filterBar =
    recipes && inventory && menus && totalCount > 0 ? (
      <FilterBar
        baseFilter={baseFilter}
        onBaseChange={setBaseFilter}
        flavorFilters={flavorFilters}
        onToggleFlavor={toggleFlavor}
        onClear={clearFilters}
        spiritCounts={spiritCounts}
        totalCount={totalCount}
      />
    ) : null;

  const menu = loadError ? (
    <p className={styles.sub}>Could not load menu: {loadError}</p>
  ) : !recipes || !inventory || !menus ? (
    <p className={styles.loading}>Loading menu…</p>
  ) : recipes.length === 0 && allPours.length === 0 ? (
    <p className={styles.loading}>No recipes configured.</p>
  ) : visible.length === 0 && pourSections.length === 0 ? (
    <p className={styles.loading}>No drinks on the menu right now.</p>
  ) : filtersActive ? (
    <FilteredView
      baseFilter={baseFilter}
      flavorFilters={flavorFilters}
      recipes={filteredRecipes}
      pourEntries={filteredPourEntries}
      disabled={placing}
      onPickRecipe={(recipe) => setConfirming({ recipe, orderable: true })}
      onPickPour={(entry) => setPourConfirming(entry)}
      onShowAll={clearFilters}
    />
  ) : (
    <>
      {pourSections.map(({ menu: pm, entries }) => (
        <PourMenuSection
          key={pm.id}
          menu={pm}
          entries={entries}
          disabled={placing}
          onPick={(entry) => setPourConfirming(entry)}
        />
      ))}
      <DrinkMenu
        recipes={visible}
        inventory={inventory}
        pendingPours={pendingPours}
        disabled={placing}
        onPick={(recipe, orderable) => setConfirming({ recipe, orderable })}
        onPickPendingPour={(entry) => setPourPreview(entry)}
      />
    </>
  );

  return (
    <>
      {!kioskMode && <Nav current="kiosk" />}
      <div className={styles.scope}>
        {kioskMode && (
          <header className={styles.kioskHeader}>
            <span className={styles.wordmark}>Cocktail Bot</span>
            <StatusPill state={queue.state} connected={connected} />
          </header>
        )}
        <main className={`${styles.main} ${kioskMode ? styles.mainKiosk : ""}`}>
          <h1 className={styles.title}>What can we make you?</h1>
          <p className={styles.sub}>
            {kioskMode ? "Tap a drink to order." : "Tap a drink to see what’s in it."}
            {!connected && !kioskMode && (
              <span className={styles.warn}>
                {" · reconnecting to "}
                {machineName || conn.hostname}
              </span>
            )}
            {conn.isDev && " · dev mock"}
          </p>

          {filterBar}
          <div className={styles.kBody}>
            <div className={styles.menuWrap}>{menu}</div>
            <KioskQueue
              state={queue.state}
              mine={myOrders}
              glasses={glasses}
              onCancelOrder={onCancelOrder}
            />
          </div>
        </main>

        {confirming && (
          <OrderSheet
            recipe={confirming.recipe}
            orderable={confirming.orderable}
            onCancel={() => setConfirming(null)}
            onConfirm={(double) => {
              const picked = confirming.recipe;
              setConfirming(null);
              startOrder(picked, double);
            }}
          />
        )}

        {pourConfirming && (
          <PourSheet
            entry={pourConfirming}
            onCancel={() => setPourConfirming(null)}
            onOrder={(size, serve) => {
              const picked = pourConfirming;
              setPourConfirming(null);
              startPourOrder(picked, size, serve);
            }}
          />
        )}

        {pourPreview && (
          <PourPreviewSheet
            entry={pourPreview}
            onClose={() => setPourPreview(null)}
          />
        )}

        {queue.state?.awaiting_pickup && (
          <PickupDialog
            held={queue.state.awaiting_pickup}
            next={queue.state.queue[0]}
            glasses={glasses}
            onStartNext={onStartNext}
          />
        )}

        {queue.state?.awaiting_glass && (
          <PlaceGlassDialog next={queue.state.awaiting_glass} glasses={glasses} onStartNext={onStartNext} />
        )}

        {placed && !failure && (
          <PlacedToast placed={placed} onDismiss={() => setPlaced(null)} />
        )}

        {failure && (
          <OrderError
            label={failure.label}
            message={kioskMode ? guestErrorMessage(failure.message) : failure.message}
            onDismiss={() => setFailure(null)}
          />
        )}
      </div>
    </>
  );
}

function StatusPill({ state, connected }: { state: QueueState | null; connected: boolean }) {
  let dotColor = "var(--ok)";
  let text: string;
  const inLine = state ? ordersInLine(state) : 0;
  if (!connected) {
    dotColor = "var(--bad)";
    text = "Reconnecting…";
  } else if (state?.awaiting_pickup) {
    dotColor = state.awaiting_pickup.error ? "var(--bad)" : "var(--ok)";
    text = state.awaiting_pickup.error
      ? `${state.awaiting_pickup.drink} needs help`
      : `${state.awaiting_pickup.drink} is ready`;
  } else if (state?.current) {
    dotColor = "var(--verm)";
    text = `Making ${state.current.drink}${inLine > 1 ? ` · ${inLine - 1} waiting` : ""}`;
  } else {
    text = `Nobody in line · about ${MINUTES_PER_DRINK} min per drink`;
  }
  return (
    <span className={styles.statusPill}>
      <span className={styles.statusDot} style={{ background: dotColor }} aria-hidden />
      {text}
    </span>
  );
}

function DrinkArt({
  slug,
  name,
  compact,
  large,
}: {
  slug?: string;
  name: string;
  compact?: boolean;
  large?: boolean;
}) {
  const className = compact ? styles.artCompact : large ? styles.artLarge : styles.art;
  if (!slug) return <div className={className} aria-hidden />;
  const size = compact ? 48 : large ? 180 : 150;
  return (
    <div className={className}>
      <picture>
        <source srcSet={`/drinks/png/dark/${slug}.png`} media="(prefers-color-scheme: dark)" />
        <img src={`/drinks/png/light/${slug}.png`} alt="" width={size} height={size} aria-label={name} />
      </picture>
    </div>
  );
}

const DEFAULT_SECTION = "Menu";

function orderOf(recipe: Recipe): number {
  return recipe.sort_order ?? Number.POSITIVE_INFINITY;
}

function compareRecipes(a: Recipe, b: Recipe): number {
  const oa = orderOf(a);
  const ob = orderOf(b);
  if (oa !== ob) return oa - ob;
  return a.name.localeCompare(b.name);
}

function groupBySection(recipes: Recipe[]): { section: string; recipes: Recipe[] }[] {
  const groups = new Map<string, Recipe[]>();
  for (const recipe of recipes) {
    const section = recipe.section?.trim() || DEFAULT_SECTION;
    const list = groups.get(section) ?? [];
    list.push(recipe);
    groups.set(section, list);
  }
  for (const list of groups.values()) list.sort(compareRecipes);
  const ordered = [...groups.entries()].sort(([aSection, aList], [bSection, bList]) => {
    if (aSection === DEFAULT_SECTION && bSection !== DEFAULT_SECTION) return 1;
    if (bSection === DEFAULT_SECTION && aSection !== DEFAULT_SECTION) return -1;
    const aMin = Math.min(...aList.map(orderOf));
    const bMin = Math.min(...bList.map(orderOf));
    if (aMin !== bMin) return aMin - bMin;
    return aSection.localeCompare(bSection);
  });
  return ordered.map(([section, recipes]) => ({ section, recipes }));
}

function firstMissingIngredient(recipe: Recipe, inventory: Inventory): string | null {
  for (const pour of recipePours(recipe)) {
    const stock = inventory.ingredients[pour.ingredient];
    if (!stock || !stock.in_stock) return pour.ingredient;
  }
  return null;
}

function DrinkMenu({
  recipes,
  inventory,
  pendingPours,
  disabled,
  onPick,
  onPickPendingPour,
}: {
  recipes: Recipe[];
  inventory: Inventory;
  pendingPours: PourEntry[];
  disabled: boolean;
  onPick: (recipe: Recipe, orderable: boolean) => void;
  onPickPendingPour: (entry: PourEntry) => void;
}) {
  const available: Recipe[] = [];
  const unavailable: Recipe[] = [];
  for (const recipe of recipes) {
    (isAvailable(recipe, inventory) ? available : unavailable).push(recipe);
  }
  const sections = groupBySection(available);
  unavailable.sort(compareRecipes);
  const hasBackSoon = unavailable.length > 0 || pendingPours.length > 0;

  return (
    <>
      {sections.map(({ section, recipes }) => (
        <section key={section} className={styles.menuSection} aria-label={section}>
          <h2>{section}</h2>
          <ul className={styles.menu}>
            {recipes.map((recipe) => (
              <li key={recipe.id}>
                <DrinkCard recipe={recipe} disabled={disabled} orderable onPick={onPick} />
              </li>
            ))}
          </ul>
        </section>
      ))}
      {hasBackSoon && (
        <section className={styles.backSoon} aria-label="Back soon">
          <h2>Back soon</h2>
          <div className={styles.backSoonRow}>
            {unavailable.map((recipe) => {
              const missing = firstMissingIngredient(recipe, inventory);
              return (
                <button
                  key={recipe.id}
                  type="button"
                  className={styles.backSoonPill}
                  onClick={() => onPick(recipe, false)}
                  disabled={disabled}
                >
                  <DrinkArt slug={recipe.image} name={recipe.name} compact />
                  <span className={styles.backSoonBody}>
                    <span className={styles.backSoonName}>{recipe.name}</span>
                    <span className={styles.backSoonReason}>
                      {missing ? `Out of ${titleCase(missing)}` : "Unavailable"}
                    </span>
                  </span>
                </button>
              );
            })}
            {pendingPours.map((entry) => (
              <button
                key={`${entry.menu.id}-${entry.ingredient}`}
                type="button"
                className={styles.backSoonPill}
                onClick={() => onPickPendingPour(entry)}
                disabled={disabled}
              >
                <DrinkArt slug={slugForMenu(entry.menu)} name={titleCase(entry.ingredient)} compact />
                <span className={styles.backSoonBody}>
                  <span className={styles.backSoonName}>{titleCase(entry.ingredient)}</span>
                  <span className={styles.backSoonReason}>{entry.menu.name} · coming soon</span>
                </span>
              </button>
            ))}
          </div>
        </section>
      )}
    </>
  );
}

function DrinkCard({
  recipe,
  disabled,
  orderable,
  onPick,
}: {
  recipe: Recipe;
  disabled: boolean;
  orderable: boolean;
  onPick: (recipe: Recipe, orderable: boolean) => void;
}) {
  const ingredientLine = recipePours(recipe).map((p) => titleCase(p.ingredient)).join(" · ");
  const flavors = (recipe.flavors ?? []).slice(0, 2);
  return (
    <button
      type="button"
      className={`${styles.drink} ${!orderable ? styles.out : ""}`}
      onClick={() => onPick(recipe, orderable)}
      disabled={disabled}
    >
      <DrinkArt slug={recipe.image} name={recipe.name} />
      <div className={styles.drinkHead}>
        <h3>{recipe.name}</h3>
        {recipe.tag && <span className={styles.tagBadge}>{recipe.tag}</span>}
      </div>
      {ingredientLine && <p className={styles.ingLine}>{ingredientLine}</p>}
      {flavors.length > 0 && (
        <div className={styles.flavorPills}>
          {flavors.map((f) => (
            <span key={f} className={styles.flavorPill}>
              {f}
            </span>
          ))}
        </div>
      )}
    </button>
  );
}

function FilterBar({
  baseFilter,
  onBaseChange,
  flavorFilters,
  onToggleFlavor,
  onClear,
  spiritCounts,
  totalCount,
}: {
  baseFilter: BaseSpirit | "";
  onBaseChange: (b: BaseSpirit | "") => void;
  flavorFilters: ReadonlySet<Flavor>;
  onToggleFlavor: (f: Flavor) => void;
  onClear: () => void;
  spiritCounts: Record<string, number>;
  totalCount: number;
}) {
  const anyActive = baseFilter !== "" || flavorFilters.size > 0;
  return (
    <div className={styles.filterBar}>
      <div className={styles.filterRow}>
        <button
          type="button"
          className={`${styles.filterChip} ${baseFilter === "" ? styles.filterChipActive : ""}`}
          onClick={() => onBaseChange("")}
          aria-pressed={baseFilter === ""}
        >
          All <span className={styles.filterChipCount}>{totalCount}</span>
        </button>
        {BASE_SPIRITS.map((s) => {
          const count = spiritCounts[s] ?? 0;
          if (count === 0) return null;
          const active = baseFilter === s;
          return (
            <button
              key={s}
              type="button"
              className={`${styles.filterChip} ${active ? styles.filterChipActive : ""}`}
              onClick={() => onBaseChange(active ? "" : s)}
              aria-pressed={active}
            >
              {s} <span className={styles.filterChipCount}>{count}</span>
            </button>
          );
        })}
      </div>
      <div className={styles.filterRow}>
        <span className={styles.filterRowLabel}>Flavor</span>
        {FLAVORS.map((f) => {
          const active = flavorFilters.has(f);
          return (
            <button
              key={f}
              type="button"
              className={`${styles.filterChip} ${styles.filterFlavor} ${active ? styles.filterFlavorActive : ""}`}
              onClick={() => onToggleFlavor(f)}
              aria-pressed={active}
            >
              {f}
            </button>
          );
        })}
        {anyActive && (
          <button type="button" className={styles.filterClear} onClick={onClear}>
            Clear
          </button>
        )}
      </div>
    </div>
  );
}

function FilteredView({
  baseFilter,
  flavorFilters,
  recipes,
  pourEntries,
  disabled,
  onPickRecipe,
  onPickPour,
  onShowAll,
}: {
  baseFilter: BaseSpirit | "";
  flavorFilters: ReadonlySet<Flavor>;
  recipes: Recipe[];
  pourEntries: PourEntry[];
  disabled: boolean;
  onPickRecipe: (recipe: Recipe) => void;
  onPickPour: (entry: PourEntry) => void;
  onShowAll: () => void;
}) {
  const headingParts: string[] = [];
  if (baseFilter) headingParts.push(baseFilter);
  for (const f of flavorFilters) headingParts.push(f);
  const heading = headingParts.join(" · ") || "All drinks";
  const total = recipes.length + pourEntries.length;
  if (total === 0) {
    return (
      <>
        <h2 className={styles.filterHead}>
          {heading} <span className={styles.filterHeadCount}>0 drinks</span>
        </h2>
        <div className={styles.emptyState}>
          <span>Nothing matches that combination.</span>
          <button type="button" className={`${styles.btn} ${styles.btnMain}`} onClick={onShowAll}>
            Show all drinks
          </button>
        </div>
      </>
    );
  }
  return (
    <>
      <h2 className={styles.filterHead}>
        {heading}{" "}
        <span className={styles.filterHeadCount}>
          {total} {total === 1 ? "drink" : "drinks"}
        </span>
      </h2>
      <ul className={styles.menu}>
        {recipes.map((recipe) => (
          <li key={recipe.id}>
            <DrinkCard
              recipe={recipe}
              disabled={disabled}
              orderable
              onPick={() => onPickRecipe(recipe)}
            />
          </li>
        ))}
        {pourEntries.map((entry) => (
          <li key={`${entry.menu.id}-${entry.ingredient}`}>
            <BottleCard entry={entry} disabled={disabled} onPick={onPickPour} />
          </li>
        ))}
      </ul>
    </>
  );
}

function OrderSheet({
  recipe,
  orderable,
  onCancel,
  onConfirm,
}: {
  recipe: Recipe;
  orderable: boolean;
  onCancel: () => void;
  onConfirm: (double: boolean) => void;
}) {
  const [double, setDouble] = useState(false);
  return (
    <div className={styles.scrim} onClick={onCancel}>
      <div className={`${styles.sheet} ${styles.sheetWide}`} onClick={(e) => e.stopPropagation()}>
        <div className={styles.sheetArt}>
          <DrinkArt slug={recipe.image} name={recipe.name} large />
        </div>
        <h3>{recipe.name}</h3>
        <ul className={styles.pours}>
          {recipePours(recipe).map((pour, i) => (
            <li key={i}>
              <span>{titleCase(pour.ingredient)}</span>
              <span>{pour.oz} oz</span>
            </li>
          ))}
        </ul>
        {recipe.can_double && (
          <button
            type="button"
            role="switch"
            aria-checked={double}
            className={styles.doubleRow}
            onClick={() => setDouble((v) => !v)}
          >
            <span className={styles.doubleText}>
              <strong>Make it a double</strong>
              <span>
                2 of the same drink, made together in about the same time. Served in one large glass to share
                or split.
              </span>
            </span>
            <span className={styles.switch} aria-hidden>
              <span className={styles.switchThumb} />
            </span>
          </button>
        )}
        {!orderable && <p className={styles.outNote}>Out of stock</p>}
        <div className={styles.btns}>
          <button type="button" className={`${styles.btn} ${styles.btnBig}`} onClick={onCancel}>
            Back
          </button>
          <button
            type="button"
            className={`${styles.btn} ${styles.btnBig} ${styles.btnMain}`}
            onClick={() => onConfirm(double)}
            disabled={!orderable}
          >
            Order
          </button>
        </div>
      </div>
    </div>
  );
}

function useQueue(conn: ViamConnection, connected: boolean, onState: (s: QueueState) => void) {
  const [state, setState] = useState<QueueState | null>(null);
  const cancelled = useRef(false);
  const refresh = useCallback(() => {
    getQueue(conn)
      .then((s) => {
        if (cancelled.current) return;
        setState(s);
        onState(s);
      })
      .catch(() => {});
  }, [conn, onState]);

  useEffect(() => {
    if (!connected) return;
    cancelled.current = false;
    refresh();
    const id = window.setInterval(refresh, QUEUE_POLL_MS);
    return () => {
      cancelled.current = true;
      window.clearInterval(id);
    };
  }, [connected, refresh]);

  return useMemo(() => ({ state, refresh }), [state, refresh]);
}

function waitText(position: number): string {
  if (position <= 1) return "Up next";
  return `#${position} · ~${position * MINUTES_PER_DRINK} min`;
}

// Blocks the menu until staff confirm the finished drink was taken and the next glass is in place.
function PickupDialog({
  held,
  next,
  glasses,
  onStartNext,
}: {
  held: QueueOrder;
  next?: QueueOrder;
  glasses: Glass[];
  onStartNext: () => void;
}) {
  const nextGlass = next?.glass;
  const image = glasses.find((g) => g.name === nextGlass)?.image;
  return (
    <div className={`${styles.scrim} ${styles.pickupScrim}`}>
      <div
        className={styles.pickup}
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="pickup-title"
      >
        <h3 id="pickup-title" className={held.error ? styles.pickupTitleBad : styles.pickupTitle}>
          {held.error ? `${held.drink} didn’t finish` : `${held.drink} is ready`}
        </h3>
        <p className={styles.pickupNote}>
          {held.error
            ? "Ask a staff member to clear the station."
            : held.double
              ? `Both drinks come in one ${held.glass ? held.glass.toLowerCase() + " glass" : "large glass"}. Grab an extra glass to split it.`
              : "Take your drink from the station."}
        </p>
        <p className={styles.pickupNote}>
          {!next
            ? "No one else is waiting. When the next order comes in, you’ll be told which glass to place."
            : nextGlass
              ? "Then place this glass at the station for the next drink:"
              : "Then put an empty glass at the station for the next drink."}
        </p>
        {nextGlass && (
          <div className={styles.pickupGlass}>
            <GlassIcon image={image} size={120} />
            <span>
              {nextGlass}
              <span className={styles.kqPlaceFor}>for {next.drink}</span>
            </span>
          </div>
        )}
        <button type="button" className={`${styles.btn} ${styles.btnMain} ${styles.pickupBtn}`} onClick={onStartNext}>
          {next ? "Glass ready, start next" : "Drink taken"}
        </button>
      </div>
    </div>
  );
}

function PlaceGlassDialog({
  next,
  glasses,
  onStartNext,
}: {
  next: QueueOrder;
  glasses: Glass[];
  onStartNext: () => void;
}) {
  const image = glasses.find((g) => g.name === next.glass)?.image;
  return (
    <div className={`${styles.scrim} ${styles.pickupScrim}`}>
      <div
        className={styles.pickup}
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="place-glass-title"
      >
        <h3 id="place-glass-title" className={styles.pickupTitle}>
          Place the glass for the next drink
        </h3>
        <p className={styles.pickupNote}>The bartender starts once this glass is at the station:</p>
        <div className={styles.pickupGlass}>
          <GlassIcon image={image} size={120} />
          <span>
            {next.glass}
            <span className={styles.kqPlaceFor}>for {next.drink}</span>
          </span>
        </div>
        <button type="button" className={`${styles.btn} ${styles.btnMain} ${styles.pickupBtn}`} onClick={onStartNext}>
          Glass ready, start
        </button>
      </div>
    </div>
  );
}

function GlassTag({ name, images }: { name?: string; images: Map<string, GlassImage | undefined> }) {
  if (!name) return null;
  return (
    <span className={styles.kqGlass}>
      <GlassIcon image={images.get(name)} size={18} />
      {name}
    </span>
  );
}

function KioskQueue({
  state,
  mine,
  glasses,
  onCancelOrder,
}: {
  state: QueueState | null;
  mine: MyOrder[];
  glasses: Glass[];
  onCancelOrder: (id: string) => void;
}) {
  const mineIds = new Set(mine.map((o) => o.id));
  const held = state?.awaiting_pickup;
  const holdingForGlass = held || state?.awaiting_glass;
  const waiting: QueueOrder[] = state?.queue ?? [];
  const images = new Map(glasses.map((g) => [g.name, g.image]));
  const current = state?.current;

  return (
    <aside className={styles.kq} aria-label="Queue">
      <h2>In line</h2>

      {!held && current?.glass && (
        <div className={styles.kqStart} role="status">
          Glass at the station
          <div className={styles.kqPlace}>
            <GlassIcon image={images.get(current.glass)} size={84} />
            <span>
              {current.glass}
              <span className={styles.kqPlaceFor}>for {current.drink}</span>
            </span>
          </div>
        </div>
      )}

      {current || waiting.length > 0 ? (
        <div className={styles.rows}>
          {current && (
            <div className={styles.kqRow}>
              <span className={styles.kqName}>
                {current.drink}
                {current.raw_step && <span className={styles.kqStep}>{current.raw_step}</span>}
                <GlassTag name={current.glass} images={images} />
              </span>
              <span className={styles.kqTag}>Making now</span>
            </div>
          )}
          {waiting.map((o, i) => (
            <div key={o.id} className={styles.kqRow}>
              <span className={styles.kqName}>
                {o.drink}
                <GlassTag name={o.glass} images={images} />
              </span>
              <span className={styles.kqWait}>
                {holdingForGlass && i === 0 ? "Waiting for glass" : waitText(i + 1)}
                {mineIds.has(o.id) && (
                  <button type="button" className={styles.kqCancel} onClick={() => onCancelOrder(o.id)}>
                    Cancel
                  </button>
                )}
              </span>
            </div>
          ))}
        </div>
      ) : (
        !held && <p className={styles.kqNote}>Nobody in line.</p>
      )}
    </aside>
  );
}

function PlacedToast({ placed, onDismiss }: { placed: Placed; onDismiss: () => void }) {
  const position = placed.ahead + 1;
  return (
    <div className={styles.toast} role="status" onClick={onDismiss}>
      <strong>{placed.label} is in line</strong>
      <span>
        {placed.ahead === 0
          ? "You’re first in line."
          : `You’re #${position}, about ${position * MINUTES_PER_DRINK} min.`}{" "}
        Watch the line for your drink.
      </span>
    </div>
  );
}

function OrderError({
  label,
  message,
  onDismiss,
}: {
  label: string;
  message: string;
  onDismiss: () => void;
}) {
  return (
    <div className={styles.scrim}>
      <div className={styles.sheet}>
        <h3 className={styles.err}>{label} failed</h3>
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

function PourMenuSection({
  menu,
  entries,
  disabled,
  onPick,
}: {
  menu: PourMenu;
  entries: PourEntry[];
  disabled: boolean;
  onPick: (entry: PourEntry) => void;
}) {
  return (
    <section className={styles.menuSection} aria-label={menu.name}>
      <h2>{menu.name}</h2>
      {menu.intro && <p className={styles.whiskeyIntro}>{menu.intro}</p>}
      <ul className={styles.menu}>
        {entries.map((entry) => (
          <li key={entry.ingredient}>
            <BottleCard entry={entry} disabled={disabled} onPick={onPick} />
          </li>
        ))}
      </ul>
    </section>
  );
}

function rarityFromUs(us?: string): string | null {
  if (!us) return null;
  const lower = us.toLowerCase();
  if (lower.startsWith("very unlikely")) return "Very rare";
  if (lower.startsWith("unlikely")) return "Rare";
  return null;
}

function BottleCard({
  entry,
  disabled,
  onPick,
}: {
  entry: PourEntry;
  disabled: boolean;
  onPick: (entry: PourEntry) => void;
}) {
  const name = titleCase(entry.ingredient);
  const profile = entry.profile;
  const line = profile?.notes || profile?.style || "";
  const rarity = rarityFromUs(profile?.us);
  return (
    <button
      type="button"
      className={styles.drink}
      onClick={() => onPick(entry)}
      disabled={disabled}
    >
      <DrinkArt slug={slugForMenu(entry.menu)} name={name} />
      <div className={styles.drinkHead}>
        <h3>{name}</h3>
        <span className={styles.drinkHeadChips}>
          {profile?.abv ? <span className={styles.tagBadge}>{profile.abv}%</span> : null}
          {rarity && <span className={styles.whiskeyRarity}>{rarity}</span>}
        </span>
      </div>
      {line && <p className={styles.ingLine}>{line}</p>}
    </button>
  );
}

function BottleDetails({
  entry,
  traits,
}: {
  entry: PourEntry;
  traits: string[];
}) {
  const profile = entry.profile;
  if (!profile) return null;
  const { abv, region, story, notes, profile: meters, us, style } = profile;
  const traitList = traits.length > 0 ? traits : WHISKEY_TRAITS;
  const hasMeters = meters && meters.length > 0;
  return (
    <>
      {(abv || region || style) && (
        <p className={styles.whiskeySheetMeta}>
          {abv ? `${abv}% · ` : ""}
          {region ?? style ?? ""}
        </p>
      )}
      {story && <p className={styles.whiskeySheetStory}>{story}</p>}
      {notes && (
        <p className={styles.whiskeySheetNotes}>
          <strong>Tasting notes:</strong> {notes}
        </p>
      )}
      {hasMeters && (
        <div className={styles.whiskeySheetMeters} aria-label="Flavor profile">
          {traitList.map((trait, i) => {
            const value = meters![i] ?? 0;
            return (
              <div key={trait} className={styles.whiskeySheetMeter}>
                <span className={styles.whiskeySheetMeterLabel}>{trait}</span>
                <span
                  className={styles.whiskeySheetMeterPips}
                  role="img"
                  aria-label={`${trait}: ${value} out of 5`}
                >
                  {[0, 1, 2, 3, 4].map((n) => (
                    <i
                      key={n}
                      className={n < value ? styles.whiskeySheetPipOn : styles.whiskeySheetPip}
                    />
                  ))}
                </span>
              </div>
            );
          })}
        </div>
      )}
      {us && <span className={styles.whiskeySheetUs}>{us}</span>}
    </>
  );
}

function PourSheet({
  entry,
  onCancel,
  onOrder,
}: {
  entry: PourEntry;
  onCancel: () => void;
  onOrder: (size: PourSize, serve: PourServe) => void;
}) {
  const { menu } = entry;
  const sizes = menu.sizes;
  const serves = menu.serves;
  const [size, setSize] = useState<PourSize>(defaultSize(menu) ?? sizes[0]);
  const [serve, setServe] = useState<PourServe>(serves[0]);
  const name = titleCase(entry.ingredient);
  const traits = menu.traits ?? [];

  const canOrder = !!size && !!serve;

  return (
    <div className={styles.scrim} onClick={onCancel}>
      <div className={`${styles.sheet} ${styles.sheetWide}`} onClick={(e) => e.stopPropagation()}>
        <div className={styles.sheetArt}>
          <DrinkArt slug={slugForMenu(menu)} name={name} large />
        </div>
        <h3>{name}</h3>
        <BottleDetails entry={entry} traits={traits} />

        {sizes.length > 0 && (
          <div>
            <span className={styles.whiskeySegLabel} id="lbl-size">Size</span>
            <div className={styles.whiskeySeg} role="group" aria-labelledby="lbl-size">
              {sizes.map((s) => (
                <button
                  key={s.label}
                  type="button"
                  className={size?.label === s.label ? `${styles.whiskeySegBtn} ${styles.whiskeySegBtnOn}` : styles.whiskeySegBtn}
                  onClick={() => setSize(s)}
                  aria-pressed={size?.label === s.label}
                >
                  {s.label}, {s.oz} oz
                </button>
              ))}
            </div>
          </div>
        )}

        {serves.length >= 2 && (
          <div>
            <span className={styles.whiskeySegLabel} id="lbl-serve">Serve it</span>
            <div className={styles.whiskeySeg} role="group" aria-labelledby="lbl-serve">
              {serves.map((s) => (
                <button
                  key={s.label}
                  type="button"
                  className={serve?.label === s.label ? `${styles.whiskeySegBtn} ${styles.whiskeySegBtnOn}` : styles.whiskeySegBtn}
                  onClick={() => setServe(s)}
                  aria-pressed={serve?.label === s.label}
                >
                  {s.label}
                </button>
              ))}
            </div>
          </div>
        )}

        <div className={styles.btns}>
          <button type="button" className={`${styles.btn} ${styles.btnBig}`} onClick={onCancel}>
            Back
          </button>
          <button
            type="button"
            className={`${styles.btn} ${styles.btnBig} ${styles.btnMain}`}
            onClick={() => canOrder && onOrder(size, serve)}
            disabled={!canOrder}
          >
            Order
          </button>
        </div>
      </div>
    </div>
  );
}

function PourPreviewSheet({
  entry,
  onClose,
}: {
  entry: PourEntry;
  onClose: () => void;
}) {
  const name = titleCase(entry.ingredient);
  const traits = entry.menu.traits ?? [];
  return (
    <div className={styles.scrim} onClick={onClose}>
      <div className={`${styles.sheet} ${styles.sheetWide}`} onClick={(e) => e.stopPropagation()}>
        <div className={styles.sheetArt}>
          <DrinkArt slug={slugForMenu(entry.menu)} name={name} large />
        </div>
        <h3>{name}</h3>
        <p className={styles.whiskeyPreviewTag}>
          Coming soon · tap when it&rsquo;s on the {entry.menu.name.toLowerCase()}
        </p>
        <BottleDetails entry={entry} traits={traits} />
        <div className={styles.btns}>
          <button type="button" className={`${styles.btn} ${styles.btnBig}`} onClick={onClose}>
            Close
          </button>
        </div>
      </div>
    </div>
  );
}
