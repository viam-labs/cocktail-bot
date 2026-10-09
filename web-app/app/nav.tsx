"use client";

import { useSearchParams } from "next/navigation";
import styles from "./nav.module.css";

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
    <nav className={styles.nav}>
      <a href="?" className={styles.brand}>Cocktails</a>
      <div className={styles.links}>
        {TABS.map((tab) => (
          <a
            key={tab.view}
            href={href(tab.view)}
            className={`${styles.link} ${tab.view === current ? styles.on : ""}`}
          >
            {tab.label}
          </a>
        ))}
      </div>
    </nav>
  );
}
