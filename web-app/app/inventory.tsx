"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ViamConnection } from "./lib/viamClient";
import {
  adjustUnopened,
  clearSlotAssignment,
  clearWhiskeyProfile,
  deleteInventoryItem,
  getInventory,
  getRecipes,
  updateCategory,
  updateInventoryItem,
  updatePourerOz,
  updateRefrigerated,
  updateSlotAssignment,
  updateWhiskeyProfile,
  setUnopened,
} from "./lib/viamClient";
import type { IngredientStock, Inventory, WhiskeyProfile } from "./lib/inventory";
import { WHISKEY_TRAITS, findEquivalentIngredient } from "./lib/inventory";
import type { Recipe } from "./lib/recipes";
import { recipeIngredients } from "./lib/recipes";
import { Nav } from "./nav";
import { titleCase } from "./lib/display";
import styles from "./inventory.module.css";

const COMMON_CATEGORIES = ["Spirits", "Liqueurs", "Juices & mixers", "Syrups & bitters"];
const UNCATEGORIZED = "Uncategorized";

export function InventoryPage({ conn }: { conn: ViamConnection }) {
  const [inventory, setInventory] = useState<Inventory | null>(null);
  const [recipes, setRecipes] = useState<Recipe[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [newName, setNewName] = useState("");
  const [busy, setBusy] = useState(false);
  const [selectedSlotId, setSelectedSlotId] = useState<string | null>(null);
  const [selectedIngredient, setSelectedIngredient] = useState<string | null>(null);
  const [filter, setFilter] = useState<string>("All");

  const load = useCallback(() => {
    Promise.all([getInventory(conn), getRecipes(conn)])
      .then(([inv, rs]) => {
        setInventory(inv);
        setRecipes(rs);
      })
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

  async function remove(ingredient: string) {
    if (!confirm(`Delete "${titleCase(ingredient)}" from inventory?`)) return;
    setBusy(true);
    setError(null);
    try {
      await deleteInventoryItem(conn, ingredient);
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
    const existing = inventory ? findEquivalentIngredient(name, Object.keys(inventory.ingredients)) : undefined;
    if (existing) {
      setError(`"${titleCase(existing)}" is already in inventory.`);
      return;
    }
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

  function applyUnopened(ingredient: string, next: (prev: number) => number) {
    setInventory((inv) => {
      const stock = inv?.ingredients[ingredient];
      if (!inv || !stock) return inv;
      const unopened = Math.max(next(stock.unopened ?? 0), 0);
      return { ...inv, ingredients: { ...inv.ingredients, [ingredient]: { ...stock, unopened } } };
    });
  }

  // Updates locally first so repeated taps feel instant; the server applies deltas, so taps from two devices both count.
  async function changeUnopened(ingredient: string, delta: number) {
    setError(null);
    applyUnopened(ingredient, (prev) => prev + delta);
    try {
      await adjustUnopened(conn, ingredient, delta);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      load();
    }
  }

  async function saveUnopened(ingredient: string, count: number) {
    setError(null);
    applyUnopened(ingredient, () => count);
    try {
      await setUnopened(conn, ingredient, count);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      load();
    }
  }

  async function saveRefrigerated(ingredient: string, refrigerated: boolean) {
    setBusy(true);
    setError(null);
    try {
      await updateRefrigerated(conn, ingredient, refrigerated);
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function setFridgeStock(names: string[], inStock: boolean) {
    const verb = inStock ? "back in stock" : "out of stock";
    if (!confirm(`Mark ${names.length} refrigerated item${names.length === 1 ? "" : "s"} ${verb}?\n\n${names.map(titleCase).join(", ")}`)) {
      return;
    }
    setBusy(true);
    setError(null);
    const results = await Promise.allSettled(names.map((name) => updateInventoryItem(conn, name, inStock)));
    const failed = names.filter((_, i) => results[i].status === "rejected");
    if (failed.length > 0) {
      setError(`Couldn't mark ${failed.map(titleCase).join(", ")} ${verb}. Try again.`);
    }
    load();
    setBusy(false);
  }

  async function savePourerOz(ingredient: string, oz: number) {
    setBusy(true);
    setError(null);
    try {
      await updatePourerOz(conn, ingredient, oz);
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function assignSlot(slot: string, ingredient: string) {
    setBusy(true);
    setError(null);
    try {
      if (ingredient === "") {
        await clearSlotAssignment(conn, slot);
      } else {
        await updateSlotAssignment(conn, slot, ingredient);
      }
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function saveCategory(ingredient: string, category: string) {
    setBusy(true);
    setError(null);
    try {
      await updateCategory(conn, ingredient, category);
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function saveWhiskeyProfile(ingredient: string, profile: WhiskeyProfile) {
    setBusy(true);
    setError(null);
    try {
      await updateWhiskeyProfile(conn, ingredient, profile);
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function removeWhiskeyProfile(ingredient: string) {
    setBusy(true);
    setError(null);
    try {
      await clearWhiskeyProfile(conn, ingredient);
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  const usedIn = useMemo(() => buildRecipeIndex(recipes), [recipes]);

  const items = inventory ? Object.entries(inventory.ingredients).sort(([a], [b]) => a.localeCompare(b)) : [];
  const slots = inventory?.slots ?? [];
  const ingredientNames = items.map(([name]) => name);
  const hasRefrigerated = items.some(([, s]) => s.refrigerated);
  const refrigeratedInStock = items.filter(([, s]) => s.refrigerated && s.in_stock).map(([name]) => name);
  // Only slotted ingredients can be in stock, so unslotted fridge items would never leave this list.
  const refrigeratedOut = items.filter(([, s]) => s.refrigerated && s.slot && !s.in_stock).map(([name]) => name);
  const selectedSlot = selectedSlotId ? slots.find((s) => s.id === selectedSlotId) ?? null : null;
  const selectedStock =
    inventory && selectedSlot?.ingredient ? inventory.ingredients[selectedSlot.ingredient] : undefined;
  const activeIngredient = selectedIngredient && inventory?.ingredients[selectedIngredient] ? selectedIngredient : null;
  const activeIngredientStock = activeIngredient ? inventory!.ingredients[activeIngredient] : undefined;

  function pickIngredient(name: string) {
    const stock = inventory?.ingredients[name];
    if (stock?.slot) {
      setSelectedSlotId((prev) => (prev === stock.slot ? null : stock.slot!));
      setSelectedIngredient(null);
    } else {
      setSelectedSlotId(null);
      setSelectedIngredient((prev) => (prev === name ? null : name));
    }
  }

  return (
    <>
      <Nav current="inventory" />
      <div className={styles.scope}>
        <main className={styles.page}>
          <div className={styles.toolbar}>
            <h1 className={styles.pageTitle}>Inventory</h1>
            <div className={styles.addRow}>
              {hasRefrigerated && (
                <>
                  <button
                    type="button"
                    className={`${styles.btn} ${styles.btnFridge}`}
                    onClick={() => setFridgeStock(refrigeratedInStock, false)}
                    disabled={busy || refrigeratedInStock.length === 0}
                  >
                    Fridge items out
                    {refrigeratedInStock.length > 0 && <span className={styles.chipCount}>{refrigeratedInStock.length}</span>}
                  </button>
                  <button
                    type="button"
                    className={`${styles.btn} ${styles.btnFridge}`}
                    onClick={() => setFridgeStock(refrigeratedOut, true)}
                    disabled={busy || refrigeratedOut.length === 0}
                  >
                    Fridge items back in
                    {refrigeratedOut.length > 0 && <span className={styles.chipCount}>{refrigeratedOut.length}</span>}
                  </button>
                </>
              )}
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
          ) : (
            <div className={styles.layout}>
              <div className={styles.layoutLeft}>
                {slots.length > 0 && (
                  <RackSection
                    slots={slots}
                    ingredients={inventory.ingredients}
                    selectedId={selectedSlotId}
                    onSelect={(id) => setSelectedSlotId((prev) => (prev === id ? null : id))}
                  />
                )}
                {items.length === 0 ? (
                  <p className={styles.empty}>No ingredients yet. Add one above.</p>
                ) : (
                  <AllIngredientsSection
                    items={items}
                    filter={filter}
                    onFilter={setFilter}
                    selectedName={activeIngredient ?? (selectedSlot?.ingredient ?? null)}
                    onPick={pickIngredient}
                    onAdjustUnopened={changeUnopened}
                    onSetUnopened={saveUnopened}
                  />
                )}
              </div>
              {selectedSlot ? (
                <SlotPanel
                  slot={selectedSlot}
                  stock={selectedStock}
                  ingredientNames={ingredientNames}
                  usedIn={selectedSlot.ingredient ? usedIn.get(selectedSlot.ingredient) ?? [] : []}
                  busy={busy}
                  onAssign={assignSlot}
                  onPourerOz={savePourerOz}
                  onToggleStock={toggle}
                  onRefrigerated={saveRefrigerated}
                  onAdjustUnopened={changeUnopened}
                  onSetUnopened={saveUnopened}
                  onDeleteIngredient={remove}
                  onCategory={saveCategory}
                  onSaveWhiskeyProfile={saveWhiskeyProfile}
                  onClearWhiskeyProfile={removeWhiskeyProfile}
                  onClose={() => setSelectedSlotId(null)}
                />
              ) : activeIngredient ? (
                <IngredientPanel
                  name={activeIngredient}
                  stock={activeIngredientStock}
                  usedIn={usedIn.get(activeIngredient) ?? []}
                  busy={busy}
                  onCategory={saveCategory}
                  onSaveWhiskeyProfile={saveWhiskeyProfile}
                  onClearWhiskeyProfile={removeWhiskeyProfile}
                  onRefrigerated={saveRefrigerated}
                  onAdjustUnopened={changeUnopened}
                  onSetUnopened={saveUnopened}
                  onDeleteIngredient={remove}
                  onClose={() => setSelectedIngredient(null)}
                />
              ) : (
                <aside className={`${styles.panel} ${styles.panelEmpty}`} aria-hidden>
                  <div className={styles.panelHead}>
                    <div>
                      <p className={styles.panelCrumb}>Side panel</p>
                      <h2 className={styles.panelTitle}>Pick a slot</h2>
                    </div>
                  </div>
                  <div className={styles.panelBody}>
                    <p className={styles.panelHint}>
                      Tap any card above to change its bottle, set the pourer size, or mark it out of stock.
                    </p>
                  </div>
                </aside>
              )}
            </div>
          )}
        </main>
      </div>
    </>
  );
}

type Slot = { id: string; ingredient?: string };
type IngredientRecord = Record<string, IngredientStock>;
type IngredientEntry = [string, IngredientStock];

function buildRecipeIndex(recipes: Recipe[]): Map<string, string[]> {
  const out = new Map<string, string[]>();
  for (const recipe of recipes) {
    for (const ingredient of recipeIngredients(recipe)) {
      const list = out.get(ingredient) ?? [];
      list.push(recipe.name);
      out.set(ingredient, list);
    }
  }
  for (const list of out.values()) list.sort((a, b) => a.localeCompare(b));
  return out;
}

function categoryOf(stock: { category?: string }): string {
  return stock.category && stock.category.trim() ? stock.category : UNCATEGORIZED;
}

function parseSlotId(id: string): { letter: string; row: string } {
  const parts = id.split("-");
  const letter = (parts[1] ?? "").toUpperCase();
  const row = parts.slice(2).join(" ");
  return { letter, row };
}

const POURER_SIZES = [0.5, 1, 1.5] as const;

function RackSection({
  slots,
  ingredients,
  selectedId,
  onSelect,
}: {
  slots: Slot[];
  ingredients: IngredientRecord;
  selectedId: string | null;
  onSelect: (slotId: string) => void;
}) {
  const groups = new Map<string, Slot[]>();
  for (const slot of slots) {
    const { row } = parseSlotId(slot.id);
    const key = row || "other";
    const list = groups.get(key) ?? [];
    list.push(slot);
    groups.set(key, list);
  }
  for (const list of groups.values()) {
    list.sort((a, b) => a.id.localeCompare(b.id));
  }

  return (
    <section className={styles.section}>
      <h2>
        On the rack
        <span className={styles.sectionCount}>{slots.length}</span>
      </h2>
      {[...groups.entries()].map(([row, rowSlots]) => (
        <div key={row} className={styles.rackGroup}>
          <h3 className={styles.rowLabel}>{titleCase(row)}</h3>
          <div className={styles.rack}>
            {rowSlots.map((slot) => (
              <RackCard
                key={slot.id}
                slot={slot}
                stock={slot.ingredient ? ingredients[slot.ingredient] : undefined}
                selected={selectedId === slot.id}
                onSelect={() => onSelect(slot.id)}
              />
            ))}
          </div>
        </div>
      ))}
    </section>
  );
}

function RackCard({
  slot,
  stock,
  selected,
  onSelect,
}: {
  slot: Slot;
  stock: { in_stock: boolean; pourer_oz?: number } | undefined;
  selected: boolean;
  onSelect: () => void;
}) {
  const { letter } = parseSlotId(slot.id);
  const hasBottle = Boolean(slot.ingredient);
  const inStock = hasBottle && stock?.in_stock;
  const statusLabel = !hasBottle ? "Empty" : inStock ? "In stock" : "Out of stock";
  const dotColor = !hasBottle ? "var(--smoke)" : inStock ? "var(--ok)" : "var(--bad)";

  return (
    <button
      type="button"
      aria-pressed={selected}
      className={`${styles.rackCard} ${selected ? styles.rackCardSelected : ""}`}
      onClick={onSelect}
    >
      <span className={styles.rackHead}>
        <span className={styles.rackLetter}>{letter}</span>
      </span>
      <span className={styles.rackBottle}>{hasBottle ? titleCase(slot.ingredient!) : "Empty slot"}</span>
      {hasBottle && stock?.pourer_oz != null && (
        <span className={styles.rackPourer}>{stock.pourer_oz} oz pourer</span>
      )}
      <span className={styles.rackStatus}>
        <span className={styles.dot} style={{ background: dotColor }} aria-hidden />
        {statusLabel}
      </span>
    </button>
  );
}

function SlotPanel({
  slot,
  stock,
  ingredientNames,
  usedIn,
  busy,
  onAssign,
  onPourerOz,
  onToggleStock,
  onRefrigerated,
  onAdjustUnopened,
  onSetUnopened,
  onDeleteIngredient,
  onCategory,
  onSaveWhiskeyProfile,
  onClearWhiskeyProfile,
  onClose,
}: {
  slot: Slot;
  stock: IngredientStock | undefined;
  ingredientNames: string[];
  usedIn: string[];
  busy: boolean;
  onAssign: (slot: string, ingredient: string) => void;
  onPourerOz: (ingredient: string, oz: number) => void;
  onToggleStock: (ingredient: string, nextInStock: boolean) => void;
  onRefrigerated: (ingredient: string, refrigerated: boolean) => void;
  onAdjustUnopened: (ingredient: string, delta: number) => void;
  onSetUnopened: (ingredient: string, count: number) => void;
  onDeleteIngredient: (ingredient: string) => void;
  onCategory: (ingredient: string, category: string) => void;
  onSaveWhiskeyProfile: (ingredient: string, profile: WhiskeyProfile) => void;
  onClearWhiskeyProfile: (ingredient: string) => void;
  onClose: () => void;
}) {
  const { letter, row } = parseSlotId(slot.id);
  const bottle = slot.ingredient;
  const hasBottle = Boolean(bottle);
  const inStock = Boolean(hasBottle && stock?.in_stock);
  const pourerOz = stock?.pourer_oz;

  return (
    <aside className={styles.panel} aria-label={`Slot ${letter}`}>
      <div className={styles.panelHead}>
        <div>
          <p className={styles.panelCrumb}>
            Slot {letter}
            {row ? ` · ${titleCase(row)}` : ""}
          </p>
          <h2 className={styles.panelTitle}>{hasBottle ? titleCase(bottle!) : "Empty slot"}</h2>
          {hasBottle && (
            <p className={styles.panelStatus}>
              <span
                className={styles.dot}
                style={{ background: inStock ? "var(--ok)" : "var(--bad)" }}
                aria-hidden
              />
              {inStock ? "In stock" : "Out of stock"}
            </p>
          )}
        </div>
        <button type="button" className={styles.panelClose} onClick={onClose} aria-label="Close">
          ✕
        </button>
      </div>

      <div className={styles.panelBody}>
      <label className={styles.panelField}>
        <span className={styles.panelLabel}>Bottle in this slot</span>
        <select
          className={styles.select}
          value={bottle ?? ""}
          onChange={(e) => onAssign(slot.id, e.target.value)}
          disabled={busy}
        >
          <option value="">— empty —</option>
          {ingredientNames.map((n) => (
            <option key={n} value={n}>
              {titleCase(n)}
            </option>
          ))}
        </select>
      </label>

      {hasBottle && (
        <CategoryField
          ingredient={bottle!}
          initial={stock?.category ?? ""}
          busy={busy}
          onSave={onCategory}
        />
      )}

      {hasBottle && usedIn.length > 0 && (
        <div className={styles.panelField}>
          <span className={styles.panelLabel}>Used in</span>
          <div className={styles.usedChips}>
            {usedIn.map((name) => (
              <span key={name} className={styles.usedChip}>
                {name}
              </span>
            ))}
          </div>
        </div>
      )}

      {hasBottle && (
        <div className={styles.panelField}>
          <span className={styles.panelLabel}>Unopened bottles</span>
          <UnopenedStepper
            ingredient={bottle!}
            count={stock?.unopened ?? 0}
            large
            onAdjust={onAdjustUnopened}
            onSet={onSetUnopened}
          />
        </div>
      )}

      {hasBottle && (
        <StorageField
          ingredient={bottle!}
          refrigerated={Boolean(stock?.refrigerated)}
          busy={busy}
          onChange={onRefrigerated}
        />
      )}

      {hasBottle && (
        <fieldset className={styles.panelField}>
          <legend className={styles.panelLabel}>Pourer</legend>
          <div className={styles.segmented}>
            {POURER_SIZES.map((oz) => {
              const active = pourerOz === oz;
              return (
                <button
                  key={oz}
                  type="button"
                  aria-pressed={active}
                  className={`${styles.segment} ${active ? styles.segmentActive : ""}`}
                  onClick={() => onPourerOz(bottle!, oz)}
                  disabled={busy}
                >
                  {oz} oz
                </button>
              );
            })}
          </div>
          {pourerOz != null && !POURER_SIZES.includes(pourerOz as typeof POURER_SIZES[number]) && (
            <p className={styles.panelHint}>Currently set to {pourerOz} oz.</p>
          )}
        </fieldset>
      )}

      {hasBottle && stock?.category === "Whiskey" && (
        <WhiskeyProfileSection
          ingredient={bottle!}
          profile={stock?.whiskey_profile}
          busy={busy}
          onSave={onSaveWhiskeyProfile}
          onClear={onClearWhiskeyProfile}
        />
      )}
      </div>

      {hasBottle && (
        <div className={styles.panelActions}>
          <button
            type="button"
            className={styles.panelBtn}
            onClick={() => onToggleStock(bottle!, !inStock)}
            disabled={busy}
          >
            Mark {inStock ? "out of stock" : "in stock"}
          </button>
          <button
            type="button"
            className={styles.panelBtnGhost}
            onClick={() => onAssign(slot.id, "")}
            disabled={busy}
          >
            Clear slot
          </button>
          <button
            type="button"
            className={styles.panelBtnDanger}
            onClick={() => onDeleteIngredient(bottle!)}
            disabled={busy}
          >
            Delete ingredient…
          </button>
        </div>
      )}
    </aside>
  );
}

function IngredientPanel({
  name,
  stock,
  usedIn,
  busy,
  onCategory,
  onSaveWhiskeyProfile,
  onClearWhiskeyProfile,
  onRefrigerated,
  onAdjustUnopened,
  onSetUnopened,
  onDeleteIngredient,
  onClose,
}: {
  name: string;
  stock: IngredientStock | undefined;
  usedIn: string[];
  busy: boolean;
  onCategory: (ingredient: string, category: string) => void;
  onSaveWhiskeyProfile: (ingredient: string, profile: WhiskeyProfile) => void;
  onClearWhiskeyProfile: (ingredient: string) => void;
  onRefrigerated: (ingredient: string, refrigerated: boolean) => void;
  onAdjustUnopened: (ingredient: string, delta: number) => void;
  onSetUnopened: (ingredient: string, count: number) => void;
  onDeleteIngredient: (ingredient: string) => void;
  onClose: () => void;
}) {
  return (
    <aside className={styles.panel} aria-label={name}>
      <div className={styles.panelHead}>
        <div>
          <p className={styles.panelCrumb}>Not on rack</p>
          <h2 className={styles.panelTitle}>{titleCase(name)}</h2>
          <p className={styles.panelHint}>Assign a slot above before you can mark this in stock.</p>
        </div>
        <button type="button" className={styles.panelClose} onClick={onClose} aria-label="Close">
          ✕
        </button>
      </div>

      <div className={styles.panelBody}>
        <div className={styles.panelField}>
          <span className={styles.panelLabel}>Unopened bottles</span>
          <UnopenedStepper
            ingredient={name}
            count={stock?.unopened ?? 0}
            large
            onAdjust={onAdjustUnopened}
            onSet={onSetUnopened}
          />
        </div>

        <StorageField
          ingredient={name}
          refrigerated={Boolean(stock?.refrigerated)}
          busy={busy}
          onChange={onRefrigerated}
        />

        <CategoryField ingredient={name} initial={stock?.category ?? ""} busy={busy} onSave={onCategory} />

        {usedIn.length > 0 && (
          <div className={styles.panelField}>
            <span className={styles.panelLabel}>Used in</span>
            <div className={styles.usedChips}>
              {usedIn.map((recipeName) => (
                <span key={recipeName} className={styles.usedChip}>
                  {recipeName}
                </span>
              ))}
            </div>
          </div>
        )}

        {stock?.category === "Whiskey" && (
          <WhiskeyProfileSection
            ingredient={name}
            profile={stock?.whiskey_profile}
            busy={busy}
            onSave={onSaveWhiskeyProfile}
            onClear={onClearWhiskeyProfile}
          />
        )}
      </div>

      <div className={styles.panelActions}>
        <button
          type="button"
          className={styles.panelBtnDanger}
          onClick={() => onDeleteIngredient(name)}
          disabled={busy}
        >
          Delete ingredient…
        </button>
      </div>
    </aside>
  );
}

function StorageField({
  ingredient,
  refrigerated,
  busy,
  onChange,
}: {
  ingredient: string;
  refrigerated: boolean;
  busy: boolean;
  onChange: (ingredient: string, refrigerated: boolean) => void;
}) {
  const options = [
    { label: "Room temp", value: false },
    { label: "Refrigerated", value: true },
  ];
  return (
    <fieldset className={styles.panelField}>
      <legend className={styles.panelLabel}>Storage</legend>
      <div className={`${styles.segmented} ${styles.segmentedTwo}`}>
        {options.map((opt) => {
          const active = refrigerated === opt.value;
          return (
            <button
              key={opt.label}
              type="button"
              aria-pressed={active}
              className={`${styles.segment} ${active ? styles.segmentActive : ""}`}
              onClick={() => {
                if (!active) onChange(ingredient, opt.value);
              }}
              disabled={busy}
            >
              {opt.label}
            </button>
          );
        })}
      </div>
    </fieldset>
  );
}

function CategoryField({
  ingredient,
  initial,
  busy,
  onSave,
}: {
  ingredient: string;
  initial: string;
  busy: boolean;
  onSave: (ingredient: string, category: string) => void;
}) {
  const options =
    initial && !COMMON_CATEGORIES.includes(initial) ? [initial, ...COMMON_CATEGORIES] : COMMON_CATEGORIES;
  return (
    <label className={styles.panelField}>
      <span className={styles.panelLabel}>Category</span>
      <select
        className={styles.select}
        value={initial}
        onChange={(e) => {
          if (e.target.value !== initial) onSave(ingredient, e.target.value);
        }}
        disabled={busy}
      >
        <option value="">— None —</option>
        {options.map((c) => (
          <option key={c} value={c}>
            {c}
          </option>
        ))}
      </select>
    </label>
  );
}

function WhiskeyProfileSection({
  ingredient,
  profile,
  busy,
  onSave,
  onClear,
}: {
  ingredient: string;
  profile: WhiskeyProfile | undefined;
  busy: boolean;
  onSave: (ingredient: string, profile: WhiskeyProfile) => void;
  onClear: (ingredient: string) => void;
}) {
  const [editing, setEditing] = useState(false);
  const hasProfile =
    !!profile &&
    (profile.abv ||
      profile.region ||
      profile.story ||
      profile.notes ||
      profile.us ||
      profile.style ||
      (profile.profile && profile.profile.length > 0));

  if (editing) {
    return (
      <WhiskeyProfileEditor
        initial={profile ?? {}}
        busy={busy}
        onCancel={() => setEditing(false)}
        onSave={(next) => {
          onSave(ingredient, next);
          setEditing(false);
        }}
      />
    );
  }

  return (
    <div className={styles.panelField}>
      <span className={styles.panelLabel}>Tasting info</span>
      {hasProfile ? (
        <div className={styles.whiskeyView}>
          {(profile!.abv || profile!.region) && (
            <p className={styles.whiskeyMeta}>
              {profile!.abv ? `${profile!.abv}%` : ""}
              {profile!.abv && profile!.region ? " · " : ""}
              {profile!.region ?? ""}
            </p>
          )}
          {profile!.notes && <p className={styles.whiskeyNotes}>{profile!.notes}</p>}
          {profile!.story && <p className={styles.whiskeyStory}>{profile!.story}</p>}
          {profile!.profile && profile!.profile.length > 0 && (
            <div className={styles.whiskeyMeters}>
              {WHISKEY_TRAITS.map((trait, i) => {
                const value = profile!.profile?.[i] ?? 0;
                return (
                  <div key={trait} className={styles.whiskeyMeter}>
                    <span className={styles.whiskeyMeterLabel}>{trait}</span>
                    <span
                      className={styles.whiskeyMeterPips}
                      role="img"
                      aria-label={`${trait}: ${value} out of 5`}
                    >
                      {[0, 1, 2, 3, 4].map((n) => (
                        <i key={n} className={n < value ? styles.whiskeyPipOn : styles.whiskeyPip} />
                      ))}
                    </span>
                  </div>
                );
              })}
            </div>
          )}
          {profile!.us && <p className={styles.whiskeyUs}>{profile!.us}</p>}
          {profile!.style && !profile!.story && (
            <p className={styles.whiskeyMeta}>Style: {profile!.style}</p>
          )}
          <div className={styles.whiskeyActions}>
            <button
              type="button"
              className={styles.panelBtn}
              onClick={() => setEditing(true)}
              disabled={busy}
            >
              Edit tasting info
            </button>
            <button
              type="button"
              className={styles.panelBtnGhost}
              onClick={() => onClear(ingredient)}
              disabled={busy}
            >
              Clear
            </button>
          </div>
        </div>
      ) : (
        <button
          type="button"
          className={styles.panelBtn}
          onClick={() => setEditing(true)}
          disabled={busy}
        >
          Add tasting info
        </button>
      )}
    </div>
  );
}

function WhiskeyProfileEditor({
  initial,
  busy,
  onCancel,
  onSave,
}: {
  initial: WhiskeyProfile;
  busy: boolean;
  onCancel: () => void;
  onSave: (profile: WhiskeyProfile) => void;
}) {
  const [abv, setAbv] = useState<string>(initial.abv != null ? String(initial.abv) : "");
  const [region, setRegion] = useState(initial.region ?? "");
  const [story, setStory] = useState(initial.story ?? "");
  const [notes, setNotes] = useState(initial.notes ?? "");
  const [us, setUs] = useState(initial.us ?? "");
  const [style, setStyle] = useState(initial.style ?? "");
  const [scores, setScores] = useState<number[]>(() => {
    const seed = initial.profile ?? [];
    return WHISKEY_TRAITS.map((_, i) => seed[i] ?? 0);
  });

  function save() {
    const next: WhiskeyProfile = {};
    const abvNum = parseFloat(abv);
    if (!Number.isNaN(abvNum) && abvNum > 0) next.abv = abvNum;
    if (region.trim()) next.region = region.trim();
    if (story.trim()) next.story = story.trim();
    if (notes.trim()) next.notes = notes.trim();
    if (us.trim()) next.us = us.trim();
    if (style.trim()) next.style = style.trim();
    if (scores.some((n) => n > 0)) next.profile = scores;
    onSave(next);
  }

  return (
    <div className={styles.panelField}>
      <span className={styles.panelLabel}>Tasting info</span>
      <div className={styles.whiskeyEditor}>
        <label className={styles.whiskeyEditorRow}>
          <span className={styles.whiskeyEditorLabel}>ABV %</span>
          <input
            className={styles.input}
            type="number"
            step="0.1"
            min="0"
            max="100"
            value={abv}
            onChange={(e) => setAbv(e.target.value)}
            disabled={busy}
          />
        </label>
        <label className={styles.whiskeyEditorRow}>
          <span className={styles.whiskeyEditorLabel}>Region</span>
          <input
            className={styles.input}
            type="text"
            value={region}
            onChange={(e) => setRegion(e.target.value)}
            placeholder="Distillery, place"
            disabled={busy}
          />
        </label>
        <label className={styles.whiskeyEditorRow}>
          <span className={styles.whiskeyEditorLabel}>Tasting notes</span>
          <textarea
            className={styles.textarea}
            rows={2}
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            placeholder="Honey, orange peel, …"
            disabled={busy}
          />
        </label>
        <label className={styles.whiskeyEditorRow}>
          <span className={styles.whiskeyEditorLabel}>Story</span>
          <textarea
            className={styles.textarea}
            rows={3}
            value={story}
            onChange={(e) => setStory(e.target.value)}
            placeholder="A couple of sentences about the whiskey"
            disabled={busy}
          />
        </label>
        <label className={styles.whiskeyEditorRow}>
          <span className={styles.whiskeyEditorLabel}>Availability</span>
          <input
            className={styles.input}
            type="text"
            value={us}
            onChange={(e) => setUs(e.target.value)}
            placeholder="Easy to find in the US"
            disabled={busy}
          />
        </label>
        <label className={styles.whiskeyEditorRow}>
          <span className={styles.whiskeyEditorLabel}>Style (fallback)</span>
          <input
            className={styles.input}
            type="text"
            value={style}
            onChange={(e) => setStyle(e.target.value)}
            placeholder="Bourbon, Rye, Scotch…"
            disabled={busy}
          />
        </label>
        <div className={styles.whiskeyEditorMeters}>
          {WHISKEY_TRAITS.map((trait, i) => (
            <div key={trait} className={styles.whiskeyEditorMeterRow}>
              <span className={styles.whiskeyEditorLabel}>{trait}</span>
              <div className={styles.whiskeyEditorPips}>
                {[0, 1, 2, 3, 4].map((n) => {
                  const value = n + 1;
                  const on = scores[i] >= value;
                  return (
                    <button
                      key={n}
                      type="button"
                      aria-label={`${trait} ${value}`}
                      aria-pressed={on}
                      className={on ? styles.whiskeyEditorPipOn : styles.whiskeyEditorPip}
                      onClick={() => {
                        const nextScores = [...scores];
                        nextScores[i] = nextScores[i] === value ? value - 1 : value;
                        setScores(nextScores);
                      }}
                      disabled={busy}
                    />
                  );
                })}
              </div>
            </div>
          ))}
        </div>
        <div className={styles.whiskeyActions}>
          <button
            type="button"
            className={`${styles.panelBtn} ${styles.panelBtnMain}`}
            onClick={save}
            disabled={busy}
          >
            Save tasting info
          </button>
          <button
            type="button"
            className={styles.panelBtnGhost}
            onClick={onCancel}
            disabled={busy}
          >
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}

function AllIngredientsSection({
  items,
  filter,
  onFilter,
  selectedName,
  onPick,
  onAdjustUnopened,
  onSetUnopened,
}: {
  items: IngredientEntry[];
  filter: string;
  onFilter: (next: string) => void;
  selectedName: string | null;
  onPick: (name: string) => void;
  onAdjustUnopened: (ingredient: string, delta: number) => void;
  onSetUnopened: (ingredient: string, count: number) => void;
}) {
  const counts = new Map<string, number>();
  for (const [, stock] of items) {
    const cat = categoryOf(stock);
    counts.set(cat, (counts.get(cat) ?? 0) + 1);
  }
  const activeCategories = [...counts.keys()].sort((a, b) => {
    if (a === UNCATEGORIZED) return 1;
    if (b === UNCATEGORIZED) return -1;
    return a.localeCompare(b);
  });

  const filteredItems = filter === "All" ? items : items.filter(([, s]) => categoryOf(s) === filter);
  const grouped = new Map<string, IngredientEntry[]>();
  for (const entry of filteredItems) {
    const cat = categoryOf(entry[1]);
    const list = grouped.get(cat) ?? [];
    list.push(entry);
    grouped.set(cat, list);
  }
  const orderedCats = [...grouped.keys()].sort((a, b) => {
    if (a === UNCATEGORIZED) return 1;
    if (b === UNCATEGORIZED) return -1;
    return a.localeCompare(b);
  });

  return (
    <section className={styles.section}>
      <div className={styles.sectionBar}>
        <h2>All ingredients</h2>
        <div className={styles.chips}>
          <FilterChip label="All" count={items.length} active={filter === "All"} onClick={() => onFilter("All")} />
          {activeCategories.map((cat) => (
            <FilterChip
              key={cat}
              label={cat}
              count={counts.get(cat) ?? 0}
              active={filter === cat}
              onClick={() => onFilter(cat)}
            />
          ))}
        </div>
      </div>

      <div className={styles.tableHead}>
        <span>Ingredient</span>
        <span>Status</span>
        <span className={styles.colPourer}>Pourer</span>
        <span className={styles.colUnopened}>Unopened</span>
      </div>

      {orderedCats.map((cat) => (
        <div key={cat} className={styles.catGroup}>
          <h3 className={styles.rowLabel}>{cat}</h3>
          {(grouped.get(cat) ?? []).map(([name, stock]) => (
            <IngredientRow
              key={name}
              name={name}
              stock={stock}
              selected={selectedName === name}
              onPick={onPick}
              onAdjustUnopened={onAdjustUnopened}
              onSetUnopened={onSetUnopened}
            />
          ))}
        </div>
      ))}
    </section>
  );
}

function FilterChip({
  label,
  count,
  active,
  onClick,
}: {
  label: string;
  count: number;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      className={`${styles.chip} ${active ? styles.chipActive : ""}`}
      onClick={onClick}
    >
      {label} <span className={styles.chipCount}>{count}</span>
    </button>
  );
}

function IngredientRow({
  name,
  stock,
  selected,
  onPick,
  onAdjustUnopened,
  onSetUnopened,
}: {
  name: string;
  stock: IngredientStock;
  selected: boolean;
  onPick: (name: string) => void;
  onAdjustUnopened: (ingredient: string, delta: number) => void;
  onSetUnopened: (ingredient: string, count: number) => void;
}) {
  let dotColor = "transparent";
  let statusLabel: string;
  if (!stock.slot) {
    statusLabel = "Not on rack";
    dotColor = "var(--smoke)";
  } else if (stock.in_stock) {
    statusLabel = `On rack · Slot ${parseSlotId(stock.slot).letter}`;
    dotColor = "var(--ok)";
  } else {
    statusLabel = "Out of stock";
    dotColor = "var(--bad)";
  }
  return (
    <div
      className={`${styles.tableRow} ${selected ? styles.tableRowSelected : ""}`}
      role="button"
      tabIndex={0}
      aria-pressed={selected}
      onClick={() => onPick(name)}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onPick(name);
        }
      }}
    >
      <span className={styles.rowName}>
        {titleCase(name)}
        {stock.refrigerated && <span className={styles.fridgeTag}>Fridge</span>}
      </span>
      <span className={styles.rowStatus}>
        <span
          className={styles.dot}
          style={{ background: stock.slot ? dotColor : "transparent", border: stock.slot ? "0" : "1.5px solid var(--smoke)" }}
          aria-hidden
        />
        {statusLabel}
      </span>
      <span className={`${styles.rowPourer} ${styles.colPourer}`}>
        {stock.pourer_oz != null ? `${stock.pourer_oz} oz` : "—"}
      </span>
      <span className={styles.colUnopened}>
        <UnopenedStepper
          ingredient={name}
          count={stock.unopened ?? 0}
          onAdjust={onAdjustUnopened}
          onSet={onSetUnopened}
        />
      </span>
    </div>
  );
}

function UnopenedStepper({
  ingredient,
  count,
  large = false,
  onAdjust,
  onSet,
}: {
  ingredient: string;
  count: number;
  large?: boolean;
  onAdjust: (ingredient: string, delta: number) => void;
  onSet: (ingredient: string, count: number) => void;
}) {
  const [draft, setDraft] = useState<string | null>(null);
  const cancelled = useRef(false);
  const label = titleCase(ingredient);

  function commit() {
    if (cancelled.current) {
      cancelled.current = false;
      setDraft(null);
      return;
    }
    if (draft === null) return;
    const next = Number(draft);
    setDraft(null);
    if (draft.trim() === "" || !Number.isInteger(next) || next < 0 || next === count) return;
    onSet(ingredient, next);
  }

  return (
    // The row behind this is clickable; keep stepper taps from selecting it.
    <span
      className={`${styles.stepper} ${large ? styles.stepperLarge : ""}`}
      onClick={(e) => e.stopPropagation()}
      onKeyDown={(e) => e.stopPropagation()}
    >
      <button
        type="button"
        className={styles.stepperBtn}
        onClick={() => onAdjust(ingredient, -1)}
        disabled={count <= 0}
        aria-label={`One fewer unopened ${label}`}
      >
        −
      </button>
      <input
        type="text"
        inputMode="numeric"
        pattern="[0-9]*"
        className={styles.stepperInput}
        value={draft ?? String(count)}
        onFocus={(e) => {
          setDraft(String(count));
          e.target.select();
        }}
        onChange={(e) => setDraft(e.target.value.replace(/[^0-9]/g, ""))}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === "Enter") e.currentTarget.blur();
          if (e.key === "Escape") {
            cancelled.current = true;
            e.currentTarget.blur();
          }
        }}
        aria-label={`Unopened ${label} bottles`}
      />
      <button
        type="button"
        className={styles.stepperBtn}
        onClick={() => onAdjust(ingredient, 1)}
        aria-label={`One more unopened ${label}`}
      >
        +
      </button>
    </span>
  );
}

