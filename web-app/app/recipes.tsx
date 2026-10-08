"use client";

import { useCallback, useEffect, useState } from "react";
import type { ViamConnection } from "./lib/viamClient";
import { getRecipes, updateRecipes } from "./lib/viamClient";
import type { Recipe, RecipeStep } from "./lib/recipes";
import { Nav } from "./nav";

const VERB_OPTIONS: { verb: string; label: string }[] = [
  { verb: "pour_into_shaker", label: "Pour ingredient into shaker" },
  { verb: "dispense_ice", label: "Dispense ice" },
  { verb: "mix", label: "Mix" },
  { verb: "strain_shaker", label: "Strain shaker" },
  { verb: "pour_from_shaker", label: "Pour from shaker" },
  { verb: "rotate_shakers", label: "Rotate shakers (reset)" },
];

function emptyStep(verb: string): RecipeStep {
  switch (verb) {
    case "pour_into_shaker":
      return { verb, bottle: "", oz: 1 };
    case "dispense_ice":
      return { verb, station: "ice-station", dwell_ms: 3000 };
    case "mix":
      return { verb, station: "mixer", dwell_ms: 10000 };
    case "strain_shaker":
      return { verb, source: "ice-station", strain_flow: "strain-flow", drain_dwell_ms: 4000, dump_dwell_ms: 2000 };
    case "pour_from_shaker":
      return { verb, station: "serving-glass-center", pour_ms: 5000 };
    case "rotate_shakers":
      return { verb, strain_flow: "strain-flow" };
    default:
      return { verb };
  }
}

function slugify(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, "_").replace(/^_|_$/g, "");
}

