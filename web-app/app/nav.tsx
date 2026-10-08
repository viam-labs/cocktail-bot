"use client";

import { useSearchParams } from "next/navigation";

type View = "kiosk" | "inventory" | "recipes" | "admin";

const TABS: { view: View; label: string }[] = [
  { view: "kiosk", label: "Kiosk" },
  { view: "inventory", label: "Inventory" },
  { view: "recipes", label: "Recipes" },
];

export function Nav({ current }: { current: View }) {
  const params = useSearchParams();
  const partId = params.get("partId") ?? "";

  const href = (view: View) => {
    const p = new URLSearchParams();
    if (partId) p.set("partId", partId);
    if (view !== "kiosk") p.set("view", view);
    const query = p.toString();
    return query ? `?${query}` : "?";
  };

  return (
    <nav className="border-b border-gray-200 bg-white sticky top-0 z-10">
      <div className="max-w-5xl mx-auto flex items-center gap-1 px-4 h-14">
        <span className="text-sm font-semibold tracking-tight mr-6">Cocktails</span>
        {TABS.map((tab) => {
          const active = tab.view === current;
          return (
            <a
              key={tab.view}
              href={href(tab.view)}
              className={`px-4 h-10 flex items-center rounded-lg text-sm font-medium ${
                active ? "bg-gray-900 text-white" : "text-gray-700 hover:bg-gray-100"
              }`}
            >
              {tab.label}
            </a>
          );
        })}
      </div>
    </nav>
  );
}
