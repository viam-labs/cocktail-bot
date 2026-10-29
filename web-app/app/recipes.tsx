"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  DndContext,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import { SortableContext, arrayMove, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import type { ViamConnection } from "./lib/viamClient";
import { getInventory, getRecipes, updateRecipes } from "./lib/viamClient";
import type { Recipe } from "./lib/recipes";
import { COMMON_RECIPE_SECTIONS, COMMON_RECIPE_TAGS, DRINK_IMAGE_SLUGS, recipePours, stepsFor } from "./lib/recipes";
import type { MixMode, RecipeStep } from "./lib/recipes";
import type { Inventory } from "./lib/inventory";
import { isAvailable } from "./lib/inventory";
import { Nav } from "./nav";
import { titleCase as cap } from "./lib/display";
import styles from "./recipes.module.css";

type IdStep = { id: string; step: RecipeStep };

const DEFAULT_SECTION = "Other";

function slugify(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, "_").replace(/^_|_$/g, "");
}

function fmtOz(v: number): string {
  return (Math.round(v * 100) / 100).toString();
}

type SectionGroup = {
  section: string;
  items: { recipe: Recipe; index: number }[];
};

function orderOf(recipe: Recipe): number {
  return recipe.sort_order ?? Number.POSITIVE_INFINITY;
}

function groupRecipesBySection(recipes: Recipe[]): SectionGroup[] {
  const groups = new Map<string, { recipe: Recipe; index: number }[]>();
  recipes.forEach((recipe, index) => {
    const section = recipe.section?.trim() || DEFAULT_SECTION;
    const list = groups.get(section) ?? [];
    list.push({ recipe, index });
    groups.set(section, list);
  });
  for (const list of groups.values()) {
    list.sort((a, b) => {
      const oa = orderOf(a.recipe);
      const ob = orderOf(b.recipe);
      if (oa !== ob) return oa - ob;
      return a.recipe.name.localeCompare(b.recipe.name);
    });
  }
  return [...groups.entries()]
    .sort(([aSection, aList], [bSection, bList]) => {
      if (aSection === DEFAULT_SECTION && bSection !== DEFAULT_SECTION) return 1;
      if (bSection === DEFAULT_SECTION && aSection !== DEFAULT_SECTION) return -1;
      const aMin = Math.min(...aList.map((x) => orderOf(x.recipe)));
      const bMin = Math.min(...bList.map((x) => orderOf(x.recipe)));
      if (aMin !== bMin) return aMin - bMin;
      return aSection.localeCompare(bSection);
    })
    .map(([section, items]) => ({ section, items }));
}

type RecipeStatus =
  | { kind: "loading" }
  | { kind: "off_menu" }
  | { kind: "ready" }
  | { kind: "missing"; ingredient: string };

function recipeStatus(recipe: Recipe, inventory: Inventory | null): RecipeStatus {
  if (recipe.on_menu === false) return { kind: "off_menu" };
  if (!inventory) return { kind: "loading" };
  for (const pour of recipePours(recipe)) {
    const stock = inventory.ingredients[pour.ingredient];
    if (!stock || !stock.in_stock) return { kind: "missing", ingredient: pour.ingredient };
  }
  return { kind: "ready" };
}