export function RecipesPage({ conn }: { conn: ViamConnection }) {
  const [recipes, setRecipes] = useState<Recipe[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState<{ index: number | null; recipe: Recipe } | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(() => {
    getRecipes(conn)
      .then(setRecipes)
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
    const next = recipes.filter((_, i) => i !== index);
    save(next);
  }

  return (
    <>
      <Nav current="recipes" />
      <main className="max-w-3xl mx-auto p-6 flex flex-col gap-6">
        <header className="flex items-center justify-between">
          <h1 className="text-3xl font-semibold tracking-tight">Recipes</h1>
          <button
            type="button"
            onClick={() =>
              setEditing({ index: null, recipe: { id: "", name: "", steps: [] } })
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
                    {recipe.steps.length} step{recipe.steps.length === 1 ? "" : "s"}
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
  busy,
  onClose,
  onSave,
}: {
  initial: Recipe;
  busy: boolean;
  onClose: () => void;
  onSave: (recipe: Recipe) => void;
}) {
  const [name, setName] = useState(initial.name);
  const [id, setId] = useState(initial.id);
  const [idTouched, setIdTouched] = useState(initial.id !== "");
  const [steps, setSteps] = useState<RecipeStep[]>(initial.steps);
  const [newVerb, setNewVerb] = useState("pour_into_shaker");

  function updateStep(i: number, patch: Partial<RecipeStep>) {
    setSteps((prev) => prev.map((s, idx) => (idx === i ? { ...s, ...patch } : s)));
  }

  function removeStep(i: number) {
    setSteps((prev) => prev.filter((_, idx) => idx !== i));
  }

  function moveStep(i: number, dir: -1 | 1) {
    const j = i + dir;
    if (j < 0 || j >= steps.length) return;
    setSteps((prev) => {
      const next = [...prev];
      [next[i], next[j]] = [next[j], next[i]];
      return next;
    });
  }

  function addStep() {
    setSteps((prev) => [...prev, emptyStep(newVerb)]);
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
    onSave({ id: effectiveId, name: trimmedName, steps });
  }

  return (
    <div className="fixed inset-0 bg-black/40 z-20 flex items-end sm:items-center justify-center p-4">
      <div className="bg-white rounded-2xl w-full max-w-2xl max-h-[90vh] flex flex-col overflow-hidden">
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
            <h3 className="text-sm font-medium text-gray-700">Steps</h3>
            {steps.length === 0 ? (
              <p className="text-sm text-gray-500">No steps yet. Add one below.</p>
            ) : (
              <ul className="flex flex-col gap-3">
                {steps.map((step, i) => (
                  <li key={i} className="border border-gray-200 rounded-lg p-4 flex flex-col gap-3">
                    <div className="flex items-center justify-between">
                      <span className="text-sm font-semibold text-gray-800">
                        {i + 1}. {VERB_OPTIONS.find((v) => v.verb === step.verb)?.label ?? step.verb}
                      </span>
                      <div className="flex gap-1">
                        <button
                          type="button"
                          onClick={() => moveStep(i, -1)}
                          disabled={i === 0}
                          className="h-7 w-7 rounded border border-gray-300 text-sm disabled:opacity-30"
                          aria-label="Move up"
                        >
                          ↑
                        </button>
                        <button
                          type="button"
                          onClick={() => moveStep(i, 1)}
                          disabled={i === steps.length - 1}
                          className="h-7 w-7 rounded border border-gray-300 text-sm disabled:opacity-30"
                          aria-label="Move down"
                        >
                          ↓
                        </button>
                        <button
                          type="button"
                          onClick={() => removeStep(i)}
                          className="h-7 w-7 rounded border border-gray-300 text-sm text-red-600"
                          aria-label="Remove"
                        >
                          ×
                        </button>
                      </div>
                    </div>
                    <StepFields step={step} onChange={(patch) => updateStep(i, patch)} />
                  </li>
                ))}
              </ul>
            )}

            <div className="flex gap-2">
              <select
                value={newVerb}
                onChange={(e) => setNewVerb(e.target.value)}
                className="flex-1 h-11 border border-gray-300 rounded-lg px-3"
              >
                {VERB_OPTIONS.map((v) => (
                  <option key={v.verb} value={v.verb}>
                    {v.label}
                  </option>
                ))}
              </select>
              <button
                type="button"
                onClick={addStep}
                className="h-11 px-5 rounded-lg border border-gray-300 font-medium hover:bg-gray-50"
              >
                Add step
              </button>
            </div>
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

function StepFields({
  step,
  onChange,
}: {
  step: RecipeStep;
  onChange: (patch: Partial<RecipeStep>) => void;
}) {
  switch (step.verb) {
    case "pour_into_shaker":
      return (
        <div className="grid grid-cols-2 gap-3">
          <TextField label="Bottle" value={step.bottle ?? ""} onChange={(v) => onChange({ bottle: v })} placeholder="vodka" />
          <NumberField label="Ounces" value={step.oz ?? 0} onChange={(v) => onChange({ oz: v })} step={0.25} />
        </div>
      );
    case "dispense_ice":
      return (
        <div className="grid grid-cols-2 gap-3">
          <TextField label="Station" value={step.station ?? ""} onChange={(v) => onChange({ station: v })} placeholder="ice-station" />
          <NumberField label="Dwell (ms)" value={step.dwell_ms ?? 0} onChange={(v) => onChange({ dwell_ms: v })} step={500} />
        </div>
      );
    case "mix":
      return (
        <div className="grid grid-cols-2 gap-3">
          <TextField label="Station" value={step.station ?? ""} onChange={(v) => onChange({ station: v })} placeholder="mixer" />
          <NumberField label="Dwell (ms)" value={step.dwell_ms ?? 0} onChange={(v) => onChange({ dwell_ms: v })} step={1000} />
        </div>
      );
    case "strain_shaker":
      return (
        <div className="grid grid-cols-2 gap-3">
          <TextField label="Source" value={step.source ?? ""} onChange={(v) => onChange({ source: v })} placeholder="ice-station" />
          <TextField label="Strain flow" value={step.strain_flow ?? ""} onChange={(v) => onChange({ strain_flow: v })} placeholder="strain-flow" />
          <NumberField label="Drain dwell (ms)" value={step.drain_dwell_ms ?? 0} onChange={(v) => onChange({ drain_dwell_ms: v })} step={500} />
          <NumberField label="Dump dwell (ms)" value={step.dump_dwell_ms ?? 0} onChange={(v) => onChange({ dump_dwell_ms: v })} step={500} />
        </div>
      );
    case "pour_from_shaker":
      return (
        <div className="grid grid-cols-2 gap-3">
          <TextField label="Station" value={step.station ?? ""} onChange={(v) => onChange({ station: v })} placeholder="serving-glass-center" />
          <NumberField label="Pour (ms)" value={step.pour_ms ?? 0} onChange={(v) => onChange({ pour_ms: v })} step={500} />
        </div>
      );
    case "rotate_shakers":
      return (
        <div className="grid grid-cols-2 gap-3">
          <TextField label="Strain flow" value={step.strain_flow ?? ""} onChange={(v) => onChange({ strain_flow: v })} placeholder="strain-flow" />
        </div>
      );
    default:
      return <p className="text-sm text-gray-500">Unknown step type.</p>;
  }
}

function TextField({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
}) {
  return (
    <label className="flex flex-col gap-1">
      <span className="text-xs text-gray-600">{label}</span>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="h-10 border border-gray-300 rounded-lg px-3 text-sm"
      />
    </label>
  );
}

function NumberField({
  label,
  value,
  onChange,
  step,
}: {
  label: string;
  value: number;
  onChange: (v: number) => void;
  step: number;
}) {
  return (
    <label className="flex flex-col gap-1">
      <span className="text-xs text-gray-600">{label}</span>
      <input
        type="number"
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        step={step}
        className="h-10 border border-gray-300 rounded-lg px-3 text-sm"
      />
    </label>
  );
}
