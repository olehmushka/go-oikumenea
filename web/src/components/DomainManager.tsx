"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api/client";
import { ErrorBox } from "./ErrorBox";
import { Localized } from "./Localized";
import { UnitKindManager } from "./UnitKindManager";
import { T } from "./T";
import { useTg } from "@/lib/locale";
import type { Domain } from "@/lib/api/types";

/**
 * Create / edit / retire the org-kind domain catalog (D-TenantOrganizations, M40) — military /
 * university / company / … — the classes above organizations. Each domain row expands to manage its
 * domain-scoped unit-kinds ({@link UnitKindManager}). `code` is immutable and there is no delete
 * endpoint: a domain is retired via `updateDomain{ status: "retired" }`.
 */
export function DomainManager({ domains }: { domains: Domain[] }) {
  const router = useRouter();
  const tr = useTg();
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const [editing, setEditing] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  const toggle = (id: string) =>
    setExpanded((prev) => {
      const next = new Set(prev);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });

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
    <div className="space-y-4">
      {err ? <ErrorBox error={err} /> : null}

      {/* Create */}
      <form
        className="card grid grid-cols-1 gap-3 p-4 sm:grid-cols-[1fr_1fr_auto_auto]"
        onSubmit={(e) => {
          e.preventDefault();
          const f = new FormData(e.currentTarget);
          const form = e.currentTarget;
          const sortRaw = String(f.get("sortOrder") || "").trim();
          run(
            () =>
              api.tenant.createDomain({
                code: String(f.get("code") || "").trim(),
                name: String(f.get("name") || "").trim(),
                sortOrder: sortRaw ? Number(sortRaw) : undefined,
              }),
            () => form.reset(),
          );
        }}
      >
        <div>
          <label className="label"><T>Code *</T></label>
          <input name="code" required className="input" placeholder="university" />
        </div>
        <div>
          <label className="label"><T>Name *</T></label>
          <input name="name" required className="input" placeholder={tr("University")} />
        </div>
        <div>
          <label className="label"><T>Order</T></label>
          <input name="sortOrder" type="number" className="input w-24" placeholder="0" />
        </div>
        <div className="flex items-end">
          <button className="btn-primary" disabled={busy}><T>Create</T></button>
        </div>
      </form>

      {/* List */}
      {domains.length === 0 ? (
        <p className="text-sm text-slate-400"><T>No domains yet — create one above.</T></p>
      ) : null}
      <div className="space-y-2">
        {domains.map((d) => {
          const isOpen = expanded.has(d.id);
          return (
            <div key={d.id} className="card p-4">
              {editing === d.id ? (
                <form
                  className="grid grid-cols-1 gap-3 sm:grid-cols-[1fr_auto_auto_auto]"
                  onSubmit={(e) => {
                    e.preventDefault();
                    const f = new FormData(e.currentTarget);
                    const sortRaw = String(f.get("sortOrder") || "").trim();
                    run(
                      () =>
                        api.tenant.updateDomain(d.id, {
                          name: String(f.get("name") || "").trim() || undefined,
                          status: String(f.get("status") || "").trim() || undefined,
                          sortOrder: sortRaw ? Number(sortRaw) : undefined,
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
                    <label className="label"><T>Order</T></label>
                    <input
                      name="sortOrder"
                      type="number"
                      className="input w-24"
                      defaultValue={d.sortOrder ?? undefined}
                    />
                  </div>
                  <div>
                    <label className="label"><T>Status</T></label>
                    <select name="status" className="input" defaultValue={d.status ?? "active"}>
                      <option value="active">active</option>
                      <option value="retired">retired</option>
                    </select>
                  </div>
                  <div className="flex items-end gap-2">
                    <button className="btn-primary" disabled={busy}><T>Save</T></button>
                    <button type="button" className="btn-ghost" onClick={() => setEditing(null)}>
                      <T>Cancel</T>
                    </button>
                  </div>
                </form>
              ) : (
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <button
                    type="button"
                    className="flex min-w-0 items-center gap-2 text-left"
                    onClick={() => toggle(d.id)}
                  >
                    <span className="text-slate-400">{isOpen ? "▾" : "▸"}</span>
                    <span className="font-medium text-slate-900"><Localized map={d.name} fallback={d.code} /></span>
                    <span className="font-mono text-xs text-slate-400">{d.code}</span>
                    {d.status === "retired" ? (
                      <span className="rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-500 ring-1 ring-inset ring-slate-200">
                        <T>retired</T>
                      </span>
                    ) : null}
                  </button>
                  <button
                    type="button"
                    className="text-xs font-medium text-indigo-600 hover:underline"
                    onClick={() => setEditing(d.id)}
                  >
                    <T>Edit</T>
                  </button>
                </div>
              )}

              {isOpen ? <UnitKindManager domainId={d.id} /> : null}
            </div>
          );
        })}
      </div>
    </div>
  );
}
