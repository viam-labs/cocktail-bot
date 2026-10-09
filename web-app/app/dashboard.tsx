"use client";

import { useEffect, useState } from "react";
import * as VIAM from "@viamrobotics/sdk";
import Cookies from "js-cookie";
import { listMachines, type Machine } from "./lib/machines";
import styles from "./dashboard.module.css";

function fmtWhen(d: Date | null): string {
  if (!d) return "never";
  const diffMs = Date.now() - d.getTime();
  const mins = Math.floor(diffMs / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

export function Dashboard() {
  const [status, setStatus] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState<string | null>(null);
  const [machines, setMachines] = useState<Machine[]>([]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const raw = Cookies.get("userToken");
        if (!raw) {
          throw new Error("No userToken cookie. Open the webapp from app.viam.com so Viam can log you in.");
        }
        const { access_token } = JSON.parse(raw) as { access_token: string };
        const client = await VIAM.createViamClient({
          credentials: { type: "access-token", payload: access_token },
        });
        const list = await listMachines(client);
        if (cancelled) return;
        setMachines(list);
        setStatus("ready");
      } catch (err) {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : String(err));
        setStatus("error");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className={styles.scope}>
      <main className={styles.page}>
        <h1 className={styles.title}>Cocktails</h1>
        <p className={styles.sub}>Pick a machine to run</p>

        {status === "loading" && <p className={styles.sub}>Loading…</p>}
        {status === "error" && <p className={`${styles.sub} ${styles.err}`}>{error}</p>}
        {status === "ready" && machines.length === 0 && (
          <p className={styles.sub}>No machines in this location.</p>
        )}

        {status === "ready" && machines.length > 0 && (
          <ul className={styles.grid}>
            {machines.map((m) => {
              const href = m.mainPartId ? `?view=machine&partId=${m.mainPartId}` : undefined;
              return (
                <li key={m.id}>
                  <a
                    href={href}
                    className={styles.card}
                    aria-disabled={!href}
                    onClick={(e) => {
                      if (!href) e.preventDefault();
                    }}
                  >
                    <div className={styles.cardHead}>
                      <div>
                        <h2 className={styles.cardName}>{m.name}</h2>
                        <p className={styles.cardLoc}>{m.locationName}</p>
                      </div>
                      <span
                        className={`${styles.statusDot} ${m.online ? styles.statusOn : styles.statusOff}`}
                        aria-label={m.online ? "online" : "offline"}
                      />
                    </div>
                    <div className={styles.cardFoot}>
                      {m.online ? "Online" : `Last online ${fmtWhen(m.lastOnline)}`}
                    </div>
                  </a>
                </li>
              );
            })}
          </ul>
        )}
      </main>
    </div>
  );
}
