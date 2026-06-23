"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api/client";
import { ErrorBox } from "./ErrorBox";
import { T } from "./T";

/** Rebuild / verify the unit transitive-closure table (tenant; closure.manage). */
export function ClosureTools() {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const [result, setResult] = useState<string | null>(null);

  const run = (label: string, path: string) => {
    setBusy(true);
    setErr(null);
    setResult(null);
    api.request("POST", path)
      .then((r) => {
        setResult(`${label}: ${typeof r === "object" ? JSON.stringify(r) : "ok"}`);
        router.refresh();
      })
      .catch(setErr)
      .finally(() => setBusy(false));
  };

  return (
    <div className="space-y-3">
      {err ? <ErrorBox error={err} /> : null}
      <div className="flex gap-2">
        <button
          className="btn-ghost"
          disabled={busy}
          onClick={() => run("Rebuild", "/tenant/v1/closure/rebuild")}
        >
          <T>Rebuild</T>
        </button>
        <button
          className="btn-ghost"
          disabled={busy}
          onClick={() => run("Verify", "/tenant/v1/closure/verify")}
        >
          <T>Verify</T>
        </button>
      </div>
      {result ? (
        <pre className="overflow-x-auto rounded bg-slate-50 p-2 text-xs text-slate-600">{result}</pre>
      ) : null}
    </div>
  );
}
