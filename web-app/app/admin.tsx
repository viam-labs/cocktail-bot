"use client";

import { useEffect, useState } from "react";
import type { ViamConnection } from "./lib/viamClient";
import { getRecipes, updateRecipes } from "./lib/viamClient";
import type { Recipe } from "./lib/recipes";

type SaveState =
  | { kind: "idle" }
  | { kind: "saving" }
  | { kind: "saved"; count: number }
  | { kind: "error"; message: string };

export function Admin({ conn }: { conn: ViamConnection }) {
  const [text, setText] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [save, setSave] = useState<SaveState>({ kind: "idle" });

  useEffect(() => {
    let cancelled = false;
    getRecipes(conn)
      .then((recipes) => {
        if (cancelled) return;
        setText(JSON.stringify(recipes, null, 2));
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setLoadError(err instanceof Error ? err.message : String(err));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [conn]);

  async function onSave() {
    setSave({ kind: "saving" });
    let recipes: Recipe[];
    try {
      const parsed = JSON.parse(text);
      if (!Array.isArray(parsed)) {
        throw new Error("Top-level JSON must be an array of recipes");
      }
      recipes = parsed as Recipe[];
    } catch (err) {
      setSave({ kind: "error", message: err instanceof Error ? err.message : String(err) });
      return;
    }
    try {
      const resp = await updateRecipes(conn, recipes);
      setSave({ kind: "saved", count: resp.count });
    } catch (err) {
      setSave({ kind: "error", message: err instanceof Error ? err.message : String(err) });
    }
  }

  return (
    <main className="min-h-screen flex flex-col p-8 gap-6 max-w-4xl mx-auto">
      <header className="flex items-center justify-between">
        <h1 className="text-3xl font-semibold tracking-tight">Recipes</h1>
        <a href="?partId=" className="text-sm text-gray-600 underline">Back to kiosk</a>
      </header>
      <p className="text-sm text-gray-600">
        A JSON array of recipes. Save replaces the current list on the machine.
      </p>

      {loading ? (
        <p className="text-gray-600">Loading…</p>
      ) : loadError ? (
        <p className="text-red-600">Could not load recipes: {loadError}</p>
      ) : (
        <>
          <textarea
            value={text}
            onChange={(e) => setText(e.target.value)}
            spellCheck={false}
            className="w-full h-[32rem] border border-gray-300 rounded-lg p-4 font-mono text-sm"
          />
          <div className="flex items-center gap-4">
            <button
              type="button"
              onClick={onSave}
              disabled={save.kind === "saving"}
              className="h-12 px-6 rounded-lg bg-black text-white font-medium disabled:opacity-50"
            >
              {save.kind === "saving" ? "Saving…" : "Save"}
            </button>
            {save.kind === "saved" && (
              <span className="text-green-700">Saved {save.count} recipe{save.count === 1 ? "" : "s"}</span>
            )}
            {save.kind === "error" && (
              <span className="text-red-600 break-words">{save.message}</span>
            )}
          </div>
        </>
      )}
    </main>
  );
}
