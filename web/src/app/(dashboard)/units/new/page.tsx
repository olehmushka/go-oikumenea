"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { mutate } from "@/lib/api/client";
import { PageHeader } from "@/components/ui";
import { ErrorBox } from "@/components/ErrorBox";
import { EntitySelect } from "@/components/EntitySelect";
import { GraphSelect } from "@/components/GraphSelect";

export default function NewUnitPage() {
  const router = useRouter();
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    const f = new FormData(e.currentTarget);
    const body = {
      code: String(f.get("code") || "").trim(),
      name: String(f.get("name") || "").trim(),
      unitKind: String(f.get("unitKind") || "").trim() || undefined,
      visibility: String(f.get("visibility") || "PUBLIC"),
    };
    const parentId = String(f.get("parentId") || "").trim();
    const graph = String(f.get("graph") || "").trim();
    try {
      const u = await mutate<{ id: string }>("POST", "/tenant/v1/units", body);
      // If a parent was picked, attach this new unit as its child (a child/"descending" unit).
      if (parentId) {
        await mutate("POST", `/tenant/v1/units/${u.id}/edges`, {
          parentId,
          graph: graph || undefined,
        });
      }
      router.push(`/units/${u.id}`);
    } catch (e) {
      setErr(e);
      setBusy(false);
    }
  }

  return (
    <div className="max-w-lg">
      <PageHeader title="New unit" description="Create a unit. Optionally pick a parent to nest it under (you can also manage edges later from the unit's detail page)." />
      {err ? <div className="mb-4"><ErrorBox error={err} /></div> : null}
      <form onSubmit={onSubmit} className="card space-y-4 p-5">
        <div>
          <label className="label">Code *</label>
          <input name="code" required className="input" placeholder="hq-1" />
          <p className="mt-1 text-xs text-slate-400">Stable, locale-agnostic identifier.</p>
        </div>
        <div>
          <label className="label">Name *</label>
          <input name="name" required className="input" placeholder="Headquarters" />
        </div>
        <div>
          <label className="label">Kind</label>
          <input name="unitKind" className="input" placeholder="command / department / faculty" />
        </div>
        <div>
          <label className="label">Visibility</label>
          <select name="visibility" className="input" defaultValue="PUBLIC">
            <option value="PUBLIC">PUBLIC</option>
            <option value="SHADOW">SHADOW</option>
          </select>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="label">Parent unit (optional)</label>
            <EntitySelect name="parentId" kind="unit" allowEmpty placeholder="Search a parent…" />
            <p className="mt-1 text-xs text-slate-400">Nests this unit under the chosen parent.</p>
          </div>
          <div>
            <label className="label">Graph</label>
            <GraphSelect name="graph" />
          </div>
        </div>
        <div className="flex gap-2">
          <button type="submit" className="btn-primary" disabled={busy}>
            {busy ? "Creating…" : "Create unit"}
          </button>
          <button type="button" className="btn-ghost" onClick={() => router.back()}>
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}
