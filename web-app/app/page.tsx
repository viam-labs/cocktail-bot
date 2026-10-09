"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { useViamConnection } from "./lib/useViamConnection";
import { Kiosk } from "./kiosk";
import { Admin } from "./admin";
import { InventoryPage } from "./inventory";
import { RecipesPage } from "./recipes";
import { Dashboard } from "./dashboard";

function PageInner() {
  const params = useSearchParams();
  const partId = params.get("partId") ?? "";
  const view = params.get("view") ?? "";
  const { conn, connected, error } = useViamConnection(partId);

  if (!partId) {
    return <Dashboard />;
  }

  if (error) {
    return (
      <main className="flex min-h-screen flex-col items-center justify-center gap-4 p-8">
        <h1 className="text-4xl font-semibold tracking-tight">Cocktails</h1>
        <p className="text-red-600">{error}</p>
      </main>
    );
  }

  if (!conn) {
    return (
      <main className="flex min-h-screen flex-col items-center justify-center gap-4 p-8">
        <h1 className="text-4xl font-semibold tracking-tight">Cocktails</h1>
        <p className="text-gray-600">Dialing…</p>
      </main>
    );
  }

  if (view === "admin") {
    return <Admin conn={conn} />;
  }
  if (view === "inventory") {
    return <InventoryPage conn={conn} />;
  }
  if (view === "recipes") {
    return <RecipesPage conn={conn} />;
  }
  return <Kiosk conn={conn} connected={connected} />;
}

export default function Home() {
  return (
    <Suspense fallback={null}>
      <PageInner />
    </Suspense>
  );
}
