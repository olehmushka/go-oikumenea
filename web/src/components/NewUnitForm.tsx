"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api/client";
import { ErrorBox } from "./ErrorBox";
import { GraphSelect } from "./GraphSelect";
import { T } from "./T";
import { useTg, useLocale } from "@/lib/locale";
import { pickLabel } from "@/lib/i18n";
import type { Visibility } from "@/lib/api/types";

export type Opt = { id: string; label: string };

/**
 * Create a unit inside a single, pre-chosen domain (military / university / …). The domain is fixed by
 * the entry button (UnitCreateMenu) — it is NOT a field here — so this form only asks for the things
 * that vary within a domain: the owning organization, the (domain-scoped) unit kind, name/code, and an
 * optional parent. orgId + domainId are sent to POST /units (D-TenantOrganizations, M40).
 */
export function NewUnitForm({
  domainId,
  domainLabel,
  orgs,
  kinds,
}: {
  domainId: string;
  domainLabel: string;
  orgs: Opt[];
  kinds: Opt[];
}) {
  const router = useRouter();
  const tr = useTg();
  const { locale } = useLocale();
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [orgId, setOrgId] = useState(orgs[0]?.id ?? "");
  const [parents, setParents] = useState<Opt[]>([]);

  // Parent candidates are the units already in the chosen organization (a unit nests under a parent in
  // the same org). Re-fetch whenever the org changes.
  useEffect(() => {
    if (!orgId) {
      setParents([]);
      return;
    }
    let alive = true;
    api.tenant
      .listUnits(orgId, null, null, null, null, null, null, 200)
      .then((p) => {
        if (alive)
          setParents(
            (p.units ?? []).map((u) => ({
              id: u.id,
              label: pickLabel(u.name, locale) || u.code || u.id,
            })),
          );
      })
      .catch(() => {
        if (alive) setParents([]);
      });
    return () => {
      alive = false;
    };
  }, [orgId, locale]);

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    const f = new FormData(e.currentTarget);
    try {
      const u = await api.tenant.createUnit({
        orgId,
        domainId,
        kindId: String(f.get("kindId") || "").trim() || undefined,
        code: String(f.get("code") || "").trim() || undefined,
        name: String(f.get("name") || "").trim(),
        visibility: (String(f.get("visibility") || "PUBLIC") as Visibility) || undefined,
      });
      // If a parent was picked, attach the new unit as its child in the chosen graph.
      const parentId = String(f.get("parentId") || "").trim();
      const graph = String(f.get("graph") || "").trim();
      if (parentId) {
        await api.tenant.addEdge(u.id, { parentId, graph: graph || undefined });
      }
      router.push(`/units/${u.id}`);
    } catch (e) {
      setErr(e);
      setBusy(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="card space-y-4 p-5">
      <div className="flex items-center gap-2">
        <span className="label mb-0"><T>Domain</T></span>
        <span className="inline-flex items-center rounded bg-indigo-50 px-2 py-0.5 text-xs font-medium text-indigo-700 ring-1 ring-inset ring-indigo-200">
          {domainLabel}
        </span>
        <span className="text-xs text-slate-400"><T>(fixed by the chosen unit type)</T></span>
      </div>
      {err ? <ErrorBox error={err} /> : null}
      <div>
        <label className="label"><T>Organization *</T></label>
        <select
          name="orgId"
          required
          className="input"
          value={orgId}
          onChange={(e) => setOrgId(e.target.value)}
        >
          {orgs.length === 0 ? <option value=""><T>No organizations in this domain</T></option> : null}
          {orgs.map((o) => (
            <option key={o.id} value={o.id}>{o.label}</option>
          ))}
        </select>
        <p className="mt-1 text-xs text-slate-400"><T>The owning organization (one of this domain&apos;s realms).</T></p>
      </div>
      <div>
        <label className="label"><T>Unit kind</T></label>
        <select name="kindId" className="input" defaultValue="">
          <option value=""><T>(unspecified)</T></option>
          {kinds.map((k) => (
            <option key={k.id} value={k.id}>{k.label}</option>
          ))}
        </select>
        <p className="mt-1 text-xs text-slate-400"><T>The role within the domain (e.g. brigade, faculty). Optional.</T></p>
      </div>
      <div>
        <label className="label"><T>Code</T></label>
        <input name="code" className="input" placeholder="hq-1" />
        <p className="mt-1 text-xs text-slate-400"><T>Optional human-readable identifier. Leave empty for a non-separate sub-unit — the RID is the stable external handle.</T></p>
      </div>
      <div>
        <label className="label"><T>Name *</T></label>
        <input name="name" required className="input" placeholder={tr("Headquarters")} />
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
          <select name="parentId" className="input" defaultValue="">
            <option value=""><T>(no parent — a root)</T></option>
            {parents.map((p) => (
              <option key={p.id} value={p.id}>{p.label}</option>
            ))}
          </select>
          <p className="mt-1 text-xs text-slate-400"><T>Nests this unit under a parent in the same organization.</T></p>
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
  );
}
