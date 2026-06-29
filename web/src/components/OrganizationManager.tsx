"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api/client";
import { ErrorBox } from "./ErrorBox";
import { ActionButton } from "./ActionButton";
import { Localized } from "./Localized";
import { T } from "./T";
import { useTg, useLocale } from "@/lib/locale";
import { pickLabel } from "@/lib/i18n";
import type { Domain, Organization, Visibility } from "@/lib/api/types";

/**
 * Create / edit / lifecycle organizations (the realms a person joins — D-TenantOrganizations, M40).
 * An organization is required before any unit can be created, so this is the entry point for standing
 * up a new military/university/company/… realm. `domain` is fixed at creation (it is the org's kind).
 */
export function OrganizationManager({
  organizations,
  domains,
}: {
  organizations: Organization[];
  domains: Domain[];
}) {
  const router = useRouter();
  const tr = useTg();
  const { locale } = useLocale();
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const [editing, setEditing] = useState<string | null>(null);

  const domainLabel = useMemo(() => {
    const byId = new Map(domains.map((d) => [d.id, pickLabel(d.name, locale) || d.code]));
    return (id: string) => byId.get(id) ?? id;
  }, [domains, locale]);

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

  const stateTone: Record<string, string> = {
    ACTIVE: "bg-emerald-50 text-emerald-700 ring-emerald-200",
    SUSPENDED: "bg-amber-50 text-amber-800 ring-amber-200",
    ARCHIVED: "bg-slate-100 text-slate-500 ring-slate-200",
  };

  return (
    <div className="space-y-4">
      {err ? <ErrorBox error={err} /> : null}

      {/* Create */}
      <form
        className="card grid grid-cols-1 gap-3 p-4 sm:grid-cols-[1fr_1fr_1fr_1fr_auto]"
        onSubmit={(e) => {
          e.preventDefault();
          const f = new FormData(e.currentTarget);
          const form = e.currentTarget;
          run(
            () =>
              api.tenant.createOrganization({
                code: String(f.get("code") || "").trim(),
                name: String(f.get("name") || "").trim(),
                domainId: String(f.get("domainId") || "").trim(),
                visibility: (String(f.get("visibility") || "PUBLIC") as Visibility) || undefined,
              }),
            () => form.reset(),
          );
        }}
      >
        <div>
          <label className="label"><T>Code *</T></label>
          <input name="code" required className="input" placeholder="us-army" />
        </div>
        <div>
          <label className="label"><T>Name *</T></label>
          <input name="name" required className="input" placeholder={tr("US Army")} />
        </div>
        <div>
          <label className="label"><T>Domain *</T></label>
          <select name="domainId" required className="input" defaultValue="">
            <option value="" disabled><T>Pick a domain…</T></option>
            {domains.map((d) => (
              <option key={d.id} value={d.id}>{pickLabel(d.name, locale) || d.code}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="label"><T>Visibility</T></label>
          <select name="visibility" className="input" defaultValue="PUBLIC">
            <option value="PUBLIC">PUBLIC</option>
            <option value="SHADOW">SHADOW</option>
          </select>
        </div>
        <div className="flex items-end">
          <button className="btn-primary w-full" disabled={busy}><T>Create</T></button>
        </div>
      </form>

      {/* List */}
      {organizations.length === 0 ? (
        <p className="text-sm text-slate-400"><T>No organizations yet — create one above.</T></p>
      ) : null}
      <div className="space-y-2">
        {organizations.map((o) => {
          const state = (o.state ?? "").toUpperCase();
          return editing === o.id ? (
            <form
              key={o.id}
              className="card grid grid-cols-1 gap-3 p-4 sm:grid-cols-[1fr_1fr_1fr_auto]"
              onSubmit={(e) => {
                e.preventDefault();
                const f = new FormData(e.currentTarget);
                run(
                  () =>
                    api.tenant.updateOrganization(o.id, {
                      name: String(f.get("name") || "").trim() || undefined,
                      domainId: String(f.get("domainId") || "").trim() || undefined,
                      visibility: (String(f.get("visibility") || "").trim() || undefined) as Visibility | undefined,
                    }),
                  () => setEditing(null),
                );
              }}
            >
              <div>
                <label className="label"><T>Name</T></label>
                <input name="name" className="input" placeholder={tr("(unchanged)")} />
              </div>
              <div>
                <label className="label"><T>Domain</T></label>
                <select name="domainId" className="input" defaultValue={o.domainId}>
                  {domains.map((d) => (
                    <option key={d.id} value={d.id}>{pickLabel(d.name, locale) || d.code}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="label"><T>Visibility</T></label>
                <select name="visibility" className="input" defaultValue={o.visibility ?? "PUBLIC"}>
                  <option value="PUBLIC">PUBLIC</option>
                  <option value="SHADOW">SHADOW</option>
                </select>
              </div>
              <div className="flex items-end gap-2">
                <button className="btn-primary" disabled={busy}><T>Save</T></button>
                <button type="button" className="btn-ghost" onClick={() => setEditing(null)}><T>Cancel</T></button>
              </div>
            </form>
          ) : (
            <div key={o.id} className="card flex flex-wrap items-center justify-between gap-3 p-4">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <span className="font-medium text-slate-900"><Localized map={o.name} fallback={o.code} /></span>
                  <span className="font-mono text-xs text-slate-400">{o.code}</span>
                  <span className={`inline-flex items-center rounded px-1.5 py-0.5 text-xs font-medium ring-1 ring-inset ${stateTone[state] ?? "bg-slate-100 text-slate-500 ring-slate-200"}`}>
                    {state || "—"}
                  </span>
                  {o.visibility === "SHADOW" ? (
                    <span className="text-xs text-amber-700"><T>shadow</T></span>
                  ) : null}
                </div>
                <div className="text-xs text-slate-500">
                  <T>Domain</T>: {domainLabel(o.domainId)}
                </div>
              </div>
              <div className="flex items-center gap-3">
                <button
                  type="button"
                  className="text-xs font-medium text-indigo-600 hover:underline"
                  onClick={() => setEditing(o.id)}
                >
                  <T>Edit</T>
                </button>
                {state !== "SUSPENDED" && state !== "ARCHIVED" ? (
                  <ActionButton method="PUT" path={`/tenant/v1/organizations/${o.id}/state`} body={{ toState: "SUSPENDED" }} label="Suspend" confirm="Suspend this organization?" />
                ) : null}
                {state !== "ARCHIVED" ? (
                  <ActionButton method="PUT" path={`/tenant/v1/organizations/${o.id}/state`} body={{ toState: "ARCHIVED" }} label="Archive" confirm="Archive this organization?" tone="danger" />
                ) : null}
                {state !== "ACTIVE" ? (
                  <ActionButton method="PUT" path={`/tenant/v1/organizations/${o.id}/state`} body={{ toState: "ACTIVE" }} label="Restore" />
                ) : null}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