export function RecipesPage({ conn }: { conn: ViamConnection }) {
  const [recipes, setRecipes] = useState<Recipe[] | null>(null);
  const [inventory, setInventory] = useState<Inventory | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState<{ index: number | null; recipe: Recipe } | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(() => {
    Promise.all([getRecipes(conn), getInventory(conn)])
      .then(([r, inv]) => {
        setRecipes(r);
        setInventory(inv);
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

  const ingredientNames = useMemo(
    () => (inventory ? Object.keys(inventory.ingredients).sort() : []),
    [inventory],
  );

  const sections = useMemo(() => groupRecipesBySection(recipes ?? []), [recipes]);
  const readyCount = useMemo(() => {
    if (!recipes || !inventory) return 0;
    return recipes.filter((r) => isAvailable(r, inventory)).length;
  }, [recipes, inventory]);

  return (
    <>
      <Nav current="recipes" />
      <div className={styles.scope}>
        <main className={styles.page}>
          <div className={styles.toolbar}>
            <div className={styles.toolbarTitle}>
              <h1 className={styles.pageTitle}>Recipes</h1>
              {recipes && (
                <p className={styles.toolbarMeta}>
                  {recipes.length} recipe{recipes.length === 1 ? "" : "s"}
                  {inventory ? ` · ${readyCount} ready to make` : ""}
                </p>
              )}
            </div>
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
            sections.map(({ section, items }) => (
              <section key={section} className={styles.recipeSection} aria-label={section}>
                <h2 className={styles.recipeSectionTitle}>{section}</h2>
                <div className={styles.recipeGrid}>
                  {items.map(({ recipe, index }) => (
                    <RecipeCard
                      key={recipe.id || index}
                      recipe={recipe}
                      status={recipeStatus(recipe, inventory)}
                      onClick={() => setEditing({ index, recipe: structuredClone(recipe) })}
                    />
                  ))}
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
          inventory={inventory}
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

function RecipeArt({ slug, name }: { slug?: string; name: string }) {
  if (!slug) return <div className={styles.recipeArt} aria-hidden />;
  return (
    <div className={styles.recipeArt}>
      <picture>
        <source srcSet={`/drinks/png/dark/${slug}.png`} media="(prefers-color-scheme: dark)" />
        <img src={`/drinks/png/light/${slug}.png`} alt="" width={74} height={74} aria-label={name} />
      </picture>
    </div>
  );
}

function RecipeCard({
  recipe,
  status,
  onClick,
}: {
  recipe: Recipe;
  status: RecipeStatus;
  onClick: () => void;
}) {
  const pours = recipePours(recipe)
    .map((p) => `${fmtOz(p.oz)} oz ${cap(p.ingredient)}`)
    .join(" · ");
  return (
    <button type="button" className={styles.recipeCard} onClick={onClick}>
      <RecipeArt slug={recipe.image} name={recipe.name || recipe.id} />
      <span className={styles.recipeBody}>
        <span className={styles.recipeNameRow}>
          <span className={styles.recipeName}>{recipe.name || recipe.id || "(unnamed)"}</span>
          {recipe.tag && <span className={styles.tagBadge}>{recipe.tag}</span>}
        </span>
        <span className={styles.recipePours}>{pours || "No pours"}</span>
        <RecipeStatusLine status={status} />
      </span>
      <span className={styles.recipeChev} aria-hidden>
        ›
      </span>
    </button>
  );
}

function RecipeStatusLine({ status }: { status: RecipeStatus }) {
  if (status.kind === "off_menu") {
    return (
      <span className={`${styles.recipeStatus} ${styles.recipeStatusMuted}`}>
        <span className={styles.statusDot} style={{ background: "var(--smoke)" }} aria-hidden />
        Off the menu
      </span>
    );
  }
  if (status.kind === "loading") {
    return (
      <span className={`${styles.recipeStatus} ${styles.recipeStatusMuted}`}>
        <span className={styles.statusDot} style={{ background: "var(--smoke)" }} aria-hidden />
        On the menu
      </span>
    );
  }
  if (status.kind === "ready") {
    return (
      <span className={styles.recipeStatus}>
        <span className={styles.statusDot} style={{ background: "var(--ok)" }} aria-hidden />
        On the menu · ready to make
      </span>
    );
  }
  return (
    <span className={`${styles.recipeStatus} ${styles.recipeStatusWarn}`}>
      <svg
        width="15"
        height="15"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2.2"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden
      >
        <path d="M12 3l9.5 17h-19z" />
        <path d="M12 10v4" />
        <path d="M12 17.5v.5" />
      </svg>
      On the menu · can&rsquo;t make: {cap(status.ingredient)} not on the rack
    </span>
  );
}

function slotLabel(slotId: string): string {
  const parts = slotId.split("-");
  const letter = (parts[1] ?? "").toUpperCase();
  return letter ? `Slot ${letter}` : slotId;
}

function tiltsLine(oz: number, pourerOz: number | undefined): string | null {
  if (pourerOz == null || pourerOz <= 0 || oz <= 0) return null;
  const raw = oz / pourerOz;
  const rounded = Math.round(raw);
  if (Math.abs(raw - rounded) > 0.05) return null;
  return `${rounded} tilt${rounded === 1 ? "" : "s"} of the ${fmtOz(pourerOz)} oz pourer`;
}

function SortableStepRow({
  id,
  index,
  step,
  ingredientNames,
  inventory,
  onUpdate,
  onRemove,
}: {
  id: string;
  index: number;
  step: RecipeStep;
  ingredientNames: string[];
  inventory: Inventory | null;
  onUpdate: (patch: Partial<RecipeStep>) => void;
  onRemove: () => void;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };
  return (
    <div ref={setNodeRef} style={style} className={styles.pourRow}>
      <span
        className={styles.stepGrip}
        aria-label={`Reorder step ${index + 1}`}
        title="Drag to reorder"
        {...attributes}
        {...listeners}
      >
        ⠿
      </span>
      <div className={styles.pourMain}>
        <StepBody step={step} ingredientNames={ingredientNames} inventory={inventory} onUpdate={onUpdate} />
      </div>
      <button type="button" className={styles.rm} onClick={onRemove} aria-label="Remove step">
        ✕
      </button>
    </div>
  );
}

function StepBody({
  step,
  ingredientNames,
  inventory,
  onUpdate,
}: {
  step: RecipeStep;
  ingredientNames: string[];
  inventory: Inventory | null;
  onUpdate: (patch: Partial<RecipeStep>) => void;
}) {
  if (step.kind === "pour") {
    const stock = inventory?.ingredients[step.ingredient];
    const slot = stock?.slot ? slotLabel(stock.slot) : null;
    const tilts = tiltsLine(step.oz, stock?.pourer_oz);
    return (
      <>
        <div className={styles.stepPourRow}>
          {ingredientNames.length > 0 ? (
            <select
              className={styles.select}
              value={step.ingredient}
              onChange={(e) => onUpdate({ ingredient: e.target.value })}
              aria-label="Pour ingredient"
            >
              {!ingredientNames.includes(step.ingredient) && step.ingredient !== "" && (
                <option value={step.ingredient}>{cap(step.ingredient)} (unknown)</option>
              )}
              {step.ingredient === "" && <option value="">Pick…</option>}
              {ingredientNames.map((n) => {
                const nStock = inventory?.ingredients[n];
                const nSlot = nStock?.slot ? slotLabel(nStock.slot) : null;
                return (
                  <option key={n} value={n}>
                    {cap(n)}
                    {nSlot ? ` · ${nSlot}` : ""}
                  </option>
                );
              })}
            </select>
          ) : (
            <input
              className={styles.input}
              type="text"
              value={step.ingredient}
              onChange={(e) => onUpdate({ ingredient: e.target.value })}
              placeholder="ingredient"
              aria-label="Pour ingredient"
            />
          )}
          <input
            className={styles.input}
            type="number"
            min={0}
            step={0.25}
            value={step.oz}
            onChange={(e) => onUpdate({ oz: Math.max(0, Number(e.target.value)) })}
            aria-label="Ounces"
          />
        </div>
        {tilts && <span className={styles.pourTilt}>{tilts}</span>}
        {!tilts && slot && <span className={styles.pourTilt}>Lives in {slot}</span>}
      </>
    );
  }
  if (step.kind === "ice") {
    return <span className={styles.stepLabel}>Crushed ice → shaker</span>;
  }
  if (step.kind === "mix") {
    return <span className={styles.stepLabel}>{step.mode === "stir" ? "Stir" : "Shake"}</span>;
  }
  return <span className={styles.stepLabel}>Strain</span>;
}

function AddStepMenu({ onPick }: { onPick: (kind: RecipeStep["kind"], mode?: MixMode) => void }) {
  const [open, setOpen] = useState(false);
  function pick(kind: RecipeStep["kind"], mode?: MixMode) {
    onPick(kind, mode);
    setOpen(false);
  }
  return (
    <div className={styles.addStep}>
      <button type="button" className={styles.btn} onClick={() => setOpen((v) => !v)}>
        + Add step
      </button>
      {open && (
        <div className={styles.addStepMenu} role="menu">
          <button type="button" className={styles.addStepItem} onClick={() => pick("pour")}>
            Pour
          </button>
          <button type="button" className={styles.addStepItem} onClick={() => pick("ice")}>
            Crushed ice
          </button>
          <button type="button" className={styles.addStepItem} onClick={() => pick("mix", "shake")}>
            Shake
          </button>
          <button type="button" className={styles.addStepItem} onClick={() => pick("mix", "stir")}>
            Stir
          </button>
          <button type="button" className={styles.addStepItem} onClick={() => pick("strain")}>
            Strain
          </button>
        </div>
      )}
    </div>
  );
}

function RecipeEditor({
  initial,
  existingIndex,
  ingredientNames,
  inventory,
  busy,
  onClose,
  onSave,
  onDelete,
}: {
  initial: Recipe;
  existingIndex: number | null;
  ingredientNames: string[];
  inventory: Inventory | null;
  busy: boolean;
  onClose: () => void;
  onSave: (recipe: Recipe) => void;
  onDelete: () => void;
}) {
  const [name, setName] = useState(initial.name);
  const [onMenu, setOnMenu] = useState(initial.on_menu !== false);
  const [image, setImage] = useState(initial.image ?? "");
  const [section, setSection] = useState(initial.section ?? "");
  const [tag, setTag] = useState(initial.tag ?? "");
  const [sortOrder, setSortOrder] = useState<string>(
    initial.sort_order != null ? String(initial.sort_order) : "",
  );
  const [steps, setSteps] = useState<IdStep[]>(() => {
    const existing = stepsFor(initial);
    const seed =
      existing.length > 0
        ? existing
        : [
            { kind: "ice", type: "crushed" } as RecipeStep,
            { kind: "pour", ingredient: ingredientNames[0] ?? "", oz: 1 } as RecipeStep,
            { kind: "mix", mode: "shake" } as RecipeStep,
            { kind: "strain" } as RecipeStep,
          ];
    return seed.map((step) => ({ id: crypto.randomUUID(), step }));
  });

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
  );

  const effectiveId = slugify(name) || initial.id;
  const totalOz = steps
    .filter(
      (x): x is { id: string; step: Extract<RecipeStep, { kind: "pour" }> } => x.step.kind === "pour",
    )
    .reduce((sum, x) => sum + (Number.isFinite(x.step.oz) ? x.step.oz : 0), 0);

  function updateStep(id: string, patch: Partial<RecipeStep>) {
    setSteps((prev) =>
      prev.map((x) =>
        x.id === id ? { ...x, step: { ...x.step, ...patch } as RecipeStep } : x,
      ),
    );
  }

  function removeStep(id: string) {
    setSteps((prev) => prev.filter((x) => x.id !== id));
  }

  function addStep(kind: RecipeStep["kind"], mode: MixMode = "shake") {
    const step: RecipeStep =
      kind === "pour"
        ? { kind: "pour", ingredient: ingredientNames[0] ?? "", oz: 1 }
        : kind === "ice"
        ? { kind: "ice", type: "crushed" }
        : kind === "mix"
        ? { kind: "mix", mode }
        : { kind: "strain" };
    setSteps((prev) => [...prev, { id: crypto.randomUUID(), step }]);
  }

  function handleDragEnd(event: DragEndEvent) {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    setSteps((prev) => {
      const oldIdx = prev.findIndex((x) => x.id === active.id);
      const newIdx = prev.findIndex((x) => x.id === over.id);
      if (oldIdx < 0 || newIdx < 0) return prev;
      return arrayMove(prev, oldIdx, newIdx);
    });
  }

  function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    const trimmedName = name.trim();
    if (!trimmedName) {
      alert("Recipe name is required");
      return;
    }
    if (!effectiveId) {
      alert("Recipe id is required");
      return;
    }
    const next: Recipe = {
      id: effectiveId,
      name: trimmedName,
      steps: steps.map((x) => x.step),
      on_menu: onMenu,
    };
    if (image) next.image = image;
    const trimmedSection = section.trim();
    if (trimmedSection) next.section = trimmedSection;
    const trimmedTag = tag.trim();
    if (trimmedTag) next.tag = trimmedTag;
    const parsedOrder = Number(sortOrder);
    if (sortOrder !== "" && Number.isFinite(parsedOrder)) next.sort_order = parsedOrder;
    onSave(next);
  }

  return (
    <div className={styles.scope}>
      <div className={styles.scrim} onClick={onClose} />
      <form className={styles.panel} onSubmit={onSubmit}>
        <div className={styles.editorHead}>
          <div className={styles.editorHeadLeft}>
            <RecipeArt slug={image} name={name || "New recipe"} />
            <div className={styles.editorTitleGroup}>
              <h2 className={styles.editorTitle}>{name.trim() || "New recipe"}</h2>
              <p className={styles.editorIdHint}>
                ID: {effectiveId || "—"} <span className={styles.muted}>(set automatically)</span>
              </p>
            </div>
          </div>
          <button type="button" className={styles.closeRound} onClick={onClose} aria-label="Close">
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

          <div className={styles.editorColumns}>
            <label>
              <span>Kiosk section</span>
              <select
                className={styles.select}
                value={section}
                onChange={(e) => setSection(e.target.value)}
              >
                <option value="">— None —</option>
                {COMMON_RECIPE_SECTIONS.map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
                {section && !COMMON_RECIPE_SECTIONS.includes(section) && (
                  <option value={section}>{section}</option>
                )}
              </select>
            </label>

            <label>
              <span>Tag</span>
              <select className={styles.select} value={tag} onChange={(e) => setTag(e.target.value)}>
                <option value="">— None —</option>
                {COMMON_RECIPE_TAGS.map((t) => (
                  <option key={t} value={t}>
                    {t}
                  </option>
                ))}
                {tag && !COMMON_RECIPE_TAGS.includes(tag) && <option value={tag}>{tag}</option>}
              </select>
            </label>
          </div>

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

          <label>
            <span>
              Sort order <span className={styles.muted}>(lower shows first)</span>
            </span>
            <input
              className={styles.input}
              type="number"
              step={1}
              value={sortOrder}
              onChange={(e) => setSortOrder(e.target.value)}
              placeholder="0"
            />
          </label>

          <div className={styles.switchRow}>
            <span>On the menu</span>
            <button
              type="button"
              role="switch"
              aria-checked={onMenu}
              className={styles.switch}
              onClick={() => setOnMenu((v) => !v)}
            >
              <span className={styles.switchThumb} />
            </button>
          </div>

          <div className={styles.pourSection}>
            <div className={styles.pourSectionHead}>
              <h3 className={styles.pourSectionTitle}>Steps, in order</h3>
              <span className={styles.pourTotal}>{fmtOz(totalOz)} oz in the glass</span>
            </div>

            <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
              <SortableContext items={steps.map((x) => x.id)} strategy={verticalListSortingStrategy}>
                {steps.map((item, i) => (
                  <SortableStepRow
                    key={item.id}
                    id={item.id}
                    index={i}
                    step={item.step}
                    ingredientNames={ingredientNames}
                    inventory={inventory}
                    onUpdate={(patch) => updateStep(item.id, patch)}
                    onRemove={() => removeStep(item.id)}
                  />
                ))}
              </SortableContext>
            </DndContext>

            <div className={styles.pourAddRow}>
              <AddStepMenu onPick={addStep} />
            </div>
          </div>
        </div>

        <div className={styles.dlgBtns}>
          {existingIndex !== null && (
            <button
              type="button"
              className={styles.btnDangerQuiet}
              onClick={onDelete}
              disabled={busy}
            >
              Delete recipe…
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
