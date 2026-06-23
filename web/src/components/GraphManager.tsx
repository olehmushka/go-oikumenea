"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api/client";
import { ErrorBox } from "./ErrorBox";
import { ActionButton } from "./ActionButton";
import { Localized } from "./Localized";
import { T } from "./T";
import { useTg } from "@/lib/locale";
import type { Graph } from "@/lib/api/types";

/** Add / edit / delete tenant graphs (instance-admin; graph.manage). */
export function GraphManager({ graphs }: { graphs: Graph[] }) {
  const router = useRouter();
  const tr = useTg();
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const [editing, setEditing] = useState<string | null>(null);

  const run = (fn: () => Promise<unknown>, after?: () => void) => {
    setBusy(true);
    setErr(null);
    fn()
      .then(() => {
        after?.();
        router.refresh();
      })
      .catch((e) => setErr(e))
      .finally(() => setBusy(false));
  };

  return (
    <div className="space-y-2">
      {err ? <ErrorBox error={err} /> : null}
      {graphs.map((g) =>
        editing === g.id ? (
          <form
            key={g.id}
            className="flex items-center gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              const f = new FormData(e.currentTarget);
              run(
                () =>
                  api.tenant.updateGraph(g.id, {
                    name: String(f.get("name") || "").trim() || undefined,
                  }),
                () => setEditing(null),
              );
            }}
          >
            <input name="name" className="input" defaultValue={g.code} placeholder={tr("name")} />
            <button className="btn-primary" disabled={busy}>
              <T>Save</T>
            </button>
            <button type="button" className="btn-ghost" onClick={() => setEditing(null)}>
              <T>Cancel</T>
            </button>
          </form>
        ) : (
          <div key={g.id} className="flex items-center justify-between gap-2 text-sm">
            <span>
              <Localized map={g.name} fallback={g.code} />{" "}
              <span className="font-mono text-xs text-slate-400">{g.code}</span>
              {!g.isAuthorityBearing ? (
                <span className="ml-1 text-slate-400"><T>(directory-only)</T></span>
              ) : null}
            </span>
            <span className="flex items-center gap-3">
              <button
                type="button"
                className="text-xs font-medium text-indigo-600 hover:underline"
                onClick={() => setEditing(g.id)}
              >
                <T>Edit</T>
              </button>
              <ActionButton
                method="DELETE"
                path={`/tenant/v1/graphs/${g.id}`}
                label="Delete"
                confirm={`Delete graph ${g.code}?`}
                tone="danger"
              />
            </span>
          </div>
        ),
      )}

      <form
        className="mt-2 grid grid-cols-[8rem_1fr_auto] gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          const f = new FormData(e.currentTarget);
          const form = e.currentTarget;
          run(
            () =>
              api.tenant.addGraph({
                code: String(f.get("code") || "").trim(),
                name: String(f.get("name") || "").trim(),
              }),
            () => form.reset(),
          );
        }}
      >
        <input name="code" required className="input" placeholder={tr("code")} />
        <input name="name" required className="input" placeholder={tr("name")} />
        <button className="btn-ghost" disabled={busy}>
          <T>Add graph</T>
        </button>
      </form>
    </div>
  );
}
