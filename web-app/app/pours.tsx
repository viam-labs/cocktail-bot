"use client";

import { useCallback, useEffect, useState } from "react";
import type { ViamConnection } from "./lib/viamClient";
import { getGlasses, getPourMenus, updatePourMenus } from "./lib/viamClient";
import type { Glass } from "./lib/glasses";
import type { PourMenu, PourServe, PourSize } from "./lib/pour_menus";
import { slugifyMenuId } from "./lib/pour_menus";
import { Nav } from "./nav";
import styles from "./pours.module.css";

export function PoursPage({ conn }: { conn: ViamConnection }) {
  const [menus, setMenus] = useState<PourMenu[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState<number | null>(null);
  const [glasses, setGlasses] = useState<Glass[]>([]);

  useEffect(() => {
    let cancelled = false;
    getPourMenus(conn)
      .then((m) => !cancelled && setMenus(m))
      .catch((err: unknown) => !cancelled && setError(err instanceof Error ? err.message : String(err)));
    getGlasses(conn)
      .then((g) => !cancelled && setGlasses(g))
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [conn]);

  const save = useCallback(
    async (next: PourMenu[]) => {
      setMenus(next);
      try {
        await updatePourMenus(conn, next);
      } catch (err: unknown) {
        setError(err instanceof Error ? err.message : String(err));
      }
    },
    [conn],
  );

  const addMenu = () => {
    const base: PourMenu = {
      id: `menu-${Date.now()}`,
      name: "New pour menu",
      category: "",
      on_menu: false,
      sizes: [{ label: "Pour", oz: 2 }],
      default_size: "Pour",
      serves: [{ label: "Serve", glass: "" }],
      traits: [],
    };
    const next = [...(menus ?? []), base];
    save(next);
    setEditing(next.length - 1);
  };

  const update = (index: number, patch: Partial<PourMenu>) => {
    if (!menus) return;
    const next = menus.slice();
    next[index] = { ...next[index], ...patch };
    save(next);
  };

  const remove = (index: number) => {
    if (!menus) return;
    const next = menus.filter((_, i) => i !== index);
    save(next);
    setEditing(null);
  };

  return (
    <>
      <Nav current="pours" />
      <div className={styles.scope}>
        <header className={styles.head}>
          <h1 className={styles.title}>Pours</h1>
          <p className={styles.sub}>
            Straight pours, no mixing. Each menu sets the sizes and how it&rsquo;s served; the bottles and their
            info come from Inventory.
          </p>
          <button type="button" className={styles.primary} onClick={addMenu}>
            New pour menu
          </button>
        </header>

        {error && <p className={styles.error}>{error}</p>}
        {!menus && !error && <p className={styles.loading}>Loading…</p>}

        {menus && (
          <div className={styles.grid}>
            {menus.map((menu, i) => (
              <MenuCard
                key={menu.id || i}
                menu={menu}
                onEdit={() => setEditing(i)}
              />
            ))}
          </div>
        )}

        {menus && editing !== null && menus[editing] && (
          <MenuEditor
            menu={menus[editing]}
            glasses={glasses}
            onChange={(patch) => update(editing, patch)}
            onClose={() => setEditing(null)}
            onDelete={() => remove(editing)}
          />
        )}
      </div>
    </>
  );
}

function MenuCard({ menu, onEdit }: { menu: PourMenu; onEdit: () => void }) {
  const sizes = menu.sizes.map((s) => `${s.label} ${s.oz} oz`).join(" or ");
  const serves = menu.serves.map((s) => s.label).join(", ");
  return (
    <button type="button" className={styles.card} onClick={onEdit}>
      <div className={styles.cardHead}>
        <span className={styles.cardName}>{menu.name}</span>
        <span className={styles.cardCat}>{menu.category || "no category"}</span>
      </div>
      <div className={styles.cardLine}>
        {sizes}
        {serves && ` · ${serves}`}
      </div>
      <div className={`${styles.cardStatus} ${menu.on_menu ? styles.on : styles.off}`}>
        {menu.on_menu ? "On the menu" : "Hidden"}
      </div>
    </button>
  );
}

function MenuEditor({
  menu,
  glasses,
  onChange,
  onClose,
  onDelete,
}: {
  menu: PourMenu;
  glasses: Glass[];
  onChange: (patch: Partial<PourMenu>) => void;
  onClose: () => void;
  onDelete: () => void;
}) {
  const updateSize = (i: number, patch: Partial<PourSize>) => {
    const sizes = menu.sizes.slice();
    sizes[i] = { ...sizes[i], ...patch };
    onChange({ sizes });
  };
  const addSize = () => onChange({ sizes: [...menu.sizes, { label: "", oz: 1 }] });
  const removeSize = (i: number) => onChange({ sizes: menu.sizes.filter((_, j) => j !== i) });

  const updateServe = (i: number, patch: Partial<PourServe>) => {
    const serves = menu.serves.slice();
    serves[i] = { ...serves[i], ...patch };
    onChange({ serves });
  };
  const addServe = () =>
    onChange({ serves: [...menu.serves, { label: "", glass: "", with_ice: false }] });
  const removeServe = (i: number) => onChange({ serves: menu.serves.filter((_, j) => j !== i) });

  return (
    <>
      <div className={styles.scrim} onClick={onClose} />
      <aside className={styles.panel}>
        <header className={styles.panelHead}>
          <h2>{menu.name || "New pour menu"}</h2>
          <button type="button" className={styles.close} onClick={onClose}>
            ×
          </button>
        </header>

        <label className={styles.field}>
          <span>Menu name</span>
          <input
            type="text"
            value={menu.name}
            onChange={(e) => onChange({ name: e.target.value, id: menu.id || slugifyMenuId(e.target.value) })}
          />
        </label>

        <label className={styles.field}>
          <span>Bottles from category</span>
          <input
            type="text"
            value={menu.category}
            onChange={(e) => onChange({ category: e.target.value })}
            placeholder="Whiskey, Wine, ..."
          />
        </label>

        <label className={styles.checkbox}>
          <input type="checkbox" checked={!!menu.on_menu} onChange={(e) => onChange({ on_menu: e.target.checked })} />
          <span>On the menu</span>
        </label>

        <label className={styles.field}>
          <span>Intro line (optional)</span>
          <input
            type="text"
            value={menu.intro ?? ""}
            onChange={(e) => onChange({ intro: e.target.value })}
            placeholder="Japanese whisky tasting. Start light, finish smoky."
          />
        </label>

        <section className={styles.section}>
          <h3>Sizes</h3>
          {menu.sizes.map((s, i) => (
            <div key={i} className={styles.row}>
              <input
                type="text"
                value={s.label}
                onChange={(e) => updateSize(i, { label: e.target.value })}
                placeholder="Label"
              />
              <input
                type="number"
                step={0.25}
                value={s.oz}
                onChange={(e) => updateSize(i, { oz: Math.max(0, Number(e.target.value)) })}
              />
              <button
                type="button"
                className={menu.default_size === s.label ? styles.toggleOn : styles.toggle}
                onClick={() => onChange({ default_size: s.label })}
              >
                Default
              </button>
              <button type="button" className={styles.remove} onClick={() => removeSize(i)}>
                ×
              </button>
            </div>
          ))}
          <button type="button" className={styles.addRow} onClick={addSize}>
            Add a size
          </button>
        </section>

        <section className={styles.section}>
          <h3>Serve options</h3>
          {menu.serves.map((s, i) => (
            <div key={i} className={styles.serveRow}>
              <input
                type="text"
                value={s.label}
                onChange={(e) => updateServe(i, { label: e.target.value })}
                placeholder="Label (e.g. Neat)"
              />
              <select value={s.glass ?? ""} onChange={(e) => updateServe(i, { glass: e.target.value })}>
                <option value="">No glass</option>
                {glasses.map((g) => (
                  <option key={g.id} value={g.id}>
                    {g.name}
                  </option>
                ))}
                {s.glass && !glasses.some((g) => g.id === s.glass) && <option value={s.glass}>{s.glass}</option>}
              </select>
              <label className={styles.checkbox}>
                <input
                  type="checkbox"
                  checked={!!s.with_ice}
                  onChange={(e) => updateServe(i, { with_ice: e.target.checked })}
                />
                <span>With ice</span>
              </label>
              <button type="button" className={styles.remove} onClick={() => removeServe(i)}>
                ×
              </button>
            </div>
          ))}
          <button type="button" className={styles.addRow} onClick={addServe}>
            Add a serve option
          </button>
        </section>

        <section className={styles.section}>
          <h3>Flavor traits</h3>
          <input
            type="text"
            value={(menu.traits ?? []).join(", ")}
            onChange={(e) =>
              onChange({
                traits: e.target.value
                  .split(",")
                  .map((t) => t.trim())
                  .filter(Boolean),
              })
            }
            placeholder="Body, Sweetness, Acidity, Tannin, Fruit"
          />
        </section>

        <footer className={styles.panelFoot}>
          <button type="button" className={styles.danger} onClick={onDelete}>
            Delete menu
          </button>
        </footer>
      </aside>
    </>
  );
}
