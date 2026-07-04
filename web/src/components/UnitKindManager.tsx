"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import { ErrorBox } from "./ErrorBox";
import { Localized } from "./Localized";
import { T } from "./T";
import { useTg } from "@/lib/locale";
import type { UnitKind } from "@/lib/api/types";

/**
 * Manage the domain-scoped unit-kind catalog (D-TenantOrganizations, M40) — the codes that replace the
 * former free-text `unitKind` (military→brigade/battalion; university→faculty/department). Codes are
 * unique per domain; `code`/`domainId` are immutable, so edits only touch name/attrSchema/status/order.
 * Rendered inline beneath a domain row in {@link DomainManager}; loads lazily on first mount (expand).
 */
export function UnitKindManager({ domainId }: { domainId: string }) {
  const tr = useTg();
  const [kinds, setKinds] = useState<UnitKind[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const [editing, setEditing] = useState<string | null>(null);

  const load = () => {
    setLoading(true);
    api.tenant
      .listUnitKinds(domainId)
      .then((r) => setKinds(r.unitKinds ?? []))
      .catch((e) => setErr(e))
      .finally(() => setLoading(false));
  };

  useEffect(load, [domainId]);

  // Parse the optional attr-schema JSON field; empty ⇒ undefined, invalid ⇒ thrown (surfaced as error).
  const parseSchema = (raw: string): unknown => {
    const s = raw.trim();
    if (!s) return undefined;
    return JSON.parse(s);
  };

  const run = (fn: () => Promise<unknown>, after?: () => void) => {
    setBusy(true);
    setErr(null);
    fn()
      .then(() => {
        after?.();
        load();
      })
      .catch((e) => setErr(e))
      .finally(() => setBusy(false));
  };

  return (
    <div className="mt-3 space-y-3 rounded-md border border-slate-200 bg-slate-50 p-3">
      <div className="text-xs font-semibold uppercase tracking-wide text-slate-500">
        <T>Unit kinds</T>
      </div>
      {err ? <ErrorBox error={err} /> : null}

      {/* Create */}
      <form
        className="grid grid-cols-1 gap-2 sm:grid-cols-[1fr_1fr_auto_auto]"
        onSubmit={(e) => {
          e.preventDefault();
          const f = new FormData(e.currentTarget);
          const form = e.currentTarget;
          let attrSchema: unknown;
          try {
            attrSchema = parseSchema(String(f.get("attrSchema") || ""));
          } catch {
            setErr(new Error(tr("Attr schema must be valid JSON.")));
            return;
          }
          const sortRaw = String(f.get("sortOrder") || "").trim();
          run(
            () =>
              api.tenant.createUnitKind({
                domainId,
                code: String(f.get("code") || "").trim(),
                name: String(f.get("name") || "").trim(),
                sortOrder: sortRaw ? Number(sortRaw) : undefined,
                attrSchema,
              }),
            () => form.reset(),
          );
        }}
      >
        <input name="code" required className="input" placeholder={tr("code (e.g. brigade)")} />
        <input name="name" required className="input" placeholder={tr("Name")} />
        <input name="sortOrder" type="number" className="input w-24" placeholder={tr("order")} />
        <button className="btn-primary" disabled={busy}>
          <T>Add</T>
        </button>
        <textarea
          name="attrSchema"
          rows={2}
          className="input font-mono text-xs sm:col-span-4"
          placeholder={tr("optional attr schema (JSON)")}
        />
      </form>

      {/* List */}
      {loading ? (
        <p className="text-xs text-slate-400"><T>Loading…</T></p>
      ) : kinds.length === 0 ? (
        <p className="text-xs text-slate-400"><T>No unit kinds in this domain yet.</T></p>
      ) : (
        <div className="space-y-1.5">
          {kinds.map((k) =>
            editing === k.id ? (
              <form
                key={k.id}
                className="grid grid-cols-1 gap-2 rounded border border-slate-200 bg-white p-2 sm:grid-cols-[1fr_auto_auto_auto]"
                onSubmit={(e) => {
                  e.preventDefault();
                  const f = new FormData(e.currentTarget);
                  let attrSchema: unknown;
                  try {
                    attrSchema = parseSchema(String(f.get("attrSchema") || ""));
                  } catch {
                    setErr(new Error(tr("Attr schema must be valid JSON.")));
                    return;
                  }
                  const sortRaw = String(f.get("sortOrder") || "").trim();
                  run(
                    () =>
                      api.tenant.updateUnitKind(k.id, {
                        name: String(f.get("name") || "").trim() || undefined,
                        status: String(f.get("status") || "").trim() || undefined,
                        sortOrder: sortRaw ? Number(sortRaw) : undefined,
                        attrSchema,
                      }),
                    () => setEditing(null),
                  );
                }}
              >
                <input name="name" className="input" placeholder={tr("(unchanged)")} />
                <input
                  name="sortOrder"
                  type="number"
                  className="input w-24"
                  defaultValue={k.sortOrder ?? undefined}
                  placeholder={tr("order")}
                />
                <select name="status" className="input" defaultValue={k.status ?? "active"}>
                  <option value="active">active</option>
                  <option value="retired">retired</option>
                </select>
                <div className="flex items-center gap-2">
                  <button className="btn-primary" disabled={busy}><T>Save</T></button>
                  <button type="button" className="btn-ghost" onClick={() => setEditing(null)}>
                    <T>Cancel</T>
                  </button>
                </div>
                <textarea
                  name="attrSchema"
                  rows={2}
                  className="input font-mono text-xs sm:col-span-4"
                  defaultValue={k.attrSchema ? JSON.stringify(k.attrSchema, null, 2) : ""}
                  placeholder={tr("optional attr schema (JSON)")}
                />
              </form>
            ) : (
              <div
                key={k.id}
                className="flex flex-wrap items-center justify-between gap-2 rounded border border-slate-200 bg-white px-3 py-1.5"
              >
                <div className="flex items-center gap-2">
                  <span className="text-sm text-slate-800"><Localized map={k.name} fallback={k.code} /></span>
                  <span className="font-mono text-xs text-slate-400">{k.code}</span>
                  {k.status === "retired" ? (
                    <span className="rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-500 ring-1 ring-inset ring-slate-200">
                      <T>retired</T>
                    </span>
                  ) : null}
                </div>
                <button
                  type="button"
                  className="text-xs font-medium text-indigo-600 hover:underline"
                  onClick={() => setEditing(k.id)}
                >
                  <T>Edit</T>
                </button>
              </div>
            ),
          )}
        </div>
      )}
    </div>
  );
}
