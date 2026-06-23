"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { api } from "@/lib/api/client";
import { PageHeader } from "@/components/ui";
import { ErrorBox } from "@/components/ErrorBox";
import { EntitySelect } from "@/components/EntitySelect";
import { GraphSelect } from "@/components/GraphSelect";
import { T } from "@/components/T";
import { useTg } from "@/lib/locale";

export default function NewUnitPage() {
  const router = useRouter();
  const tr = useTg();
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
      const u = await api.tenant.createUnit(body as never);
      // If a parent was picked, attach this new unit as its child (a child/"descending" unit).
      if (parentId) {
        await api.tenant.addEdge(u.id, {
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
      <PageHeader title={<T>New unit</T>} description={<T>Create a unit. Optionally pick a parent to nest it under (you can also manage edges later from the unit&apos;s detail page).</T>} />
      {err ? <div className="mb-4"><ErrorBox error={err} /></div> : null}
      <form onSubmit={onSubmit} className="card space-y-4 p-5">
        <div>
          <label className="label"><T>Code *</T></label>
          <input name="code" required className="input" placeholder="hq-1" />
          <p className="mt-1 text-xs text-slate-400"><T>Stable, locale-agnostic identifier.</T></p>
        </div>
        <div>
          <label className="label"><T>Name *</T></label>
          <input name="name" required className="input" placeholder={tr("Headquarters")} />
        </div>
        <div>
          <label className="label"><T>Kind</T></label>
          <input name="unitKind" className="input" placeholder={tr("command / department / faculty")} />
        </div>
        <div>
          <label className="label"><T>Visibility</T></label>
          <select name="visibility" className="input" defaultValue="PUBLIC">
            <option value="PUBLIC">PUBLIC</option>
            <option value="SHADOW">SHADOW</option>
          </select>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="label"><T>Parent unit (optional)</T></label>
            <EntitySelect name="parentId" kind="unit" allowEmpty placeholder={tr("Search a parent…")} />
            <p className="mt-1 text-xs text-slate-400"><T>Nests this unit under the chosen parent.</T></p>
          </div>
          <div>
            <label className="label"><T>Graph</T></label>
            <GraphSelect name="graph" />
          </div>
        </div>
        <div className="flex gap-2">
          <button type="submit" className="btn-primary" disabled={busy}>
            {busy ? <T>Creating…</T> : <T>Create unit</T>}
          </button>
          <button type="button" className="btn-ghost" onClick={() => router.back()}>
            <T>Cancel</T>
          </button>
        </div>
      </form>
    </div>
  );
}
