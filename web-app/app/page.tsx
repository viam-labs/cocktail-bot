"use client";

import { Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useViamConnection } from "./lib/useViamConnection";
import { getMachineName } from "./lib/viamClient";

function Status() {
  const params = useSearchParams();
  const partId = params.get("partId") ?? "";

  if (!partId) {
    return (
      <main className="flex min-h-screen flex-col items-center justify-center gap-4 p-8">
        <h1 className="text-4xl font-semibold tracking-tight">Cocktails</h1>
        <p className="text-gray-600">
          Pass <code>?partId=&lt;machine-part-id&gt;</code> in the URL to connect.
        </p>
      </main>
    );
  }

  return <Connected partId={partId} />;
}

function Connected({ partId }: { partId: string }) {
  const { conn, connected, error } = useViamConnection(partId);
  const [machineName, setMachineName] = useState<string>("");

  useEffect(() => {
    if (!conn) return;
    let cancelled = false;
    getMachineName(conn).then((n) => {
      if (!cancelled) setMachineName(n);
    }).catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [conn]);

  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-4 p-8">
      <h1 className="text-4xl font-semibold tracking-tight">Cocktails</h1>
      {error ? (
        <p className="text-red-600">{error}</p>
      ) : !conn ? (
        <p className="text-gray-600">Dialing…</p>
      ) : (
        <p className={connected ? "text-green-700" : "text-amber-700"}>
          {connected ? "Connected" : "Reconnecting"} to {machineName || conn.hostname}
        </p>
      )}
    </main>
  );
}

export default function Home() {
  return (
    <Suspense fallback={null}>
      <Status />
    </Suspense>
  );
}
