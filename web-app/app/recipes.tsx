"use client";

import { useCallback, useEffect, useState } from "react";
import type { ViamConnection } from "./lib/viamClient";
import { getInventory, getRecipes, updateRecipes } from "./lib/viamClient";
import type { Pour, Recipe } from "./lib/recipes";
import type { Inventory } from "./lib/inventory";
import { Nav } from "./nav";

function slugify(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, "_").replace(/^_|_$/g, "");
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
      <main className="max-w-3xl mx-auto p-6 flex flex-col gap-6">
        <header className="flex items-center justify-between">
          <h1 className="text-3xl font-semibold tracking-tight">Recipes</h1>
          <button
            type="button"
            onClick={() =>
              setEditing({ index: null, recipe: { id: "", name: "", pours: [] } })
            }
            className="h-11 px-5 rounded-lg bg-black text-white font-medium"
          >
            New recipe
          </button>
        </header>

        {error && <p className="text-red-600 text-sm">{error}</p>}

        {!recipes ? (
          <p className="text-gray-600">Loading…</p>
        ) : recipes.length === 0 ? (
          <p className="text-gray-600">No recipes yet.</p>
        ) : (
          <ul className="grid gap-3">
            {recipes.map((recipe, i) => (
              <li
                key={recipe.id || i}
                className="border border-gray-200 rounded-xl p-5 bg-white flex items-center justify-between"
              >
                <div>
                  <h2 className="text-lg font-semibold">{recipe.name || recipe.id || "(unnamed)"}</h2>
                  <p className="text-sm text-gray-600 mt-0.5">
                    {recipe.pours.length === 0
                      ? "no pours"
                      : recipe.pours.map((p) => `${p.oz} oz ${p.ingredient}`).join(" · ")}
                  </p>
                </div>
                <div className="flex gap-2">
                  <button
                    type="button"
                    onClick={() => setEditing({ index: i, recipe: structuredClone(recipe) })}
                    className="h-9 px-4 rounded-lg text-sm font-medium border border-gray-300 hover:bg-gray-50"
                  >
                    Edit
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      if (confirm(`Delete "${recipe.name || recipe.id}"?`)) onDelete(i);
                    }}
                    disabled={busy}
                    className="h-9 px-4 rounded-lg text-sm font-medium border border-gray-300 text-red-600 hover:bg-red-50 disabled:opacity-50"
                  >
                    Delete
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </main>

      {editing && (
        <RecipeEditor
          initial={editing.recipe}
          ingredientNames={ingredientNames}
          busy={busy}
          onClose={() => setEditing(null)}
          onSave={onSaveEditing}
        />
      )}
    </>
  );
}

function RecipeEditor({
  initial,
  ingredientNames,
  busy,
  onClose,
  onSave,
}: {
  initial: Recipe;
  ingredientNames: string[];
  busy: boolean;
  onClose: () => void;
  onSave: (recipe: Recipe) => void;
}) {
  const [name, setName] = useState(initial.name);
  const [id, setId] = useState(initial.id);
  const [idTouched, setIdTouched] = useState(initial.id !== "");
  const [pours, setPours] = useState<Pour[]>(initial.pours);

  function updatePour(i: number, patch: Partial<Pour>) {
    setPours((prev) => prev.map((p, idx) => (idx === i ? { ...p, ...patch } : p)));
  }

  function removePour(i: number) {
    setPours((prev) => prev.filter((_, idx) => idx !== i));
  }

  function addPour() {
    const defaultIngredient = ingredientNames[0] ?? "";
    setPours((prev) => [...prev, { ingredient: defaultIngredient, oz: 1 }]);
  }

  function onSubmit() {
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
    onSave({ id: effectiveId, name: trimmedName, pours });
  }

  return (
    <div className="fixed inset-0 bg-black/40 z-20 flex items-end sm:items-center justify-center p-4">
      <div className="bg-white rounded-2xl w-full max-w-xl max-h-[90vh] flex flex-col overflow-hidden">
        <header className="flex items-center justify-between px-6 py-4 border-b border-gray-200">
          <h2 className="text-xl font-semibold">{initial.id ? "Edit recipe" : "New recipe"}</h2>
          <button
            type="button"
            onClick={onClose}
            className="h-9 w-9 rounded-full bg-gray-100 text-gray-600 text-xl leading-none"
            aria-label="Close"
          >
            ×
          </button>
        </header>

        <div className="overflow-y-auto p-6 flex flex-col gap-5">
          <section className="flex flex-col gap-2">
            <label className="text-sm font-medium text-gray-700">Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="h-11 border border-gray-300 rounded-lg px-4"
              placeholder="Espresso Martini"
            />
          </section>

          <section className="flex flex-col gap-2">
            <label className="text-sm font-medium text-gray-700">ID</label>
            <input
              type="text"
              value={idTouched ? id : slugify(name)}
              onChange={(e) => {
                setIdTouched(true);
                setId(e.target.value);
              }}
              className="h-11 border border-gray-300 rounded-lg px-4 font-mono text-sm"
              placeholder="espresso_martini"
            />
          </section>

          <section className="flex flex-col gap-3">
            <h3 className="text-sm font-medium text-gray-700">Ingredients</h3>
            {pours.length === 0 ? (
              <p className="text-sm text-gray-500">No ingredients yet. Add one below.</p>
            ) : (
              <ul className="flex flex-col gap-2">
                {pours.map((pour, i) => (
                  <li key={i} className="flex items-center gap-2">
                    {ingredientNames.length > 0 ? (
                      <select
                        value={pour.ingredient}
                        onChange={(e) => updatePour(i, { ingredient: e.target.value })}
                        className="flex-1 h-10 border border-gray-300 rounded-lg px-3 text-sm"
                      >
                        {!ingredientNames.includes(pour.ingredient) && pour.ingredient !== "" && (
                          <option value={pour.ingredient}>{pour.ingredient} (unknown)</option>
                        )}
                        {pour.ingredient === "" && <option value="">Pick an ingredient…</option>}
                        {ingredientNames.map((n) => (
                          <option key={n} value={n}>
                            {n}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <input
                        type="text"
                        value={pour.ingredient}
                        onChange={(e) => updatePour(i, { ingredient: e.target.value })}
                        placeholder="ingredient"
                        className="flex-1 h-10 border border-gray-300 rounded-lg px-3 text-sm"
                      />
                    )}
                    <input
                      type="number"
                      value={pour.oz}
                      onChange={(e) => updatePour(i, { oz: Number(e.target.value) })}
                      step={0.25}
                      min={0}
                      className="w-24 h-10 border border-gray-300 rounded-lg px-3 text-sm"
                    />
                    <span className="text-sm text-gray-500">oz</span>
                    <button
                      type="button"
                      onClick={() => removePour(i)}
                      className="h-10 w-10 rounded border border-gray-300 text-red-600"
                      aria-label="Remove"
                    >
                      ×
                    </button>
                  </li>
                ))}
              </ul>
            )}

            <button
              type="button"
              onClick={addPour}
              className="self-start h-10 px-4 rounded-lg border border-gray-300 text-sm font-medium hover:bg-gray-50"
            >
              Add ingredient
            </button>
          </section>
        </div>

        <footer className="flex items-center justify-end gap-3 px-6 py-4 border-t border-gray-200">
          <button
            type="button"
            onClick={onClose}
            className="h-11 px-5 rounded-lg border border-gray-300 font-medium hover:bg-gray-50"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={onSubmit}
            disabled={busy}
            className="h-11 px-5 rounded-lg bg-black text-white font-medium disabled:opacity-50"
          >
            {busy ? "Saving…" : "Save"}
          </button>
        </footer>
      </div>
    </div>
  );
}
