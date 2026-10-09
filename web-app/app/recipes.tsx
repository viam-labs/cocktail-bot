"use client";

import { useCallback, useEffect, useState } from "react";
import type { ViamConnection } from "./lib/viamClient";
import { getInventory, getRecipes, updateRecipes } from "./lib/viamClient";
import type { Pour, Recipe } from "./lib/recipes";
import { DRINK_IMAGE_SLUGS } from "./lib/recipes";
import type { Inventory } from "./lib/inventory";
import { Nav } from "./nav";
import { titleCase as cap } from "./lib/display";
import styles from "./recipes.module.css";

function slugify(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, "_").replace(/^_|_$/g, "");
}

function fmtOz(v: number): string {
  return (Math.round(v * 100) / 100).toString();
}

export function RecipesPage({ conn }: { conn: ViamConnection }) {
  const [recipes, setRecipes] = useState<Recipe[] | null>(null);
  const [inventory, setInventory] = useState<Inventory | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState<{ index: number | null; recipe: Recipe } | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(() => {
    Promise.all([getRecipes(conn), getInventory(conn)])
      .then(([r, i]) => {
        setRecipes(r);
        setInventory(i);
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : String(err)));
  }, [conn]);

  useEffect(() => {
    load();
  }, [load]);

  async function save(next: Recipe[]) {
    setBusy(true);
    setError(null);
    try {
      await updateRecipes(conn, next);
      setRecipes(next);
      setEditing(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  function onSaveEditing(recipe: Recipe) {
    if (!recipes || !editing) return;
    const next = [...recipes];
    if (editing.index === null) {
      next.push(recipe);
    } else {
      next[editing.index] = recipe;
    }
    save(next);
  }

  function onDelete(index: number) {
    if (!recipes) return;
    save(recipes.filter((_, i) => i !== index));
  }

  const ingredientNames = inventory ? Object.keys(inventory.ingredients).sort() : [];

  return (
    <>
      <Nav current="recipes" />
      <div className={styles.scope}>
        <main className={styles.page}>
          <div className={styles.toolbar}>
            <h1 className={styles.pageTitle}>Recipes</h1>
            <button
              type="button"
              className={`${styles.btn} ${styles.btnMain}`}
              onClick={() => setEditing({ index: null, recipe: { id: "", name: "", pours: [], on_menu: true } })}
            >
              New recipe
            </button>
          </div>

          {error && <p className={styles.error}>{error}</p>}

          {!recipes ? (
            <p className={styles.empty}>Loading…</p>
          ) : recipes.length === 0 ? (
            <p className={styles.empty}>No recipes yet.</p>
          ) : (
            recipes.map((recipe, i) => (
              <section key={recipe.id || i} className={styles.recipe}>
                <button
                  type="button"
                  className={styles.rhead}
                  onClick={() => setEditing({ index: i, recipe: structuredClone(recipe) })}
                >
                  <h2>{recipe.name || recipe.id || "(unnamed)"}</h2>
                  {recipe.on_menu !== false ? (
                    <span className={styles.st}>
                      <i style={{ background: "var(--ok)" }} />
                      On the menu
                    </span>
                  ) : (
                    <span className={`${styles.st} ${styles.muted}`}>
                      <i style={{ background: "var(--smoke)" }} />
                      Off the menu
                    </span>
                  )}
                  <span className={styles.chev}>›</span>
                </button>
                <div className={`${styles.rows} ${styles.ing}`}>
                  <div className={`${styles.row} ${styles.head}`}>
                    <span>#</span>
                    <span>Ingredient</span>
                    <span>Target</span>
                    <span>Tolerance</span>
                  </div>
                  {recipe.pours.length === 0 ? (
                    <div className={`${styles.row} ${styles.muted}`}>
                      <span className={`${styles.num} ${styles.muted}`}>—</span>
                      <span>No pours</span>
                      <span />
                      <span />
                    </div>
                  ) : (
                    recipe.pours.map((p, j) => (
                      <div key={j} className={styles.row}>
                        <span className={`${styles.num} ${styles.muted}`}>{j + 1}</span>
                        <span>{cap(p.ingredient)}</span>
                        <span className={styles.num}>{fmtOz(p.oz)} oz</span>
                        <span className={`${styles.num} ${styles.muted}`}>
                          {p.tol_oz != null && p.tol_oz > 0 ? `± ${fmtOz(p.tol_oz)} oz` : "—"}
                        </span>
                      </div>
                    ))
                  )}
                </div>
              </section>
            ))
          )}
        </main>
      </div>

      {editing && (
        <RecipeEditor
          initial={editing.recipe}
          existingIndex={editing.index}
          ingredientNames={ingredientNames}
          busy={busy}
          onClose={() => setEditing(null)}
          onSave={onSaveEditing}
          onDelete={() => {
            if (editing.index === null) return;
            if (confirm(`Delete "${editing.recipe.name || editing.recipe.id}"?`)) onDelete(editing.index);
          }}
        />
      )}
    </>
  );
}

function RecipeEditor({
  initial,
  existingIndex,
  ingredientNames,
  busy,
  onClose,
  onSave,
  onDelete,
}: {
  initial: Recipe;
  existingIndex: number | null;
  ingredientNames: string[];
  busy: boolean;
  onClose: () => void;
  onSave: (recipe: Recipe) => void;
  onDelete: () => void;
}) {
  const [name, setName] = useState(initial.name);
  const [id, setId] = useState(initial.id);
  const [idTouched, setIdTouched] = useState(initial.id !== "");
  const [onMenu, setOnMenu] = useState(initial.on_menu !== false);
  const [image, setImage] = useState(initial.image ?? "");
  const [pours, setPours] = useState<Pour[]>(
    initial.pours.length > 0 ? initial.pours : [{ ingredient: ingredientNames[0] ?? "", oz: 1, tol_oz: 0.05 }],
  );

  function updatePour(i: number, patch: Partial<Pour>) {
    setPours((prev) => prev.map((p, idx) => (idx === i ? { ...p, ...patch } : p)));
  }

  function removePour(i: number) {
    setPours((prev) => prev.filter((_, idx) => idx !== i));
  }

  function addPour() {
    setPours((prev) => [...prev, { ingredient: ingredientNames[0] ?? "", oz: 1, tol_oz: 0.05 }]);
  }

  function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    const trimmedName = name.trim();
    const effectiveId = (idTouched ? id.trim() : slugify(trimmedName)) || slugify(trimmedName);
    if (!trimmedName) {
      alert("Recipe name is required");
      return;
    }
    if (!effectiveId) {
      alert("Recipe id is required");
      return;
    }
    onSave({ id: effectiveId, name: trimmedName, pours, on_menu: onMenu, image: image || undefined });
  }

  return (
    <div className={styles.scope}>
      <div className={styles.scrim} onClick={onClose} />
      <form className={styles.panel} onSubmit={onSubmit}>
        <div className={styles.panelHead}>
          <div>
            <h4>{initial.id ? "Edit recipe" : "New recipe"}</h4>
            <div className={styles.muted}>Recipe</div>
          </div>
          <button type="button" className={styles.closeX} onClick={onClose} aria-label="Close">
            ✕
          </button>
        </div>

        <div className={styles.form}>
          <label>
            <span>Name</span>
            <input
              className={styles.input}
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Espresso martini"
              required
            />
          </label>

          <label>
            <span>ID</span>
            <input
              className={styles.input}
              type="text"
              value={idTouched ? id : slugify(name)}
              onChange={(e) => {
                setIdTouched(true);
                setId(e.target.value);
              }}
              placeholder="espresso_martini"
            />
          </label>

          <label className={styles.toggle}>
            <input type="checkbox" checked={onMenu} onChange={(e) => setOnMenu(e.target.checked)} />
            On the menu
          </label>

          <label>
            <span>Illustration</span>
            <select className={styles.select} value={image} onChange={(e) => setImage(e.target.value)}>
              <option value="">None (grey tile)</option>
              {DRINK_IMAGE_SLUGS.map((slug) => (
                <option key={slug} value={slug}>
                  {cap(slug)}
                </option>
              ))}
            </select>
          </label>

          <p className={styles.sub}>Pours, in order</p>

          {pours.map((pour, i) => (
            <div key={i} className={styles.pour}>
              <label>
                <span>Ingredient</span>
                {ingredientNames.length > 0 ? (
                  <select
                    className={styles.select}
                    value={pour.ingredient}
                    onChange={(e) => updatePour(i, { ingredient: e.target.value })}
                  >
                    {!ingredientNames.includes(pour.ingredient) && pour.ingredient !== "" && (
                      <option value={pour.ingredient}>{pour.ingredient} (unknown)</option>
                    )}
                    {pour.ingredient === "" && <option value="">Pick…</option>}
                    {ingredientNames.map((n) => (
                      <option key={n} value={n}>
                        {cap(n)}
                      </option>
                    ))}
                  </select>
                ) : (
                  <input
                    className={styles.input}
                    type="text"
                    value={pour.ingredient}
                    onChange={(e) => updatePour(i, { ingredient: e.target.value })}
                    placeholder="ingredient"
                  />
                )}
              </label>
              <label>
                <span>Target (oz)</span>
                <input
                  className={styles.input}
                  type="number"
                  min={0}
                  step={0.25}
                  value={pour.oz}
                  onChange={(e) => updatePour(i, { oz: Math.max(0, Number(e.target.value)) })}
                />
              </label>
              <label>
                <span>± (oz)</span>
                <input
                  className={styles.input}
                  type="number"
                  min={0}
                  step={0.05}
                  value={pour.tol_oz ?? 0}
                  onChange={(e) => updatePour(i, { tol_oz: Math.max(0, Number(e.target.value)) })}
                />
              </label>
              <button
                type="button"
                className={styles.rm}
                onClick={() => removePour(i)}
                disabled={pours.length === 1}
                aria-label={`Remove pour ${i + 1}`}
              >
                ✕
              </button>
            </div>
          ))}

          <button type="button" className={styles.btn} onClick={addPour}>
            Add a pour
          </button>
        </div>

        <div className={styles.dlgBtns}>
          {existingIndex !== null && (
            <button
              type="button"
              className={`${styles.btn} ${styles.btnDanger}`}
              onClick={onDelete}
              disabled={busy}
              style={{ marginRight: "auto" }}
            >
              Delete
            </button>
          )}
          <button type="button" className={styles.btn} onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className={`${styles.btn} ${styles.btnMain}`} disabled={busy}>
            {busy ? "Saving…" : "Save"}
          </button>
        </div>
      </form>
    </div>
  );
}
